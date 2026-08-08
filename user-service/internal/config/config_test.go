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

// TestLoadParsesServicePrivateConfig 验证 user-service 私有配置字段能被完整解析。
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
	require.Equal(t, 2*time.Second, cfg.RBAC.OutboxDispatcher.PollInterval)
	require.Equal(t, 25, cfg.RBAC.OutboxDispatcher.BatchSize)
	require.Equal(t, 45*time.Second, cfg.RBAC.OutboxDispatcher.ClaimTimeout)
	require.Equal(t, 3*time.Second, cfg.RBAC.OutboxDispatcher.RetryBackoff.Initial)
	require.Equal(t, 2*time.Minute, cfg.RBAC.OutboxDispatcher.RetryBackoff.Max)
	require.Equal(t, 10*time.Second, cfg.RBAC.PolicyWatcher.CheckInterval)
	require.Equal(t, 4*time.Second, cfg.RBAC.PolicyWatcher.SubscribeTimeout)
	require.Equal(t, 35*time.Second, cfg.RBAC.PolicyWatcher.MaxStaleness)
	require.Equal(t, 500*time.Millisecond, cfg.RBAC.PolicyWatcher.RetryBackoff.Initial)
	require.Equal(t, 20*time.Second, cfg.RBAC.PolicyWatcher.RetryBackoff.Max)
	require.True(t, cfg.Auth.RefreshTokenRotation)
	require.Equal(t, 5, cfg.Auth.MaxActiveSessionsPerUser)
	require.True(t, cfg.APIRateLimit.Anonymous.Enabled)
	require.Equal(t, 2.5, cfg.APIRateLimit.Anonymous.RatePerSecond)
	require.Equal(t, 7, cfg.APIRateLimit.Anonymous.Burst)
	require.Equal(t, 8192, cfg.APIRateLimit.Anonymous.MaxKeys)
	require.Equal(t, "overflow", cfg.APIRateLimit.Anonymous.CapacityPolicy)
	require.Equal(t, 11*time.Minute, cfg.APIRateLimit.Anonymous.KeyTTL)
	require.Equal(t, 15*time.Second, cfg.APIRateLimit.Anonymous.CleanupInterval)
	require.Equal(t, 32, cfg.APIRateLimit.Anonymous.Shards)
	require.True(t, cfg.APIRateLimit.Authenticated.Enabled)
	require.Equal(t, 8.0, cfg.APIRateLimit.Authenticated.RatePerSecond)
	require.Equal(t, 30, cfg.APIRateLimit.Authenticated.Burst)
	require.Equal(t, 16384, cfg.APIRateLimit.Authenticated.MaxKeys)
	require.Equal(t, "reject", cfg.APIRateLimit.Authenticated.CapacityPolicy)
	require.Equal(t, 12*time.Minute, cfg.APIRateLimit.Authenticated.KeyTTL)
	require.Equal(t, 20*time.Second, cfg.APIRateLimit.Authenticated.CleanupInterval)
	require.Equal(t, 64, cfg.APIRateLimit.Authenticated.Shards)
	require.EqualValues(t, 32768, cfg.HTTP.RequestBodyMaxBytes)
	require.True(t, cfg.Ent.Plugins.SQLLog.Enabled)
	require.True(t, cfg.Ent.Plugins.SQLLog.Debug)
	require.Equal(t, 250*time.Millisecond, cfg.Ent.Plugins.SQLLog.SlowThreshold)
	require.False(t, cfg.Ent.Plugins.Tracing.Enabled)
	require.True(t, cfg.Ent.Plugins.Metrics.Enabled)
	require.Len(t, cfg.Resources.Redis, 1)
	require.Equal(t, commonresources.RedisModeCluster, cfg.Resources.Redis[serviceresources.NameCacheRedis].Mode)
	require.Equal(t, []string{"127.0.0.1:6379"}, cfg.Resources.Redis[serviceresources.NameCacheRedis].Addrs)
	require.Equal(t, 7*time.Second, cfg.Resources.Redis[serviceresources.NameCacheRedis].Timeout)
	require.Equal(t, 8, cfg.Resources.Redis[serviceresources.NameCacheRedis].Cluster.MaxRedirects)
	require.Len(t, cfg.Resources.Postgres, 1)
	require.Equal(t, 20, cfg.Resources.Postgres[serviceresources.NamePrimaryDB].Pool.MaxOpenConns)
	runtime := cfg.RuntimeConfig()
	require.Equal(t, "aegiscore-test", runtime.App.Name)
	require.Equal(t, 21*time.Second, runtime.Runtime.Lifecycle.StartTimeout)
	require.Equal(t, 50*time.Second, runtime.Runtime.Lifecycle.StopTimeout)
}

