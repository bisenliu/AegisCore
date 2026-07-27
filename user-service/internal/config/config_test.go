package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	commonconfig "github.com/aegiscore/common/runtime/config"
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
	require.True(t, cfg.Auth.TokenVersionCache.Enabled)
	require.EqualValues(t, 2048, cfg.Auth.TokenVersionCache.Size)
	require.Equal(t, 2*time.Second, cfg.Auth.TokenVersionCache.TTL)
	require.Equal(t, 400*time.Millisecond, cfg.Auth.TokenVersionCache.LoadTimeout)
	require.True(t, cfg.RBAC.UserRoleCache.Enabled)
	require.EqualValues(t, 4096, cfg.RBAC.UserRoleCache.Size)
	require.Equal(t, 7*time.Second, cfg.RBAC.UserRoleCache.TTL)
	require.Equal(t, 600*time.Millisecond, cfg.RBAC.UserRoleCache.LoadTimeout)
	require.True(t, cfg.Auth.RefreshTokenRotation)
	require.Equal(t, 5, cfg.Auth.MaxActiveSessionsPerUser)
	require.True(t, cfg.Ent.Plugins.SQLLog.Enabled)
	require.True(t, cfg.Ent.Plugins.SQLLog.Debug)
	require.Equal(t, 250*time.Millisecond, cfg.Ent.Plugins.SQLLog.SlowThreshold)
	require.False(t, cfg.Ent.Plugins.Tracing.Enabled)
	require.True(t, cfg.Ent.Plugins.Metrics.Enabled)
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

func TestDefaultFeatureCacheConfigReturnsCompleteValue(t *testing.T) {
	cfg := DefaultFeatureCacheConfig(100000, time.Second, 300*time.Millisecond)

	require.True(t, cfg.Enabled)
	require.EqualValues(t, 100000, cfg.Size)
	require.Equal(t, time.Second, cfg.TTL)
	require.Equal(t, 300*time.Millisecond, cfg.LoadTimeout)
}

func TestDefaultConfigReturnsCompleteServiceDefaults(t *testing.T) {
	cfg := DefaultConfig()

	require.Equal(t, commonconfig.DefaultConfig(), cfg.Config)
	require.Equal(t, commonresources.DefaultRedisTimeout, cfg.Resources.Redis[serviceresources.NameCacheRedis].Timeout)
	postgres := cfg.Resources.Postgres[serviceresources.NamePrimaryDB]
	require.Equal(t, commonresources.DefaultPostgresSSLMode, postgres.SSLMode)
	require.Equal(t, commonresources.DefaultPostgresMaxOpenConns, postgres.Pool.MaxOpenConns)
	require.Equal(t, commonresources.DefaultPostgresMaxIdleConns, postgres.Pool.MaxIdleConns)
	require.Equal(t, commonresources.DefaultPostgresConnMaxLifetime, postgres.Pool.ConnMaxLifetime)
	require.Equal(t, commonresources.DefaultPostgresConnMaxIdleTime, postgres.Pool.ConnMaxIdleTime)
	require.Equal(t, DefaultFeatureCacheConfig(100000, time.Second, 300*time.Millisecond), cfg.Auth.TokenVersionCache)
	require.Equal(t, DefaultFeatureCacheConfig(100000, 5*time.Second, 500*time.Millisecond), cfg.RBAC.UserRoleCache)
	require.False(t, cfg.Ent.Plugins.SQLLog.Enabled)
	require.False(t, cfg.Ent.Plugins.SQLLog.Debug)
	require.Equal(t, DefaultEntSlowQueryThreshold, cfg.Ent.Plugins.SQLLog.SlowThreshold)
	require.True(t, cfg.Ent.Plugins.Tracing.Enabled)
	require.False(t, cfg.Ent.Plugins.Metrics.Enabled)
}

func TestFeatureCacheLocalcache(t *testing.T) {
	tests := []struct {
		name string
		size int64
		want uint64
	}{
		{name: "positive", size: 123, want: 123},
		{name: "zero", want: 0},
		{name: "negative", size: -1, want: 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := FeatureCacheConfig{Size: tt.size, TTL: time.Minute, LoadTimeout: time.Second}
			mapped := cfg.Localcache("feature_test")
			require.Equal(t, "feature_test", mapped.Name)
			require.Equal(t, tt.want, mapped.Capacity)
			require.Equal(t, time.Minute, mapped.TTL)
			require.Equal(t, time.Second, mapped.LoadTimeout)
		})
	}
}

