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
	commonresources "github.com/aegiscore/common/runtime/resources"
	"github.com/aegiscore/common/testing/containers"
	"github.com/aegiscore/user-service/internal/bootstrap"
	configtestkit "github.com/aegiscore/user-service/internal/config/testkit"
)

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
	bootstrapSuperAdmin(t, postgres.DSN, "bootstrap-secret")

	configPath := writeTestConfig(t, postgres.Config(), redis.Config())
	content, err := os.ReadFile(configPath)
	require.NoError(t, err)
	loaded, err := configtestkit.LoadFromDocuments([]commonconfig.ConfigDocument{{DataID: "config.yaml", Content: content}})
	require.NoError(t, err)
	serviceCfg := loaded.Config
	var engine *gin.Engine
	app := fxtest.New(t, bootstrap.AppOptions(
		serviceCfg,
		bootstrap.AppModule,
		fx.Populate(&engine),
	)...)
	app.RequireStart()
	t.Cleanup(func() { app.RequireStop() })

	return &httpFlowHarness{engine: engine, postgresDSN: postgres.DSN}
}

func requireE2EEnabled(t *testing.T) {
	t.Helper()
	if containers.ContainersEnabled() {
		return
	}
	t.Skip("pass -args -aegiscore.testcontainers to run user-service HTTP flow integration tests")
}

func writeTestConfig(t *testing.T, postgres commonresources.PostgresConfig, redis commonresources.RedisConfig) string {
	t.Helper()
	port := freeTCPPort(t)
	content := fmt.Sprintf(`app:
  name: aegiscore-user-service-test
  environment: test
server:
  http:
    enabled: true
    host: 127.0.0.1
    port: %d
    read_timeout: 5s
    write_timeout: 5s
    idle_timeout: 10s
    shutdown_timeout: 5s
  grpc:
    enabled: false
    host: 127.0.0.1
    port: 19090
    shutdown_timeout: 5s
auth:
  jwt:
    secret: integration-test-jwt-secret-value
    issuer: aegiscore-user-service-test
    audience: aegiscore-users
    access_token_ttl: 5m
    refresh_token_ttl: 30m
  token_version_cache:
    enabled: true
    size: 1000
    ttl: 1s
    load_timeout: 300ms
  token_version_cache_ttl: 5s
  refresh_token_rotation: true
  max_active_sessions_per_user: 5
api_rate_limit:
  anonymous:
    enabled: false
  authenticated:
    enabled: false
rbac:
  user_role_cache:
    enabled: true
    size: 1000
    ttl: 5s
    load_timeout: 500ms
log:
  level: error
  format: json
observability:
  metrics:
    enabled: true
    path: /metrics
    include_runtime: false
  tracing:
    enabled: false
    sample_ratio: 0
    otlp_endpoint: ""
    insecure: false
resources:
  redis:
    cache_redis:
      mode: cluster
      addrs:
        - %q
      username: %q
      password: %q
      timeout: %s
      cluster:
        max_redirects: %d
  postgres:
    primary_db:
      host: %q
      port: %d
      username: %q
      password: %q
      db_name: %q
      sslmode: %q
      pool:
        max_open_conns: %d
        max_idle_conns: %d
        conn_max_lifetime: %s
        conn_max_idle_time: %s
`,
		port,
		redis.Addrs[0], redis.Username, redis.Password, redis.Timeout, redis.Cluster.MaxRedirects,
		postgres.Host, postgres.Port, postgres.Username, postgres.Password, postgres.DBName, postgres.SSLMode,
		postgres.Pool.MaxOpenConns, postgres.Pool.MaxIdleConns, postgres.Pool.ConnMaxLifetime, postgres.Pool.ConnMaxIdleTime,
	)
	path := filepath.Join(t.TempDir(), "config.yaml")
	require.NoError(t, os.WriteFile(path, []byte(content), 0o600), "write test config")
	return path
}

func freeTCPPort(t *testing.T) int {
	t.Helper()
	// 这里只为 Fx 测试配置选择候选端口；listener 关闭到应用绑定之间仍存在极小竞态，绑定失败会由 App start 明确报告。
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