// TestDefaultFeatureCacheConfigReturnsCompleteValue 验证 feature cache 默认值构造器返回完整启用配置。
func TestDefaultFeatureCacheConfigReturnsCompleteValue(t *testing.T) {
	cfg := DefaultFeatureCacheConfig(100000, time.Second, 300*time.Millisecond)

	require.True(t, cfg.Enabled)
	require.EqualValues(t, 100000, cfg.Size)
	require.Equal(t, time.Second, cfg.TTL)
	require.Equal(t, 300*time.Millisecond, cfg.LoadTimeout)
}

// TestDefaultConfigReturnsCompleteServiceDefaults 验证 user-service 默认配置覆盖认证、RBAC、资源和观测插件。
func TestDefaultConfigReturnsCompleteServiceDefaults(t *testing.T) {
	cfg := DefaultConfig()

	require.Equal(t, commonconfig.DefaultConfig(), cfg.Config)
	require.Equal(t, commonresources.RedisModeCluster, cfg.Resources.Redis[serviceresources.NameCacheRedis].Mode)
	require.Equal(t, commonresources.DefaultRedisTimeout, cfg.Resources.Redis[serviceresources.NameCacheRedis].Timeout)
	require.Equal(t, commonresources.DefaultRedisClusterMaxRedirects, cfg.Resources.Redis[serviceresources.NameCacheRedis].Cluster.MaxRedirects)
	postgres := cfg.Resources.Postgres[serviceresources.NamePrimaryDB]
	require.Equal(t, commonresources.DefaultPostgresSSLMode, postgres.SSLMode)
	require.Equal(t, commonresources.DefaultPostgresMaxOpenConns, postgres.Pool.MaxOpenConns)
	require.Equal(t, commonresources.DefaultPostgresMaxIdleConns, postgres.Pool.MaxIdleConns)
	require.Equal(t, commonresources.DefaultPostgresConnMaxLifetime, postgres.Pool.ConnMaxLifetime)
	require.Equal(t, commonresources.DefaultPostgresConnMaxIdleTime, postgres.Pool.ConnMaxIdleTime)
	require.Equal(t, DefaultFeatureCacheConfig(100000, time.Second, 300*time.Millisecond), cfg.Auth.TokenVersionCache)
	require.Equal(t, DefaultFeatureCacheConfig(100000, 5*time.Second, 500*time.Millisecond), cfg.RBAC.UserRoleCache)
	require.Equal(t, OutboxDispatcherConfig{
		PollInterval: time.Second,
		BatchSize:    100,
		ClaimTimeout: 30 * time.Second,
		RetryBackoff: RetryBackoffConfig{Initial: time.Second, Max: time.Minute},
	}, cfg.RBAC.OutboxDispatcher)
	require.Equal(t, DefaultPolicyWatcherConfig(), cfg.RBAC.PolicyWatcher)
	require.Equal(t, DefaultRateLimitPolicyConfig(1, 5, 10*time.Minute, 30*time.Second, 64), cfg.APIRateLimit.Anonymous)
	require.Equal(t, DefaultRateLimitPolicyConfig(5, 20, 10*time.Minute, 30*time.Second, 128), cfg.APIRateLimit.Authenticated)
	require.Equal(t, HTTPConfig{RequestBodyMaxBytes: DefaultHTTPRequestBodyMaxBytes}, cfg.HTTP)
	require.False(t, cfg.Ent.Plugins.SQLLog.Enabled)
	require.False(t, cfg.Ent.Plugins.SQLLog.Debug)
	require.Equal(t, DefaultEntSlowQueryThreshold, cfg.Ent.Plugins.SQLLog.SlowThreshold)
	require.True(t, cfg.Ent.Plugins.Tracing.Enabled)
	require.False(t, cfg.Ent.Plugins.Metrics.Enabled)
}

// TestFeatureCacheLocalcache 验证服务私有 feature cache 可映射为通用 localcache 配置。
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

