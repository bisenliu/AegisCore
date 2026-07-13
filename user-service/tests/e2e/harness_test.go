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
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/fx"
	"go.uber.org/fx/fxtest"

	contracterrors "github.com/aegiscore/common/contract/errors"
	contractresponse "github.com/aegiscore/common/contract/response"
	commonconfig "github.com/aegiscore/common/runtime/config"
	"github.com/aegiscore/common/runtime/logger"
	"github.com/aegiscore/common/testing/containers"
	"github.com/aegiscore/user-service/internal/bootstrap"
	serviceconfig "github.com/aegiscore/user-service/internal/config"
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
	seedRBACBaseline(t, postgres.DSN)

	configPath := writeTestConfig(t, postgres.Config(), redis.Config())
	var engine *gin.Engine
	app := fxtest.New(t,
		fx.Supply(serviceconfig.ConfigPath(configPath)),
		fx.Provide(
			serviceconfig.NewConfig,
			serviceconfig.NewRuntimeConfig,
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

func writeTestConfig(t *testing.T, postgres commonconfig.PostgresConfig, redis commonconfig.RedisConfig) string {
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
  password_kdf:
    argon2_concurrency: 2
    argon2_queue_size: 16
  token_version_cache_ttl: 5s
  refresh_token_rotation: true
  max_active_sessions_per_user: 5
local_cache:
  auth_token_version:
    capacity: 1000
    ttl: 1s
    load_timeout: 300ms
    num_counters: 0
    buffer_items: 0
  rbac_user_roles:
    capacity: 1000
    ttl: 5s
    load_timeout: 500ms
    num_counters: 0
    buffer_items: 0
log:
  level: error
  format: json
  directory: %q
  filename: "aegiscore-user-services-test"
  console: false
  max_age_days: 0
  max_size_mb: 0
  max_backups: 0
observability:
  metrics:
    enabled: true
    path: /metrics
    include_runtime: false
  tracing:
    enabled: true
    sample_ratio: 0
    exporter: none
    otlp_endpoint: ""
    insecure: false
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
	require.NoError(t, os.WriteFile(path, []byte(content), 0o600), "write test config")
	return path
}

func freeTCPPort(t *testing.T) int {
	t.Helper()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err, "listen free tcp port")
	defer func() { _ = listener.Close() }()
	_, portText, err := net.SplitHostPort(listener.Addr().String())
	require.NoError(t, err, "split free tcp port")
	port, err := strconv.Atoi(portText)
	require.NoError(t, err, "parse free tcp port")
	require.Greater(t, port, 0, "free tcp port")
	return port
}

func (h *httpFlowHarness) request(t *testing.T, method string, path string, body any, token string) *httptest.ResponseRecorder {
	t.Helper()
	var payload []byte
	if body != nil {
		var err error
		payload, err = json.Marshal(body)
		require.NoError(t, err, "marshal request body for %s %s", method, path)
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
	require.NoError(t, decoder.Decode(&envelope), "decode response envelope: status=%d", recorder.Code)
	return envelope
}

func expectEnvelope(t *testing.T, recorder *httptest.ResponseRecorder, status int, success bool, code contracterrors.Code) testEnvelope {
	t.Helper()
	envelope := decodeEnvelope(t, recorder)
	assert.Equal(t, status, recorder.Code, "response status: message=%q", envelope.Message)
	assert.Equal(t, success, envelope.Success, "response success: message=%q", envelope.Message)
	assert.Equal(t, code, envelope.Code, "response code: message=%q", envelope.Message)
	if success && status != http.StatusCreated {
		assert.Equal(t, contractresponse.MessageOK, envelope.Message, "success message")
	}
	if success && status == http.StatusCreated {
		assert.Equal(t, contractresponse.MessageCreated, envelope.Message, "created message")
	}
	return envelope
}

func decodeData[T any](t *testing.T, envelope testEnvelope) T {
	t.Helper()
	var data T
	require.NotEmpty(t, envelope.Data, "response data")
	require.NoError(t, json.Unmarshal(envelope.Data, &data), "decode response data")
	return data
}

func openPostgres(t *testing.T, dsn string) *sql.DB {
	t.Helper()
	db, err := sql.Open("pgx", dsn)
	require.NoError(t, err, "open PostgreSQL")
	t.Cleanup(func() { _ = db.Close() })
	return db
}
