package e2e

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"go.uber.org/fx"
	"go.uber.org/fx/fxtest"

	contracterrors "github.com/aegiscore/common/contract/errors"
	contractresponse "github.com/aegiscore/common/contract/response"
	"github.com/aegiscore/common/runtime/config"
	"github.com/aegiscore/common/runtime/logger"
	"github.com/aegiscore/common/testing/containers"
	"github.com/aegiscore/user-service/internal/bootstrap"
)

const envTestE2E = "AEGISCORE_TEST_E2E"

type httpFlowHarness struct {
	engine      *gin.Engine
	postgresDSN string
}

type testEnvelope struct {
	Success bool                `json:"success"`
	Code    contracterrors.Code `json:"code"`
	Message string              `json:"message"`
	Data    json.RawMessage     `json:"data,omitempty"`
	Errors  json.RawMessage     `json:"errors,omitempty"`
}

func newHTTPFlowHarness(t *testing.T) *httpFlowHarness {
	t.Helper()
	requireE2EEnabled(t)

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	t.Cleanup(cancel)

	postgres := containers.StartPostgres(ctx, t, containers.PostgresOptions{})
	redis := containers.StartRedis(ctx, t, containers.RedisOptions{})
	applyMigrations(ctx, t, postgres.DSN)

	configPath := writeTestConfig(t, postgres.Config(), redis.Config())
	var engine *gin.Engine
	app := fxtest.New(t,
		fx.Supply(config.ConfigPath(configPath)),
		fx.Provide(
			config.NewConfig,
			logger.NewLogger,
		),
		bootstrap.AppModule,
		fx.Populate(&engine),
	)
	app.RequireStart()
	t.Cleanup(func() { app.RequireStop() })

	return &httpFlowHarness{engine: engine, postgresDSN: postgres.DSN}
}

func requireE2EEnabled(t *testing.T) {
	t.Helper()
	if envEnabled(envTestE2E) {
		t.Setenv(containers.EnvTestContainers, "1")
		return
	}
	if containers.ContainersEnabled() {
		return
	}
	t.Skipf("set %s=1 to run user-service HTTP flow integration tests", envTestE2E)
}

func envEnabled(name string) bool {
	switch strings.ToLower(strings.TrimSpace(os.Getenv(name))) {
	case "1", "true", "yes":
		return true
	default:
		return false
	}
}

func writeTestConfig(t *testing.T, postgres config.PostgresConfig, redis config.RedisConfig) string {
	t.Helper()
	port := freeTCPPort(t)
	logDir := filepath.Join(t.TempDir(), "logs")
	content := fmt.Sprintf(`app:
  name: aegiscore-user-services-test
  environment: test
system:
  timezone: Asia/Shanghai
http:
  host: 127.0.0.1
  port: %d
  read_timeout: 5s
  write_timeout: 5s
  idle_timeout: 10s
  shutdown_timeout: 5s
  trusted_proxies:
    - 127.0.0.1
auth:
  jwt:
    secret: integration-test-jwt-secret-value
    issuer: aegiscore-user-services-test
    audience: aegiscore-users
    access_token_ttl: 5m
    refresh_token_ttl: 30m
  token_version_cache_ttl: 5s
  refresh_token_rotation: true
  max_active_sessions_per_user: 5
log:
  level: error
  format: json
  directory: %q
  filename: "aegiscore-user-services-test"
  console: false
  max_age_days: 0
  max_size_mb: 0
  max_backups: 0
redis:
  cache_redis:
    addr: %q
    username: %q
    password: %q
    db: %d
    dial_timeout: %s
    read_timeout: %s
    write_timeout: %s
    ping_timeout: %s
postgres:
  user_db:
    host: %q
    port: %d
    username: %q
    password: %q
    db_name: %q
    driver: %q
    sslmode: %q
    max_open_conns: %d
    max_idle_conns: %d
    conn_max_lifetime: %s
    conn_max_idle_time: %s
    ping_timeout: %s
`,
		port,
		logDir,
		redis.Addr, redis.Username, redis.Password, redis.DB, redis.DialTimeout, redis.ReadTimeout, redis.WriteTimeout, redis.PingTimeout,
		postgres.Host, postgres.Port, postgres.Username, postgres.Password, postgres.DBName, postgres.Driver, postgres.SSLMode, postgres.MaxOpenConns, postgres.MaxIdleConns, postgres.ConnMaxLifetime, postgres.ConnMaxIdleTime, postgres.PingTimeout,
	)
	path := filepath.Join(t.TempDir(), "config.yaml")
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatalf("write test config: %v", err)
	}
	return path
}