// TestLoadAppliesFeatureCacheDefaults 验证缺省 feature cache 会补齐服务默认值。
func TestLoadAppliesFeatureCacheDefaults(t *testing.T) {
	yaml := strings.Replace(serviceConfigYAML(), "  token_version_cache:\n    enabled: true\n    size: 2048\n    ttl: 2s\n    load_timeout: 400ms\n", "", 1)
	yaml = strings.Replace(yaml, `rbac:
  user_role_cache:
    enabled: true
    size: 4096
    ttl: 7s
    load_timeout: 600ms
`, "rbac:\n", 1)

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

// TestLoadAppliesOutboxDispatcherDefaults 验证 RBAC outbox dispatcher 缺省配置会被补齐。
func TestLoadAppliesOutboxDispatcherDefaults(t *testing.T) {
	yaml := strings.Replace(serviceConfigYAML(), `  outbox_dispatcher:
    poll_interval: 2s
    batch_size: 25
    claim_timeout: 45s
    retry_backoff:
      initial: 3s
      max: 2m
`, "", 1)

	cfg := loadServiceConfig(t, yaml)
	require.Equal(t, DefaultOutboxDispatcherConfig(), cfg.RBAC.OutboxDispatcher)
}

// TestValidateOutboxDispatcher 验证 outbox dispatcher 配置边界和字段路径错误。
func TestValidateOutboxDispatcher(t *testing.T) {
	t.Run("requires positive values", func(t *testing.T) {
		errs := (OutboxDispatcherConfig{}).Validate("rbac.outbox_dispatcher")
		require.Len(t, errs, 5)
		require.Contains(t, errs[0].Error(), "rbac.outbox_dispatcher.poll_interval")
		require.Contains(t, errs[1].Error(), "rbac.outbox_dispatcher.batch_size")
		require.Contains(t, errs[2].Error(), "rbac.outbox_dispatcher.claim_timeout")
		require.Contains(t, errs[3].Error(), "rbac.outbox_dispatcher.retry_backoff.initial")
		require.Contains(t, errs[4].Error(), "rbac.outbox_dispatcher.retry_backoff.max")
	})

	t.Run("max backoff must not be less than initial", func(t *testing.T) {
		cfg := DefaultOutboxDispatcherConfig()
		cfg.RetryBackoff.Initial = 2 * time.Minute
		cfg.RetryBackoff.Max = time.Minute
		errs := cfg.Validate("rbac.outbox_dispatcher")
		require.Len(t, errs, 1)
		require.Contains(t, errs[0].Error(), "rbac.outbox_dispatcher.retry_backoff.max must be >= retry_backoff.initial")
	})

	t.Run("accepts complete defaults", func(t *testing.T) {
		require.Empty(t, DefaultOutboxDispatcherConfig().Validate("rbac.outbox_dispatcher"))
	})
}

// TestLoadAppliesPolicyWatcherDefaults 验证 RBAC policy watcher 缺省配置会被补齐。
func TestLoadAppliesPolicyWatcherDefaults(t *testing.T) {
	yaml := strings.Replace(serviceConfigYAML(), `  policy_watcher:
    check_interval: 10s
    subscribe_timeout: 4s
    max_staleness: 35s
    retry_backoff:
      initial: 500ms
      max: 20s
`, "", 1)

	cfg := loadServiceConfig(t, yaml)
	require.Equal(t, DefaultPolicyWatcherConfig(), cfg.RBAC.PolicyWatcher)
}

// TestValidatePolicyWatcher 验证 policy watcher 校准、新鲜度和重试退避边界。
func TestValidatePolicyWatcher(t *testing.T) {
	t.Run("requires positive values", func(t *testing.T) {
		errs := (PolicyWatcherConfig{}).Validate("rbac.policy_watcher")
		require.Len(t, errs, 5)
		for _, field := range []string{"check_interval", "subscribe_timeout", "max_staleness", "retry_backoff.initial", "retry_backoff.max"} {
			require.Condition(t, func() bool {
				for _, err := range errs {
					if strings.Contains(err.Error(), "rbac.policy_watcher."+field) {
						return true
					}
				}
				return false
			})
		}
	})

	t.Run("rejects invalid ordering", func(t *testing.T) {
		cfg := DefaultPolicyWatcherConfig()
		cfg.MaxStaleness = cfg.CheckInterval
		cfg.RetryBackoff.Max = cfg.RetryBackoff.Initial / 2
		errs := cfg.Validate("rbac.policy_watcher")
		require.Len(t, errs, 2)
		require.Contains(t, errs[0].Error()+errs[1].Error(), "retry_backoff.max must be >= retry_backoff.initial")
		require.Contains(t, errs[0].Error()+errs[1].Error(), "max_staleness must be > check_interval")
	})

	t.Run("accepts complete defaults", func(t *testing.T) {
		require.Empty(t, DefaultPolicyWatcherConfig().Validate("rbac.policy_watcher"))
	})
}

// TestLoadAppliesAPIRateLimitDefaults 验证 API rate limit 缺省策略会被补齐。
func TestLoadAppliesAPIRateLimitDefaults(t *testing.T) {
	yaml := strings.Replace(serviceConfigYAML(), `api_rate_limit:
  anonymous:
    enabled: true
    rate_per_second: 2.5
    burst: 7
    max_keys: 8192
    capacity_policy: overflow
    key_ttl: 11m
    cleanup_interval: 15s
    shards: 32
  authenticated:
    enabled: true
    rate_per_second: 8
    burst: 30
    max_keys: 16384
    capacity_policy: reject
    key_ttl: 12m
    cleanup_interval: 20s
    shards: 64
`, "", 1)

	cfg := loadServiceConfig(t, yaml)
	require.Equal(t, DefaultRateLimitPolicyConfig(1, 5, 10*time.Minute, 30*time.Second, 64), cfg.APIRateLimit.Anonymous)
	require.Equal(t, DefaultRateLimitPolicyConfig(5, 20, 10*time.Minute, 30*time.Second, 128), cfg.APIRateLimit.Authenticated)
}

// TestLoadHTTPConfig 验证 user-service 私有 HTTP 配置加载和严格字段拒绝。
func TestLoadHTTPConfig(t *testing.T) {
	t.Run("applies default when omitted", func(t *testing.T) {
		yaml := strings.Replace(serviceConfigYAML(), "http:\n  request_body_max_bytes: 32768\n", "", 1)
		cfg := loadServiceConfig(t, yaml)
		require.EqualValues(t, DefaultHTTPRequestBodyMaxBytes, cfg.HTTP.RequestBodyMaxBytes)
	})

	for _, value := range []string{"0", "-1"} {
		t.Run("rejects "+value, func(t *testing.T) {
			yaml := strings.Replace(serviceConfigYAML(), "request_body_max_bytes: 32768", "request_body_max_bytes: "+value, 1)
			err := loadServiceConfigError(t, yaml)
			require.Contains(t, err.Error(), "http.request_body_max_bytes must be > 0")
		})
	}

	t.Run("rejects unknown field", func(t *testing.T) {
		yaml := strings.Replace(serviceConfigYAML(), "  request_body_max_bytes: 32768", "  request_body_max_bytes: 32768\n  unknown: true", 1)
		err := loadServiceConfigError(t, yaml)
		require.Contains(t, err.Error(), "unknown configuration keys: http.unknown")
	})
}

// TestLoadPreservesDisabledAPIRateLimit 验证禁用限流时保留禁用状态且不校验无关字段。
func TestLoadPreservesDisabledAPIRateLimit(t *testing.T) {
	yaml := strings.Replace(serviceConfigYAML(), `api_rate_limit:
  anonymous:
    enabled: true
    rate_per_second: 2.5
    burst: 7
    max_keys: 8192
    capacity_policy: overflow
    key_ttl: 11m
    cleanup_interval: 15s
    shards: 32
  authenticated:
    enabled: true
    rate_per_second: 8
    burst: 30
    max_keys: 16384
    capacity_policy: reject
    key_ttl: 12m
    cleanup_interval: 20s
    shards: 64
`, `api_rate_limit:
  anonymous:
    enabled: false
  authenticated:
    enabled: false
`, 1)

	cfg := loadServiceConfig(t, yaml)
	require.False(t, cfg.APIRateLimit.Anonymous.Enabled)
	require.Equal(t, 1.0, cfg.APIRateLimit.Anonymous.RatePerSecond)
	require.False(t, cfg.APIRateLimit.Authenticated.Enabled)
	require.Equal(t, 5.0, cfg.APIRateLimit.Authenticated.RatePerSecond)
}

// TestValidateAPIRateLimit 验证启用限流策略时的速率、burst 和清理配置边界。
func TestValidateAPIRateLimit(t *testing.T) {
	t.Run("enabled requires positive values", func(t *testing.T) {
		errs := (RateLimitPolicyConfig{Enabled: true}).Validate("api_rate_limit.anonymous")
		require.Len(t, errs, 7)
		require.Contains(t, errs[0].Error(), "api_rate_limit.anonymous.rate_per_second")
		require.Contains(t, errs[1].Error(), "api_rate_limit.anonymous.burst")
		require.Contains(t, errs[2].Error(), "api_rate_limit.anonymous.max_keys")
		require.Contains(t, errs[3].Error(), "api_rate_limit.anonymous.capacity_policy")
		require.Contains(t, errs[4].Error(), "api_rate_limit.anonymous.key_ttl")
		require.Contains(t, errs[5].Error(), "api_rate_limit.anonymous.cleanup_interval")
		require.Contains(t, errs[6].Error(), "api_rate_limit.anonymous.shards")
	})

	t.Run("rejects invalid capacity policy", func(t *testing.T) {
		cfg := DefaultRateLimitPolicyConfig(1, 1, time.Minute, time.Hour, 1)
		cfg.CapacityPolicy = "drop"
		errs := cfg.Validate("api_rate_limit.anonymous")
		require.Len(t, errs, 1)
		require.Contains(t, errs[0].Error(), "api_rate_limit.anonymous.capacity_policy")
	})

	t.Run("disabled allows zero values", func(t *testing.T) {
		errs := (RateLimitPolicyConfig{Enabled: false}).Validate("api_rate_limit.authenticated")
		require.Empty(t, errs)
	})
}

// TestLoadAppliesEntPluginDefaults 验证 Ent 插件默认值会被补齐。
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

// TestLoadParsesEntSlowThresholdDuration 验证 Ent SQL 慢查询阈值按 duration 解析。
func TestLoadParsesEntSlowThresholdDuration(t *testing.T) {
	cfg := loadServiceConfig(t, serviceConfigYAML())
	require.Equal(t, 250*time.Millisecond, cfg.Ent.Plugins.SQLLog.SlowThreshold)
}

// TestLoadPreservesExplicitDisabledFeatureCaches 验证显式禁用的 feature cache 不会被默认值重新启用。
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

// TestLoadRejectsExplicitInvalidFeatureCacheValue 验证启用 cache 时非法容量会被拒绝。
func TestLoadRejectsExplicitInvalidFeatureCacheValue(t *testing.T) {
	yaml := strings.Replace(serviceConfigYAML(), "    size: 2048", "    size: 0", 1)
	err := loadServiceConfigError(t, yaml)
	require.Contains(t, err.Error(), "auth.token_version_cache.size must be > 0 when enabled")
}

// TestValidateFeatureCaches 验证 token-version 与 RBAC feature cache 的字段校验。
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

// TestLoadAppliesResourceDefaultsBeforeValidation 验证具名资源先补默认值再执行校验。
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

// TestLoadParsesNestedResourceConfig 验证嵌套 Redis/PostgreSQL 资源配置被正确解析。
func TestLoadParsesNestedResourceConfig(t *testing.T) {
	yaml := strings.Replace(serviceConfigYAML(), "      timeout: 7s", "      timeout: 11s", 1)
	cfg := loadServiceConfig(t, yaml)
	require.Equal(t, 11*time.Second, cfg.Resources.Redis[serviceresources.NameCacheRedis].Timeout)
}

// TestLoadPreservesAdditionalNamedResourceAndAppliesDefaults 验证额外具名资源会保留并应用通用默认值。
func TestLoadPreservesAdditionalNamedResourceAndAppliesDefaults(t *testing.T) {
	yaml := strings.Replace(serviceConfigYAML(), "  postgres:\n", "    queue_redis:\n      mode: cluster\n      addrs:\n        - 127.0.0.1:6380\n  postgres:\n", 1)

	cfg := loadServiceConfig(t, yaml)
	require.Len(t, cfg.Resources.Redis, 2)
	require.Equal(t, 7*time.Second, cfg.Resources.Redis[serviceresources.NameCacheRedis].Timeout)
	require.Equal(t, commonresources.DefaultRedisTimeout, cfg.Resources.Redis["queue_redis"].Timeout)
}

// TestLoadRepositoryConfigTargets 验证仓库内 Nacos 配置目录的目标环境保持一致。
func TestLoadRepositoryConfigTargets(t *testing.T) {
	tests := []struct {
		name         string
		environment  string
		redisAddr    string
		postgresHost string
		postgresPort int
		otlpEndpoint string
	}{
		{name: "host", environment: "local-host", redisAddr: "127.0.0.1:6379", postgresHost: "127.0.0.1", postgresPort: 5432, otlpEndpoint: "127.0.0.1:4317"},
		{name: "docker", environment: "local-docker", redisAddr: "redis:6379", postgresHost: "postgres", postgresPort: 5432, otlpEndpoint: "jaeger:4317"},
	}
	configs := make(map[string]*Config, len(tests))
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := loadFromDocumentsForTest(loadRepositoryConfigDocuments(t, tt.environment))
			require.NoError(t, err)
			cfg := result.Config
			require.Equal(t, 60*time.Second, cfg.Runtime.Lifecycle.StartTimeout)
			require.Equal(t, 120*time.Second, cfg.Runtime.Lifecycle.StopTimeout)
			require.True(t, cfg.Server.HTTP.Enabled)
			require.False(t, cfg.Server.GRPC.Enabled)
			require.Equal(t, commonresources.RedisModeStandalone, cfg.Resources.Redis[serviceresources.NameCacheRedis].Mode)
			require.Equal(t, tt.redisAddr, cfg.Resources.Redis[serviceresources.NameCacheRedis].Addr)
			require.Empty(t, cfg.Resources.Redis[serviceresources.NameCacheRedis].Addrs)
			postgres := cfg.Resources.Postgres[serviceresources.NamePrimaryDB]
			require.Equal(t, tt.postgresHost, postgres.Host)
			require.Equal(t, tt.postgresPort, postgres.Port)
			require.Equal(t, tt.otlpEndpoint, cfg.Observability.Tracing.OTLPEndpoint)
			require.Equal(t, "json", cfg.Log.Format)
			configs[tt.name] = cfg
		})
	}

	host := *configs["host"]
	hostRedis := host.Resources.Redis[serviceresources.NameCacheRedis]
	dockerRedis := configs["docker"].Resources.Redis[serviceresources.NameCacheRedis]
	hostRedis.Addr = dockerRedis.Addr
	hostRedis.Addrs = dockerRedis.Addrs
	host.Resources.Redis[serviceresources.NameCacheRedis] = hostRedis
	hostPostgres := host.Resources.Postgres[serviceresources.NamePrimaryDB]
	dockerPostgres := configs["docker"].Resources.Postgres[serviceresources.NamePrimaryDB]
	hostPostgres.Host = dockerPostgres.Host
	hostPostgres.Port = dockerPostgres.Port
	host.Resources.Postgres[serviceresources.NamePrimaryDB] = hostPostgres
	host.Observability.Tracing.OTLPEndpoint = configs["docker"].Observability.Tracing.OTLPEndpoint
	require.Equal(t, *configs["docker"], host, "host 与 Docker 配置除运行位置端点外必须一致")
}