func TestLoadAppliesFeatureCacheDefaults(t *testing.T) {
	yaml := strings.Replace(serviceConfigYAML(), "  token_version_cache:\n    enabled: true\n    size: 2048\n    ttl: 2s\n    load_timeout: 400ms\n", "", 1)
	yaml = strings.Replace(yaml, `rbac:
  user_role_cache:
    enabled: true
    size: 4096
    ttl: 7s
    load_timeout: 600ms
`, "", 1)

	cfg := loadServiceConfig(t, yaml)
	require.True(t, cfg.Auth.TokenVersionCache.Enabled)
	require.EqualValues(t, 100000, cfg.Auth.TokenVersionCache.Size)
	require.Equal(t, time.Second, cfg.Auth.TokenVersionCache.TTL)
	require.Equal(t, 300*time.Millisecond, cfg.Auth.TokenVersionCache.LoadTimeout)
	require.True(t, cfg.RBAC.UserRoleCache.Enabled)
	require.EqualValues(t, 100000, cfg.RBAC.UserRoleCache.Size)
	require.Equal(t, 5*time.Second, cfg.RBAC.UserRoleCache.TTL)
	require.Equal(t, 500*time.Millisecond, cfg.RBAC.UserRoleCache.LoadTimeout)
}

func TestLoadAppliesEntPluginDefaults(t *testing.T) {
	yaml := strings.Replace(serviceConfigYAML(), `ent:
  plugins:
    sql_log:
      enabled: true
      debug: true
      slow_threshold: 250ms
    tracing:
      enabled: false
    metrics:
      enabled: true
`, "ent: {}\n", 1)

	cfg := loadServiceConfig(t, yaml)
	require.False(t, cfg.Ent.Plugins.SQLLog.Enabled)
	require.False(t, cfg.Ent.Plugins.SQLLog.Debug)
	require.Equal(t, 500*time.Millisecond, cfg.Ent.Plugins.SQLLog.SlowThreshold)
	require.True(t, cfg.Ent.Plugins.Tracing.Enabled)
	require.False(t, cfg.Ent.Plugins.Metrics.Enabled)
}

func TestLoadParsesEntSlowThresholdDuration(t *testing.T) {
	cfg := loadServiceConfig(t, serviceConfigYAML())
	require.Equal(t, 250*time.Millisecond, cfg.Ent.Plugins.SQLLog.SlowThreshold)
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
	require.False(t, cfg.Auth.TokenVersionCache.Enabled)
	require.EqualValues(t, 100000, cfg.Auth.TokenVersionCache.Size)
	require.Equal(t, time.Second, cfg.Auth.TokenVersionCache.TTL)
	require.Equal(t, 300*time.Millisecond, cfg.Auth.TokenVersionCache.LoadTimeout)
	require.False(t, cfg.RBAC.UserRoleCache.Enabled)
	require.EqualValues(t, 100000, cfg.RBAC.UserRoleCache.Size)
	require.Equal(t, 5*time.Second, cfg.RBAC.UserRoleCache.TTL)
	require.Equal(t, 500*time.Millisecond, cfg.RBAC.UserRoleCache.LoadTimeout)
}

func TestLoadRejectsExplicitInvalidFeatureCacheValue(t *testing.T) {
	yaml := strings.Replace(serviceConfigYAML(), "    size: 2048", "    size: 0", 1)
	err := loadServiceConfigError(t, yaml)
	require.Contains(t, err.Error(), "auth.token_version_cache.size must be > 0 when enabled")
}