func freeTCPPort(t *testing.T) int {
	t.Helper()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen free tcp port: %v", err)
	}
	defer func() { _ = listener.Close() }()
	_, portText, err := net.SplitHostPort(listener.Addr().String())
	if err != nil {
		t.Fatalf("split free tcp port: %v", err)
	}
	port, err := strconv.Atoi(portText)
	if err != nil {
		t.Fatalf("parse free tcp port: %v", err)
	}
	return port
}

func (h *httpFlowHarness) request(t *testing.T, method string, path string, body any, token string) *httptest.ResponseRecorder {
	t.Helper()
	var payload []byte
	if body != nil {
		var err error
		payload, err = json.Marshal(body)
		if err != nil {
			t.Fatalf("marshal request body for %s %s: %v", method, path, err)
		}
	}
	request := httptest.NewRequest(method, path, bytes.NewReader(payload))
	request.RemoteAddr = "203.0.113.10:12345"
	request.Header.Set("User-Agent", "user-service-http-flow-test")
	if body != nil {
		request.Header.Set("Content-Type", "application/json")
	}
	if token != "" {
		request.Header.Set("Authorization", "Bearer "+token)
	}
	recorder := httptest.NewRecorder()
	h.engine.ServeHTTP(recorder, request)
	return recorder
}

func decodeEnvelope(t *testing.T, recorder *httptest.ResponseRecorder) testEnvelope {
	t.Helper()
	var envelope testEnvelope
	decoder := json.NewDecoder(recorder.Body)
	decoder.UseNumber()
	if err := decoder.Decode(&envelope); err != nil {
		t.Fatalf("decode response envelope: status=%d error=%v", recorder.Code, err)
	}
	return envelope
}

func expectEnvelope(t *testing.T, recorder *httptest.ResponseRecorder, status int, success bool, code contracterrors.Code) testEnvelope {
	t.Helper()
	envelope := decodeEnvelope(t, recorder)
	if recorder.Code != status || envelope.Success != success || envelope.Code != code {
		t.Fatalf("response = status %d success %v code %d message %q, want status %d success %v code %d", recorder.Code, envelope.Success, envelope.Code, envelope.Message, status, success, code)
	}
	if success && envelope.Message != contractresponse.MessageOK && status != http.StatusCreated {
		t.Fatalf("success message = %q, want %q", envelope.Message, contractresponse.MessageOK)
	}
	if success && status == http.StatusCreated && envelope.Message != contractresponse.MessageCreated {
		t.Fatalf("created message = %q, want %q", envelope.Message, contractresponse.MessageCreated)
	}
	return envelope
}

func decodeData[T any](t *testing.T, envelope testEnvelope) T {
	t.Helper()
	var data T
	if len(envelope.Data) == 0 {
		t.Fatal("response data is empty")
	}
	if err := json.Unmarshal(envelope.Data, &data); err != nil {
		t.Fatalf("decode response data: %v", err)
	}
	return data
}

func openPostgres(t *testing.T, dsn string) *sql.DB {
	t.Helper()
	db, err := sql.Open("pgx", dsn)
	if err != nil {
		t.Fatalf("open PostgreSQL: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	return db
}