// loadRepositoryConfigDocuments 读取指定环境的 base/resources/user-service 三份仓库配置文档。
func loadRepositoryConfigDocuments(t *testing.T, environment string) []commonconfig.ConfigDocument {
	t.Helper()
	var docs []commonconfig.ConfigDocument
	for _, dataID := range []string{"base.yaml", "resources.yaml", "user-service.yaml"} {
		content, err := os.ReadFile(filepath.Join("..", "..", "..", "deployments", "nacos", environment, dataID))
		require.NoError(t, err)
		docs = append(docs, commonconfig.ConfigDocument{DataID: dataID, Content: content})
	}
	return docs
}

// TestLoadFromTestDocumentsMergesLayeredServiceConfig 验证分层文档合并、effective render 脱敏和输入不变。
func TestLoadFromTestDocumentsMergesLayeredServiceConfig(t *testing.T) {
	// 为 render 断言显式注入非空资源密码，避免只覆盖 JWT secret 而遗漏具名资源凭据。
	baseYAML := strings.Replace(serviceConfigYAML(), "      mode: cluster\n", "      mode: cluster\n      password: redis-secret-123\n", 1)
	baseYAML = strings.Replace(baseYAML, "      password: \"\"", "      password: postgres-secret-123", 1)
	docs := []commonconfig.ConfigDocument{
		{DataID: "base.yaml", Content: []byte(baseYAML)},
		{DataID: "user-service.yaml", Content: []byte("log:\n  level: debug\n")},
	}
	result, err := loadFromDocumentsForTest(docs)
	require.NoError(t, err)
	require.Equal(t, "debug", result.Config.Log.Level)
	require.Equal(t, "json", result.Config.Log.Format)
	require.NotEmpty(t, result.Source.Digest)
	settings, err := result.EffectiveSettings()
	require.NoError(t, err)
	// RedactEffectiveSettings 必须返回副本；后续断言同时检查输出已脱敏、原 settings 未被修改。
	redacted := RedactEffectiveSettings(settings)
	rendered, err := commonconfig.RenderYAML(redacted)
	require.NoError(t, err)
	require.NotContains(t, string(rendered), "secret-123456789012345678901234567890")
	require.NotContains(t, string(rendered), "redis-secret-123")
	require.NotContains(t, string(rendered), "postgres-secret-123")
	require.Contains(t, string(rendered), "***")
	require.Equal(t, "secret-123456789012345678901234567890", settings["auth"].(map[string]any)["jwt"].(map[string]any)["secret"])
	require.Equal(t, "redis-secret-123", settings["resources"].(map[string]any)["redis"].(map[string]any)["cache_redis"].(map[string]any)["password"])
	require.Equal(t, "postgres-secret-123", settings["resources"].(map[string]any)["postgres"].(map[string]any)["primary_db"].(map[string]any)["password"])
	require.Equal(t, "***", redacted["auth"].(map[string]any)["jwt"].(map[string]any)["secret"])
	require.Equal(t, "***", redacted["resources"].(map[string]any)["redis"].(map[string]any)["cache_redis"].(map[string]any)["password"])
	require.Equal(t, "***", redacted["resources"].(map[string]any)["postgres"].(map[string]any)["primary_db"].(map[string]any)["password"])
}

