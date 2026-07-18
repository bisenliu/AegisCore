package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	commonresources "github.com/aegiscore/common/runtime/resources"
	serviceresources "github.com/aegiscore/user-service/internal/resources"
)

func TestLoadParsesServicePrivateConfig(t *testing.T) {
	cfg := loadServiceConfig(t, serviceConfigYAML())
	require.Equal(t, "secret-123456789012345678901234567890", cfg.Auth.JWT.Secret)
	require.Equal(t, 15*time.Minute, cfg.Auth.JWT.AccessTokenTTL)
	require.Equal(t, 168*time.Hour, cfg.Auth.JWT.RefreshTokenTTL)
	require.Equal(t, 5*time.Minute, cfg.Auth.JWT.PasswordChangeTokenTTL)
	require.Equal(t, 30*time.Second, cfg.Auth.TokenVersionCacheTTL)
	require.True(t, cfg.Auth.TokenVersionCache.IsEnabled())
	require.EqualValues(t, 2048, cfg.Auth.TokenVersionCache.SizeValue())
	require.Equal(t, 2*time.Second, cfg.Auth.TokenVersionCache.TTLValue())
	require.Equal(t, 400*time.Millisecond, cfg.Auth.TokenVersionCache.LoadTimeoutValue())
	require.True(t, cfg.RBAC.UserRoleCache.IsEnabled())
	require.EqualValues(t, 4096, cfg.RBAC.UserRoleCache.SizeValue())
	require.Equal(t, 7*time.Second, cfg.RBAC.UserRoleCache.TTLValue())
	require.Equal(t, 600*time.Millisecond, cfg.RBAC.UserRoleCache.LoadTimeoutValue())
	require.True(t, cfg.Auth.RefreshTokenRotation)
	require.Equal(t, 5, cfg.Auth.MaxActiveSessionsPerUser)
	require.Equal(t, 2, cfg.Auth.PasswordKDF.Argon2Concurrency)
	require.Equal(t, 16, cfg.Auth.PasswordKDF.Argon2QueueSize)
	require.True(t, cfg.Ent.SQLDebug)
	require.Len(t, cfg.Resources.Redis, 1)
	require.Equal(t, "127.0.0.1:6379", cfg.Resources.Redis[serviceresources.NameCacheRedis].Addr)
	require.Equal(t, 7*time.Second, cfg.Resources.Redis[serviceresources.NameCacheRedis].Timeout)
	require.Len(t, cfg.Resources.Postgres, 1)
	require.Equal(t, 20, cfg.Resources.Postgres[serviceresources.NamePrimaryDB].Pool.MaxOpenConns)
	runtime := cfg.RuntimeConfig()
	require.Equal(t, "aegiscore-test", runtime.App.Name)
	require.Equal(t, 21*time.Second, runtime.Runtime.Lifecycle.StartTimeout)
	require.Equal(t, 50*time.Second, runtime.Runtime.Lifecycle.StopTimeout)
}

func TestApplyDefaultsSetsFeatureCacheDefaults(t *testing.T) {
	cfg := &Config{}
	cfg.ApplyDefaults()

	require.True(t, cfg.Auth.TokenVersionCache.IsEnabled())
	require.EqualValues(t, 100000, cfg.Auth.TokenVersionCache.SizeValue())
	require.Equal(t, time.Second, cfg.Auth.TokenVersionCache.TTLValue())
	require.Equal(t, 300*time.Millisecond, cfg.Auth.TokenVersionCache.LoadTimeoutValue())
	require.True(t, cfg.RBAC.UserRoleCache.IsEnabled())
	require.EqualValues(t, 100000, cfg.RBAC.UserRoleCache.SizeValue())
	require.Equal(t, 5*time.Second, cfg.RBAC.UserRoleCache.TTLValue())
	require.Equal(t, 500*time.Millisecond, cfg.RBAC.UserRoleCache.LoadTimeoutValue())
}

func TestLoadAppliesFeatureCacheDefaults(t *testing.T) {
	yaml := strings.Replace(serviceConfigYAML(), `  token_version_cache:
    enabled: true
    size: 2048
    ttl: 2s
    load_timeout: 400ms
`, "", 1)
	yaml = strings.Replace(yaml, `rbac:
  user_role_cache:
    enabled: true
    size: 4096
    ttl: 7s
    load_timeout: 600ms
`, "", 1)

	cfg := loadServiceConfig(t, yaml)
	require.True(t, cfg.Auth.TokenVersionCache.IsEnabled())
	require.EqualValues(t, 100000, cfg.Auth.TokenVersionCache.SizeValue())
	require.Equal(t, time.Second, cfg.Auth.TokenVersionCache.TTLValue())
	require.Equal(t, 300*time.Millisecond, cfg.Auth.TokenVersionCache.LoadTimeoutValue())
	require.True(t, cfg.RBAC.UserRoleCache.IsEnabled())
	require.EqualValues(t, 100000, cfg.RBAC.UserRoleCache.SizeValue())
	require.Equal(t, 5*time.Second, cfg.RBAC.UserRoleCache.TTLValue())
	require.Equal(t, 500*time.Millisecond, cfg.RBAC.UserRoleCache.LoadTimeoutValue())
}