func TestValidateFeatureCaches(t *testing.T) {
	t.Run("enabled requires positive values", func(t *testing.T) {
		errs := (FeatureCacheConfig{Enabled: true}).Validate("auth.token_version_cache")
		require.Len(t, errs, 3)
		require.Contains(t, errs[0].Error(), "auth.token_version_cache.size")
	})

	t.Run("disabled allows zero values", func(t *testing.T) {
		errs := (FeatureCacheConfig{Enabled: false}).Validate("rbac.user_role_cache")
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

func TestLoadParsesNestedResourceConfig(t *testing.T) {
	yaml := strings.Replace(serviceConfigYAML(), "      timeout: 7s", "      timeout: 11s", 1)
	cfg := loadServiceConfig(t, yaml)
	require.Equal(t, 11*time.Second, cfg.Resources.Redis[serviceresources.NameCacheRedis].Timeout)
}

func TestLoadPreservesAdditionalNamedResourceAndAppliesDefaults(t *testing.T) {
	yaml := strings.Replace(serviceConfigYAML(), "  postgres:\n", "    queue_redis:\n      addr: 127.0.0.1:6380\n      db: 3\n  postgres:\n", 1)

	cfg := loadServiceConfig(t, yaml)
	require.Len(t, cfg.Resources.Redis, 2)
	require.Equal(t, 7*time.Second, cfg.Resources.Redis[serviceresources.NameCacheRedis].Timeout)
	require.Equal(t, commonresources.DefaultRedisTimeout, cfg.Resources.Redis["queue_redis"].Timeout)
}

func TestLoadRepositoryConfig(t *testing.T) {
	result, err := LoadFromDocuments(loadExampleConfigDocuments(t))
	require.NoError(t, err)
	cfg := result.Config
	require.Equal(t, 60*time.Second, cfg.Runtime.Lifecycle.StartTimeout)
	require.Equal(t, 120*time.Second, cfg.Runtime.Lifecycle.StopTimeout)
	require.True(t, cfg.Server.HTTP.Enabled)
	require.False(t, cfg.Server.GRPC.Enabled)
	require.Contains(t, cfg.Resources.Redis, serviceresources.NameCacheRedis)
	require.Contains(t, cfg.Resources.Postgres, serviceresources.NamePrimaryDB)
}

func loadExampleConfigDocuments(t *testing.T) []commonconfig.ConfigDocument {
	t.Helper()
	var docs []commonconfig.ConfigDocument
	for _, dataID := range []string{"base.yaml", "resources.yaml", "user-service.yaml"} {
		content, err := os.ReadFile(filepath.Join("..", "..", "configs", "examples", dataID))
		require.NoError(t, err)
		docs = append(docs, commonconfig.ConfigDocument{DataID: dataID, Content: content})
	}
	return docs
}

func TestLoadFromDocumentsMergesLayeredServiceConfig(t *testing.T) {
	docs := []commonconfig.ConfigDocument{
		{DataID: "base.yaml", Content: []byte(serviceConfigYAML())},
		{DataID: "user-service.yaml", Content: []byte("log:\n  level: debug\n")},
	}
	result, err := LoadFromDocuments(docs)
	require.NoError(t, err)
	require.Equal(t, "debug", result.Config.Log.Level)
	require.Equal(t, "json", result.Config.Log.Format)
	require.NotEmpty(t, result.Source.Digest)
	settings, err := result.EffectiveSettings()
	require.NoError(t, err)
	rendered, err := commonconfig.RenderYAML(commonconfig.RedactSettings(settings, nil))
	require.NoError(t, err)
	require.NotContains(t, string(rendered), "secret-123456789012345678901234567890")
	require.Contains(t, string(rendered), "***")
}

func TestEffectiveSettingsContainsDefaultsWithoutChangingSourceDigest(t *testing.T) {
	yaml := strings.Replace(serviceConfigYAML(), "  token_version_cache:\n    enabled: true\n    size: 2048\n    ttl: 2s\n    load_timeout: 400ms\n", "", 1)
	yaml = strings.Replace(yaml, "rbac:\n  user_role_cache:\n    enabled: true\n    size: 4096\n    ttl: 7s\n    load_timeout: 600ms\n", "", 1)
	yaml = strings.Replace(yaml, "ent:\n  plugins:\n    sql_log:\n      enabled: true\n      debug: true\n      slow_threshold: 250ms\n    tracing:\n      enabled: false\n    metrics:\n      enabled: true\n", "ent: {}\n", 1)
	yaml = strings.Replace(yaml, "      timeout: 7s\n", "", 1)
	yaml = strings.Replace(yaml, "      sslmode: disable\n      pool:\n        max_open_conns: 20\n        max_idle_conns: 4\n        conn_max_lifetime: 45m\n        conn_max_idle_time: 12m\n", "", 1)
	docs := []commonconfig.ConfigDocument{{DataID: "test.yaml", Content: []byte(yaml)}}
	rawSettings, err := commonconfig.DeepMergeYAML(docs)
	require.NoError(t, err)
	wantDigest, err := commonconfig.DigestSettings(rawSettings)
	require.NoError(t, err)

	result, err := DecodeSettings(rawSettings, commonconfig.SourceMetadata{Provider: "test"})
	require.NoError(t, err)
	settings, err := result.EffectiveSettings()
	require.NoError(t, err)

	require.Equal(t, wantDigest, result.Source.Digest)
	auth := settings["auth"].(map[string]any)
	cache := auth["token_version_cache"].(map[string]any)
	require.Equal(t, true, cache["enabled"])
	require.EqualValues(t, 100000, cache["size"])
	require.Equal(t, "1s", cache["ttl"])
	require.Equal(t, "300ms", cache["load_timeout"])
	rbac := settings["rbac"].(map[string]any)
	userRoleCache := rbac["user_role_cache"].(map[string]any)
	require.Equal(t, true, userRoleCache["enabled"])
	require.EqualValues(t, 100000, userRoleCache["size"])
	require.Equal(t, "5s", userRoleCache["ttl"])
	require.Equal(t, "500ms", userRoleCache["load_timeout"])
	ent := settings["ent"].(map[string]any)
	plugins := ent["plugins"].(map[string]any)
	require.Equal(t, "500ms", plugins["sql_log"].(map[string]any)["slow_threshold"])
	require.Equal(t, true, plugins["tracing"].(map[string]any)["enabled"])
	resources := settings["resources"].(map[string]any)
	redis := resources["redis"].(map[string]any)
	require.Equal(t, "5s", redis[serviceresources.NameCacheRedis].(map[string]any)["timeout"])
	postgres := resources["postgres"].(map[string]any)[serviceresources.NamePrimaryDB].(map[string]any)
	require.Equal(t, commonresources.DefaultPostgresSSLMode, postgres["sslmode"])
	require.EqualValues(t, commonresources.DefaultPostgresMaxOpenConns, postgres["pool"].(map[string]any)["max_open_conns"])

	result.Config.Log.Level = "warn"
	settings, err = result.EffectiveSettings()
	require.NoError(t, err)
	require.Equal(t, "warn", settings["log"].(map[string]any)["level"])
}

func TestLoadRejectsLegacyPasswordKDFConfig(t *testing.T) {
	yaml := strings.Replace(serviceConfigYAML(), "  token_version_cache:\n", "  password_kdf:\n    argon2_concurrency: 2\n    argon2_queue_size: 16\n  token_version_cache:\n", 1)
	err := loadServiceConfigError(t, yaml)
	require.Contains(t, err.Error(), "unknown configuration keys: auth.password_kdf.argon2_concurrency, auth.password_kdf.argon2_queue_size")
}

func TestLoadRejectsBootstrapSuperAdminConfig(t *testing.T) {
	yaml := strings.Replace(serviceConfigYAML(), "rbac:\n", "rbac:\n  bootstrap_super_admin:\n    password: bootstrap-secret-value\n", 1)
	err := loadServiceConfigError(t, yaml)
	require.Contains(t, err.Error(), "unknown configuration keys: rbac.bootstrap_super_admin.password")
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
	result, err := LoadFromDocuments([]commonconfig.ConfigDocument{{DataID: "test.yaml", Content: []byte(content)}})
	require.NoError(t, err)
	return result.Config
}

func loadServiceConfigError(t *testing.T, content string) error {
	t.Helper()
	_, err := LoadFromDocuments([]commonconfig.ConfigDocument{{DataID: "test.yaml", Content: []byte(content)}})
	require.Error(t, err)
	return err
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
  timezone: UTC
auth:
  jwt:
    secret: secret-123456789012345678901234567890
    issuer: aegiscore-test
    audience: aegiscore-users
    access_token_ttl: 15m
    refresh_token_ttl: 168h
    password_change_token_ttl: 5m
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
  plugins:
    sql_log:
      enabled: true
      debug: true
      slow_threshold: 250ms
    tracing:
      enabled: false
    metrics:
      enabled: true
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