// TestSensitiveConfigPathsReturnsCopy 验证服务敏感路径 API 不暴露内部 slice 所有权。
func TestSensitiveConfigPathsReturnsCopy(t *testing.T) {
	paths := SensitiveConfigPaths()
	require.Contains(t, paths, "auth.jwt.secret")
	// 调用方拿到的是副本，外部修改不能改变服务级脱敏策略。
	paths[0] = "mutated.path"
	require.NotEqual(t, paths[0], SensitiveConfigPaths()[0])
}

// TestEffectiveSettingsContainsDefaultsWithoutChangingSourceDigest 验证 effective settings 补默认值不反写 raw digest。
func TestEffectiveSettingsContainsDefaultsWithoutChangingSourceDigest(t *testing.T) {
	yaml := strings.Replace(serviceConfigYAML(), "  token_version_cache:\n    enabled: true\n    size: 2048\n    ttl: 2s\n    load_timeout: 400ms\n", "", 1)
	yaml = strings.Replace(yaml, "rbac:\n  user_role_cache:\n    enabled: true\n    size: 4096\n    ttl: 7s\n    load_timeout: 600ms\n", "", 1)
	yaml = strings.Replace(yaml, `  outbox_dispatcher:
    poll_interval: 2s
    batch_size: 25
    claim_timeout: 45s
    retry_backoff:
      initial: 3s
      max: 2m
`, "", 1)
	yaml = strings.Replace(yaml, `  policy_watcher:
    check_interval: 10s
    subscribe_timeout: 4s
    max_staleness: 35s
    retry_backoff:
      initial: 500ms
      max: 20s
`, "", 1)
	yaml = strings.Replace(yaml, `api_rate_limit:
  anonymous:
    enabled: true
    rate_per_second: 2.5
    burst: 7
    max_keys: 8192
    capacity_policy: overflow
    key_ttl: 11m
    cleanup_interval: 15s
    shards: 32
  authenticated:
    enabled: true
    rate_per_second: 8
    burst: 30
    max_keys: 16384
    capacity_policy: reject
    key_ttl: 12m
    cleanup_interval: 20s
    shards: 64
`, "", 1)
	yaml = strings.Replace(yaml, "ent:\n  plugins:\n    sql_log:\n      enabled: true\n      debug: true\n      slow_threshold: 250ms\n    tracing:\n      enabled: false\n    metrics:\n      enabled: true\n", "ent: {}\n", 1)
	yaml = strings.Replace(yaml, "      timeout: 7s\n", "", 1)
	yaml = strings.Replace(yaml, "      sslmode: disable\n      pool:\n        max_open_conns: 20\n        max_idle_conns: 4\n        conn_max_lifetime: 45m\n        conn_max_idle_time: 12m\n", "", 1)
	docs := []commonconfig.ConfigDocument{{DataID: "test.yaml", Content: []byte(yaml)}}
	rawSettings, err := commonconfig.DeepMergeYAML(docs)
	require.NoError(t, err)
	wantDigest, err := commonconfig.DigestSettings(rawSettings)
	require.NoError(t, err)

	result, err := DecodeSettings(rawSettings, commonconfig.SourceMetadata{Provider: "testkit"})
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
	outboxDispatcher := rbac["outbox_dispatcher"].(map[string]any)
	require.Equal(t, "1s", outboxDispatcher["poll_interval"])
	require.EqualValues(t, 100, outboxDispatcher["batch_size"])
	require.Equal(t, "30s", outboxDispatcher["claim_timeout"])
	retryBackoff := outboxDispatcher["retry_backoff"].(map[string]any)
	require.Equal(t, "1s", retryBackoff["initial"])
	require.Equal(t, "1m0s", retryBackoff["max"])
	policyWatcher := rbac["policy_watcher"].(map[string]any)
	require.Equal(t, "15s", policyWatcher["check_interval"])
	require.Equal(t, "5s", policyWatcher["subscribe_timeout"])
	require.Equal(t, "45s", policyWatcher["max_staleness"])
	watcherBackoff := policyWatcher["retry_backoff"].(map[string]any)
	require.Equal(t, "250ms", watcherBackoff["initial"])
	require.Equal(t, "30s", watcherBackoff["max"])
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
	apiRateLimit := settings["api_rate_limit"].(map[string]any)
	anonymous := apiRateLimit["anonymous"].(map[string]any)
	require.Equal(t, true, anonymous["enabled"])
	require.Equal(t, 1.0, anonymous["rate_per_second"])
	require.EqualValues(t, 5, anonymous["burst"])
	require.Equal(t, "10m0s", anonymous["key_ttl"])

	result.Config.Log.Level = "warn"
	settings, err = result.EffectiveSettings()
	require.NoError(t, err)
	require.Equal(t, "warn", settings["log"].(map[string]any)["level"])
}