func TestApplyDefaultsPreservesDisabledFeatureCacheZeroValues(t *testing.T) {
	disabled := false
	cfg := &Config{
		Auth: AuthConfig{TokenVersionCache: FeatureCacheConfig{Enabled: &disabled}},
		RBAC: RBACConfig{UserRoleCache: FeatureCacheConfig{Enabled: &disabled}},
	}

	cfg.ApplyDefaults()

	require.False(t, cfg.Auth.TokenVersionCache.IsEnabled())
	require.Zero(t, cfg.Auth.TokenVersionCache.SizeValue())
	require.Zero(t, cfg.Auth.TokenVersionCache.TTLValue())
	require.Zero(t, cfg.Auth.TokenVersionCache.LoadTimeoutValue())
	require.False(t, cfg.RBAC.UserRoleCache.IsEnabled())
	require.Zero(t, cfg.RBAC.UserRoleCache.SizeValue())
	require.Zero(t, cfg.RBAC.UserRoleCache.TTLValue())
	require.Zero(t, cfg.RBAC.UserRoleCache.LoadTimeoutValue())
}

func TestLoadPreservesExplicitDisabledFeatureCaches(t *testing.T) {
	yaml := strings.Replace(serviceConfigYAML(), `  token_version_cache:
    enabled: true
    size: 2048
    ttl: 2s
    load_timeout: 400ms
`, `  token_version_cache:
    enabled: false
`, 1)
	yaml = strings.Replace(yaml, `rbac:
  user_role_cache:
    enabled: true
    size: 4096
    ttl: 7s
    load_timeout: 600ms
`, `rbac:
  user_role_cache:
    enabled: false
`, 1)

	cfg := loadServiceConfig(t, yaml)
	require.False(t, cfg.Auth.TokenVersionCache.IsEnabled())
	require.Zero(t, cfg.Auth.TokenVersionCache.SizeValue())
	require.Zero(t, cfg.Auth.TokenVersionCache.TTLValue())
	require.Zero(t, cfg.Auth.TokenVersionCache.LoadTimeoutValue())
	require.False(t, cfg.RBAC.UserRoleCache.IsEnabled())
	require.Zero(t, cfg.RBAC.UserRoleCache.SizeValue())
	require.Zero(t, cfg.RBAC.UserRoleCache.TTLValue())
	require.Zero(t, cfg.RBAC.UserRoleCache.LoadTimeoutValue())
}

func TestLoadRejectsExplicitInvalidFeatureCacheValue(t *testing.T) {
	yaml := strings.Replace(serviceConfigYAML(), "    size: 2048", "    size: 0", 1)
	err := loadServiceConfigError(t, yaml)
	require.Contains(t, err.Error(), "auth.token_version_cache.size must be > 0 when enabled")
}

func TestValidateFeatureCaches(t *testing.T) {
	enabled := true
	disabled := false

	t.Run("enabled requires positive values", func(t *testing.T) {
		errs := validateFeatureCache("auth.token_version_cache", FeatureCacheConfig{Enabled: &enabled})
		require.Len(t, errs, 3)
		require.Contains(t, errs[0].Error(), "auth.token_version_cache.size")
	})

	t.Run("disabled allows zero values", func(t *testing.T) {
		errs := validateFeatureCache("rbac.user_role_cache", FeatureCacheConfig{Enabled: &disabled})
		require.Empty(t, errs)
	})
}

func TestLoadAppliesResourceDefaultsBeforeValidation(t *testing.T) {
	cfg := loadServiceConfig(t, serviceConfigYAMLWithResourceDefaults())

	require.Equal(t, commonresources.DefaultRedisTimeout, cfg.Resources.Redis[serviceresources.NameCacheRedis].Timeout)
	postgres := cfg.Resources.Postgres[serviceresources.NamePrimaryDB]
	require.Equal(t, commonresources.DefaultPostgresSSLMode, postgres.SSLMode)
	require.Equal(t, commonresources.DefaultPostgresMaxOpenConns, postgres.Pool.MaxOpenConns)
	require.Equal(t, commonresources.DefaultPostgresMaxIdleConns, postgres.Pool.MaxIdleConns)
	require.Equal(t, commonresources.DefaultPostgresConnMaxLifetime, postgres.Pool.ConnMaxLifetime)
	require.Equal(t, commonresources.DefaultPostgresConnMaxIdleTime, postgres.Pool.ConnMaxIdleTime)
}

