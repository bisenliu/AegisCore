package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestLoadParsesServicePrivateConfig(t *testing.T) {
	cfg := loadServiceConfig(t, serviceConfigYAML())
	require.Equal(t, "secret-123456789012345678901234567890", cfg.Auth.JWT.Secret)
	require.Equal(t, 15*time.Minute, cfg.Auth.JWT.AccessTokenTTL)
	require.Equal(t, 168*time.Hour, cfg.Auth.JWT.RefreshTokenTTL)
	require.Equal(t, 5*time.Minute, cfg.Auth.JWT.PasswordChangeTokenTTL)
	require.Equal(t, 30*time.Second, cfg.Auth.TokenVersionCacheTTL)
	require.True(t, cfg.Auth.RefreshTokenRotation)
	require.Equal(t, 5, cfg.Auth.MaxActiveSessionsPerUser)
	require.Equal(t, 2, cfg.Auth.PasswordKDF.Argon2Concurrency)
	require.Equal(t, 16, cfg.Auth.PasswordKDF.Argon2QueueSize)
	require.True(t, cfg.Ent.SQLDebug)
	runtime := cfg.RuntimeConfig()
	require.Equal(t, "aegiscore-test", runtime.App.Name)
}

func TestValidateRejectsInvalidAuthConfig(t *testing.T) {
	err := loadServiceConfigError(t, strings.ReplaceAll(serviceConfigYAML(), "argon2_queue_size: 16", "argon2_queue_size: 1"))
	require.Contains(t, err.Error(), "auth.password_kdf.argon2_queue_size must be >= auth.password_kdf.argon2_concurrency")
}

func TestValidateRejectsShortProductionJWTSecret(t *testing.T) {
	yaml := strings.ReplaceAll(serviceConfigYAML(), "environment: local", "environment: production")
	yaml = strings.ReplaceAll(yaml, "secret-123456789012345678901234567890", "short-secret")
	err := loadServiceConfigError(t, yaml)
	require.Contains(t, err.Error(), "auth.jwt.secret must be at least 32 bytes in production-like environments")
}

func loadServiceConfig(t *testing.T, content string) *Config {
	t.Helper()
	cfg, err := NewConfig(ConfigPath(writeTempConfig(t, content)))
	require.NoError(t, err)
	return cfg
}

func loadServiceConfigError(t *testing.T, content string) error {
	t.Helper()
	_, err := NewConfig(ConfigPath(writeTempConfig(t, content)))
	require.Error(t, err)
	return err
}

func writeTempConfig(t *testing.T, content string) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	require.NoError(t, os.WriteFile(path, []byte(content), 0o600))
	return path
}

func serviceConfigYAML() string {
	return `app:
  name: aegiscore-test
  environment: local
system:
  timezone: Asia/Shanghai
http:
  host: 127.0.0.1
  port: 18080
  read_timeout: 10s
  write_timeout: 20s
  idle_timeout: 30s
  shutdown_timeout: 5s
auth:
  jwt:
    secret: secret-123456789012345678901234567890
    issuer: aegiscore-test
    audience: aegiscore-users
    access_token_ttl: 15m
    refresh_token_ttl: 168h
    password_change_token_ttl: 5m
  password_kdf:
    argon2_concurrency: 2
    argon2_queue_size: 16
  token_version_cache_ttl: 30s
  refresh_token_rotation: true
  max_active_sessions_per_user: 5
ent:
  sql_debug: true
local_cache:
  auth_token_version:
    capacity: 1000
    ttl: 1s
    load_timeout: 300ms
log:
  level: info
  format: json
observability:
  metrics:
    enabled: false
    path: /metrics
    include_runtime: true
  tracing:
    enabled: true
    sample_ratio: 0.25
    exporter: none
redis:
  cache_redis:
    addr: 127.0.0.1:6379
    db: 2
    dial_timeout: 5s
    read_timeout: 3s
    write_timeout: 3s
    ping_timeout: 7s
postgres:
  user_db:
    host: 127.0.0.1
    port: 15432
    username: aegiscore
    password: secret
    db_name: aegiscore_user
    driver: pgx
    sslmode: disable
    max_open_conns: 20
    max_idle_conns: 4
    conn_max_lifetime: 45m
    conn_max_idle_time: 12m
    ping_timeout: 7s
`
}