// TestLoadRejectsLegacyPasswordKDFConfig 验证旧 password_kdf 配置不再被接受。
func TestLoadRejectsLegacyPasswordKDFConfig(t *testing.T) {
	yaml := strings.Replace(serviceConfigYAML(), "  token_version_cache:\n", "  password_kdf:\n    argon2_concurrency: 2\n    argon2_queue_size: 16\n  token_version_cache:\n", 1)
	err := loadServiceConfigError(t, yaml)
	require.Contains(t, err.Error(), "unknown configuration keys: auth.password_kdf.argon2_concurrency, auth.password_kdf.argon2_queue_size")
}

// TestLoadRejectsBootstrapSuperAdminConfig 验证 bootstrap 超级管理员密码不得进入运行时配置。
func TestLoadRejectsBootstrapSuperAdminConfig(t *testing.T) {
	yaml := strings.Replace(serviceConfigYAML(), "rbac:\n", "rbac:\n  bootstrap_super_admin:\n    password: bootstrap-secret-value\n", 1)
	err := loadServiceConfigError(t, yaml)
	require.Contains(t, err.Error(), "unknown configuration keys: rbac.bootstrap_super_admin.password")
}

// TestValidateRejectsShortProductionJWTSecret 验证生产类环境拒绝过短 JWT secret。
func TestValidateRejectsShortProductionJWTSecret(t *testing.T) {
	yaml := strings.ReplaceAll(serviceConfigYAML(), "environment: local", "environment: production")
	yaml = strings.ReplaceAll(yaml, "secret-123456789012345678901234567890", "short-secret")
	err := loadServiceConfigError(t, yaml)
	require.Contains(t, err.Error(), "auth.jwt.secret must be at least 32 bytes in production-like environments")
}