func TestLoadAppliesNestedResourceEnvironmentOverride(t *testing.T) {
	t.Setenv("AEGISCORE_RESOURCES_REDIS_CACHE_REDIS_TIMEOUT", "11s")
	cfg := loadServiceConfig(t, serviceConfigYAML())
	require.Equal(t, 11*time.Second, cfg.Resources.Redis[serviceresources.NameCacheRedis].Timeout)
}

func TestLoadRepositoryConfig(t *testing.T) {
	cfg, err := NewConfig(ConfigPath(filepath.Join("..", "..", "configs", "config.yaml")))
	require.NoError(t, err)
	require.Equal(t, 60*time.Second, cfg.Runtime.Lifecycle.StartTimeout)
	require.Equal(t, 120*time.Second, cfg.Runtime.Lifecycle.StopTimeout)
	require.True(t, cfg.Server.HTTP.Enabled)
	require.False(t, cfg.Server.GRPC.Enabled)
	require.Contains(t, cfg.Resources.Redis, serviceresources.NameCacheRedis)
	require.Contains(t, cfg.Resources.Postgres, serviceresources.NamePrimaryDB)
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

func TestValidateRejectsMissingRequiredResources(t *testing.T) {
	yaml := strings.Replace(serviceConfigYAML(), "  cache_redis:\n", "  other_redis:\n", 1)
	yaml = strings.Replace(yaml, "  primary_db:\n", "  other_db:\n", 1)
	err := loadServiceConfigError(t, yaml)
	require.Contains(t, err.Error(), "resources.redis.cache_redis is required")
	require.Contains(t, err.Error(), "resources.postgres.primary_db is required")
}

func TestValidateReportsFullResourceFieldPath(t *testing.T) {
	yaml := strings.Replace(serviceConfigYAML(), "    addr: 127.0.0.1:6379", "    addr: invalid", 1)
	err := loadServiceConfigError(t, yaml)
	require.Contains(t, err.Error(), "resources.redis.cache_redis.addr must be in host:port format")
}

func TestLoadRejectsLegacyTopLevelResourcePath(t *testing.T) {
	yaml := "redis:\n  cache_redis:\n    addr: 127.0.0.1:6379\n" + serviceConfigYAML()
	err := loadServiceConfigError(t, yaml)
	require.Contains(t, err.Error(), "unknown configuration keys: redis.cache_redis.addr")
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
server:
  http:
    enabled: true
    host: 127.0.0.1
    port: 18080
    read_timeout: 10s
    write_timeout: 20s
    idle_timeout: 30s
    shutdown_timeout: 5s
  grpc:
    enabled: false
runtime:
  lifecycle:
    start_timeout: 21s
    stop_timeout: 50s
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
  token_version_cache:
    enabled: true
    size: 2048
    ttl: 2s
    load_timeout: 400ms
  token_version_cache_ttl: 30s
  refresh_token_rotation: true
  max_active_sessions_per_user: 5
rbac:
  user_role_cache:
    enabled: true
    size: 4096
    ttl: 7s
    load_timeout: 600ms
ent:
  sql_debug: true
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
    otlp_endpoint: collector:4317
    insecure: true
resources:
  redis:
    cache_redis:
      addr: 127.0.0.1:6379
      db: 2
      timeout: 7s
  postgres:
    primary_db:
      host: 127.0.0.1
      port: 15432
      username: aegiscore
      password: ""
      db_name: aegiscore_user
      sslmode: disable
      pool:
        max_open_conns: 20
        max_idle_conns: 4
        conn_max_lifetime: 45m
        conn_max_idle_time: 12m
`
}

func serviceConfigYAMLWithResourceDefaults() string {
	yaml := serviceConfigYAML()
	yaml = strings.Replace(yaml, "      timeout: 7s\n", "", 1)
	yaml = strings.Replace(yaml, "      sslmode: disable\n      pool:\n        max_open_conns: 20\n        max_idle_conns: 4\n        conn_max_lifetime: 45m\n        conn_max_idle_time: 12m\n", "", 1)
	return yaml
}