// TestValidateRejectsMissingRequiredResources 验证 user-service 必需具名资源缺失会失败。
func TestValidateRejectsMissingRequiredResources(t *testing.T) {
	yaml := strings.Replace(serviceConfigYAML(), "  cache_redis:\n", "  other_redis:\n", 1)
	yaml = strings.Replace(yaml, "  primary_db:\n", "  other_db:\n", 1)
	err := loadServiceConfigError(t, yaml)
	require.Contains(t, err.Error(), "resources.redis.cache_redis is required")
	require.Contains(t, err.Error(), "resources.postgres.primary_db is required")
}

// TestValidateReportsFullResourceFieldPath 验证资源校验错误包含完整字段路径。
func TestValidateReportsFullResourceFieldPath(t *testing.T) {
	yaml := strings.Replace(serviceConfigYAML(), "        - 127.0.0.1:6379", "        - invalid", 1)
	err := loadServiceConfigError(t, yaml)
	require.Contains(t, err.Error(), "resources.redis.cache_redis.addrs[0] must be in host:port format")
}

// TestLoadRejectsLegacyTopLevelResourcePath 验证旧顶层资源路径被严格解码拒绝。
func TestLoadRejectsLegacyTopLevelResourcePath(t *testing.T) {
	yaml := "redis:\n  cache_redis:\n    addr: 127.0.0.1:6379\n" + serviceConfigYAML()
	err := loadServiceConfigError(t, yaml)
	require.Contains(t, err.Error(), "unknown configuration keys: redis.cache_redis.addr")
}

// loadServiceConfig 加载一段 user-service 测试配置并要求成功。
func loadServiceConfig(t *testing.T, content string) *Config {
	t.Helper()
	result, err := loadFromDocumentsForTest([]commonconfig.ConfigDocument{{DataID: "test.yaml", Content: []byte(content)}})
	require.NoError(t, err)
	return result.Config
}

// loadServiceConfigError 加载一段预期失败的 user-service 测试配置并返回错误。
func loadServiceConfigError(t *testing.T, content string) error {
	t.Helper()
	_, err := loadFromDocumentsForTest([]commonconfig.ConfigDocument{{DataID: "test.yaml", Content: []byte(content)}})
	require.Error(t, err)
	return err
}

// loadFromDocumentsForTest 使用测试文档解码配置，避免生产包暴露 fixture 来源 API。
func loadFromDocumentsForTest(docs []commonconfig.ConfigDocument) (*LoadResult, error) {
	settings, err := commonconfig.DeepMergeYAML(append([]commonconfig.ConfigDocument(nil), docs...))
	if err != nil {
		return nil, err
	}
	dataIDs := make([]string, 0, len(docs))
	for _, doc := range docs {
		dataIDs = append(dataIDs, doc.DataID)
	}
	return DecodeSettings(settings, commonconfig.SourceMetadata{Provider: "testkit", DataIDs: dataIDs})
}

// serviceConfigYAML 返回覆盖 user-service 私有配置和共享 runtime 配置的基线 YAML。
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
http:
  request_body_max_bytes: 32768
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
api_rate_limit:
  anonymous:
    enabled: true
    rate_per_second: 2.5
    burst: 7
    max_keys: 8192
    capacity_policy: overflow
    key_ttl: 11m
    cleanup_interval: 15s
    shards: 32
  authenticated:
    enabled: true
    rate_per_second: 8
    burst: 30
    max_keys: 16384
    capacity_policy: reject
    key_ttl: 12m
    cleanup_interval: 20s
    shards: 64
rbac:
  user_role_cache:
    enabled: true
    size: 4096
    ttl: 7s
    load_timeout: 600ms
  outbox_dispatcher:
    poll_interval: 2s
    batch_size: 25
    claim_timeout: 45s
    retry_backoff:
      initial: 3s
      max: 2m
  policy_watcher:
    check_interval: 10s
    subscribe_timeout: 4s
    max_staleness: 35s
    retry_backoff:
      initial: 500ms
      max: 20s
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
      mode: cluster
      addrs:
        - 127.0.0.1:6379
      timeout: 7s
      cluster:
        max_redirects: 8
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

// serviceConfigYAMLWithResourceDefaults 返回省略资源默认字段后的基线 YAML。
func serviceConfigYAMLWithResourceDefaults() string {
	yaml := serviceConfigYAML()
	yaml = strings.Replace(yaml, "      timeout: 7s\n", "", 1)
	yaml = strings.Replace(yaml, "      sslmode: disable\n      pool:\n        max_open_conns: 20\n        max_idle_conns: 4\n        conn_max_lifetime: 45m\n        conn_max_idle_time: 12m\n", "", 1)
	return yaml
}
