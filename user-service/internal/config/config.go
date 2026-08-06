package config

import (
	"strings"
	"time"

	commonconfig "github.com/aegiscore/common/runtime/config"
	"github.com/aegiscore/common/runtime/localcache"
	commonresources "github.com/aegiscore/common/runtime/resources"
	serviceresources "github.com/aegiscore/user-service/internal/resources"
)

const (
	minProductionJWTBytes = 32
	// DefaultHTTPRequestBodyMaxBytes 是 user-service JSON 请求体的默认字节上限。
	DefaultHTTPRequestBodyMaxBytes int64 = 64 * 1024
	// DefaultEntSlowQueryThreshold 是 Ent SQL log 插件的默认慢查询阈值。
	DefaultEntSlowQueryThreshold = 500 * time.Millisecond
)

var sensitiveConfigPaths = []string{
	"auth.jwt.secret",
	"resources.redis.*.password",
	"resources.postgres.*.password",
}

// Config 是 user-service 的根配置对象。
type Config struct {
	commonconfig.Config `mapstructure:",squash"`
	Resources           ResourcesConfig    `mapstructure:"resources"`
	Auth                AuthConfig         `mapstructure:"auth"`
	RBAC                RBACConfig         `mapstructure:"rbac"`
	APIRateLimit        APIRateLimitConfig `mapstructure:"api_rate_limit"`
	HTTP                HTTPConfig         `mapstructure:"http"`
	Ent                 EntConfig          `mapstructure:"ent"`
}

// HTTPConfig 包含 user-service 私有 HTTP 暴露策略。
type HTTPConfig struct {
	RequestBodyMaxBytes int64 `mapstructure:"request_body_max_bytes"`
}

// ResourcesConfig 包含 user-service 按名称声明的外部运行时资源。
type ResourcesConfig struct {
	Redis    commonresources.RedisConfigs    `mapstructure:"redis"`
	Postgres commonresources.PostgresConfigs `mapstructure:"postgres"`
}

// AuthConfig 包含 user-service 认证 token 与会话校验设置。
// TokenVersionCache 由各服务副本独立持有，其 TTL 表示正常路径可接受的跨副本撤销收敛窗口。
// 是否使用事务 outbox 取决于 Redis token version 投影失败后是否需要可靠补偿，与固定 TTL 阈值无关。
type AuthConfig struct {
	JWT                      JWTConfig          `mapstructure:"jwt"`
	TokenVersionCache        FeatureCacheConfig `mapstructure:"token_version_cache"`
	TokenVersionCacheTTL     time.Duration      `mapstructure:"token_version_cache_ttl"`
	RefreshTokenRotation     bool               `mapstructure:"refresh_token_rotation"`
	MaxActiveSessionsPerUser int                `mapstructure:"max_active_sessions_per_user"`
}

// RBACConfig 包含 user-service RBAC 热路径的服务私有设置。
type RBACConfig struct {
	UserRoleCache    FeatureCacheConfig     `mapstructure:"user_role_cache"`
	OutboxDispatcher OutboxDispatcherConfig `mapstructure:"outbox_dispatcher"`
	PolicyWatcher    PolicyWatcherConfig    `mapstructure:"policy_watcher"`
}

// PolicyWatcherConfig 控制 RBAC policy watcher 的校准、订阅确认和重连节奏。
type PolicyWatcherConfig struct {
	CheckInterval    time.Duration      `mapstructure:"check_interval"`
	SubscribeTimeout time.Duration      `mapstructure:"subscribe_timeout"`
	MaxStaleness     time.Duration      `mapstructure:"max_staleness"`
	RetryBackoff     RetryBackoffConfig `mapstructure:"retry_backoff"`
}

// OutboxDispatcherConfig 控制 RBAC policy outbox 的后台 claim 与投递节奏。
type OutboxDispatcherConfig struct {
	PollInterval time.Duration      `mapstructure:"poll_interval"`
	BatchSize    int                `mapstructure:"batch_size"`
	ClaimTimeout time.Duration      `mapstructure:"claim_timeout"`
	RetryBackoff RetryBackoffConfig `mapstructure:"retry_backoff"`
}

// RetryBackoffConfig 描述 outbox 投递失败后的有界指数退避范围。
type RetryBackoffConfig struct {
	Initial time.Duration `mapstructure:"initial"`
	Max     time.Duration `mapstructure:"max"`
}

// DefaultOutboxDispatcherConfig 返回安全且完整的 RBAC outbox dispatcher 默认配置。
func DefaultOutboxDispatcherConfig() OutboxDispatcherConfig {
	return OutboxDispatcherConfig{
		PollInterval: time.Second,
		BatchSize:    100,
		ClaimTimeout: 30 * time.Second,
		RetryBackoff: RetryBackoffConfig{
			Initial: time.Second,
			Max:     time.Minute,
		},
	}
}

// DefaultPolicyWatcherConfig 返回 RBAC policy watcher 的完整默认配置。
func DefaultPolicyWatcherConfig() PolicyWatcherConfig {
	return PolicyWatcherConfig{
		CheckInterval:    15 * time.Second,
		SubscribeTimeout: 5 * time.Second,
		MaxStaleness:     45 * time.Second,
		RetryBackoff: RetryBackoffConfig{
			Initial: 250 * time.Millisecond,
			Max:     30 * time.Second,
		},
	}
}

// Validate 校验 policy watcher 的校准、新鲜度和重连边界。
func (c PolicyWatcherConfig) Validate(path string) []error {
	var errs []error
	errs = append(errs, commonconfig.ValidatePositiveDuration(path+".check_interval", c.CheckInterval)...)
	errs = append(errs, commonconfig.ValidatePositiveDuration(path+".subscribe_timeout", c.SubscribeTimeout)...)
	errs = append(errs, commonconfig.ValidatePositiveDuration(path+".max_staleness", c.MaxStaleness)...)
	errs = append(errs, commonconfig.ValidatePositiveDuration(path+".retry_backoff.initial", c.RetryBackoff.Initial)...)
	errs = append(errs, commonconfig.ValidatePositiveDuration(path+".retry_backoff.max", c.RetryBackoff.Max)...)
	if c.RetryBackoff.Max > 0 && c.RetryBackoff.Initial > 0 && c.RetryBackoff.Max < c.RetryBackoff.Initial {
		errs = append(errs, commonconfig.FieldError(path+".retry_backoff.max", "must be >= retry_backoff.initial"))
	}
	if c.MaxStaleness > 0 && c.CheckInterval > 0 && c.MaxStaleness <= c.CheckInterval {
		errs = append(errs, commonconfig.FieldError(path+".max_staleness", "must be > check_interval"))
	}
	return errs
}

// Validate 校验 outbox dispatcher 的轮询、claim 和重试边界。
func (c OutboxDispatcherConfig) Validate(path string) []error {
	var errs []error
	errs = append(errs, commonconfig.ValidatePositiveDuration(path+".poll_interval", c.PollInterval)...)
	if c.BatchSize <= 0 {
		errs = append(errs, commonconfig.FieldError(path+".batch_size", "must be > 0"))
	}
	errs = append(errs, commonconfig.ValidatePositiveDuration(path+".claim_timeout", c.ClaimTimeout)...)
	errs = append(errs, commonconfig.ValidatePositiveDuration(path+".retry_backoff.initial", c.RetryBackoff.Initial)...)
	errs = append(errs, commonconfig.ValidatePositiveDuration(path+".retry_backoff.max", c.RetryBackoff.Max)...)
	if c.RetryBackoff.Max > 0 && c.RetryBackoff.Initial > 0 && c.RetryBackoff.Max < c.RetryBackoff.Initial {
		errs = append(errs, commonconfig.FieldError(path+".retry_backoff.max", "must be >= retry_backoff.initial"))
	}
	return errs
}

// APIRateLimitConfig 包含 user-service API 限流的服务私有配置。
type APIRateLimitConfig struct {
	Anonymous     RateLimitPolicyConfig `mapstructure:"anonymous"`
	Authenticated RateLimitPolicyConfig `mapstructure:"authenticated"`
}

// RateLimitPolicyConfig 描述一个限流策略的本地 token bucket 与清理设置。
type RateLimitPolicyConfig struct {
	Enabled         bool          `mapstructure:"enabled"`
	RatePerSecond   float64       `mapstructure:"rate_per_second"`
	Burst           int           `mapstructure:"burst"`
	KeyTTL          time.Duration `mapstructure:"key_ttl"`
	CleanupInterval time.Duration `mapstructure:"cleanup_interval"`
	Shards          int           `mapstructure:"shards"`
}

// DefaultRateLimitPolicyConfig 构造启用状态下的完整限流策略默认配置。
func DefaultRateLimitPolicyConfig(ratePerSecond float64, burst int, keyTTL time.Duration, cleanupInterval time.Duration, shards int) RateLimitPolicyConfig {
	return RateLimitPolicyConfig{Enabled: true, RatePerSecond: ratePerSecond, Burst: burst, KeyTTL: keyTTL, CleanupInterval: cleanupInterval, Shards: shards}
}

// Validate 校验启用的限流策略；禁用时其余字段不参与运行时构造和校验。
func (c RateLimitPolicyConfig) Validate(path string) []error {
	if !c.Enabled {
		return nil
	}
	var errs []error
	if c.RatePerSecond <= 0 {
		errs = append(errs, commonconfig.FieldError(path+".rate_per_second", "must be > 0 when enabled"))
	}
	if c.Burst <= 0 {
		errs = append(errs, commonconfig.FieldError(path+".burst", "must be > 0 when enabled"))
	}
	errs = append(errs, commonconfig.ValidatePositiveDuration(path+".key_ttl", c.KeyTTL)...)
	errs = append(errs, commonconfig.ValidatePositiveDuration(path+".cleanup_interval", c.CleanupInterval)...)
	if c.Shards <= 0 {
		errs = append(errs, commonconfig.FieldError(path+".shards", "must be > 0 when enabled"))
	}
	return errs
}

// FeatureCacheConfig 描述单个 feature 自有 bounded cache 的稳定配置面。
type FeatureCacheConfig struct {
	Enabled     bool          `mapstructure:"enabled"`
	Size        int64         `mapstructure:"size"`
	TTL         time.Duration `mapstructure:"ttl"`
	LoadTimeout time.Duration `mapstructure:"load_timeout"`
}

// DefaultFeatureCacheConfig 构造启用状态下的完整 feature cache 默认配置。
func DefaultFeatureCacheConfig(size int64, ttl time.Duration, loadTimeout time.Duration) FeatureCacheConfig {
	return FeatureCacheConfig{
		Enabled:     true,
		Size:        size,
		TTL:         ttl,
		LoadTimeout: loadTimeout,
	}
}

// Validate 校验启用的 feature cache；禁用时其余字段不参与运行时构造和校验。
func (c FeatureCacheConfig) Validate(path string) []error {
	if !c.Enabled {
		return nil
	}
	var errs []error
	if c.Size <= 0 {
		errs = append(errs, commonconfig.FieldError(path+".size", "must be > 0 when enabled"))
	}
	errs = append(errs, commonconfig.ValidatePositiveDuration(path+".ttl", c.TTL)...)
	errs = append(errs, commonconfig.ValidatePositiveDuration(path+".load_timeout", c.LoadTimeout)...)
	return errs
}

// Localcache 将服务私有 feature cache 配置集中映射为通用 localcache 配置。
func (c FeatureCacheConfig) Localcache(name string) localcache.Config {
	var capacity uint64
	if c.Size > 0 {
		capacity = uint64(c.Size)
	}
	return localcache.Config{
		Name:        name,
		Capacity:    capacity,
		TTL:         c.TTL,
		LoadTimeout: c.LoadTimeout,
	}
}

// EntConfig 控制 user-service Ent 运行时行为。
type EntConfig struct {
	Plugins EntPluginsConfig `mapstructure:"plugins"`
}

// EntPluginsConfig 控制 Ent 运行时观测插件启停。
type EntPluginsConfig struct {
	SQLLog  EntSQLLogPluginConfig  `mapstructure:"sql_log"`
	Tracing EntTracingPluginConfig `mapstructure:"tracing"`
	Metrics EntMetricsPluginConfig `mapstructure:"metrics"`
}

// EntSQLLogPluginConfig 控制 Ent SQL driver 日志插件行为。
type EntSQLLogPluginConfig struct {
	Enabled       bool          `mapstructure:"enabled"`
	Debug         bool          `mapstructure:"debug"`
	SlowThreshold time.Duration `mapstructure:"slow_threshold"`
}

// EntTracingPluginConfig 控制 Ent tracing 插件行为。
type EntTracingPluginConfig struct {
	Enabled bool `mapstructure:"enabled"`
}

// EntMetricsPluginConfig 控制 Ent Prometheus metrics 插件行为。
type EntMetricsPluginConfig struct {
	Enabled bool `mapstructure:"enabled"`
}

// JWTConfig 包含 user-service JWT 签发和校验设置。
type JWTConfig struct {
	Secret                 string        `mapstructure:"secret"`
	Issuer                 string        `mapstructure:"issuer"`
	Audience               string        `mapstructure:"audience"`
	AccessTokenTTL         time.Duration `mapstructure:"access_token_ttl"`
	RefreshTokenTTL        time.Duration `mapstructure:"refresh_token_ttl"`
	PasswordChangeTokenTTL time.Duration `mapstructure:"password_change_token_ttl"`
}

// RuntimeConfig 返回共享 runtime 配置副本。
func (c Config) RuntimeConfig() commonconfig.Config {
	return c.Config
}

// SensitiveConfigPaths 返回 user-service 拥有的配置脱敏路径副本。
func SensitiveConfigPaths() []string {
	return append([]string(nil), sensitiveConfigPaths...)
}

// RedactEffectiveSettings 使用 user-service 敏感路径策略返回脱敏副本。
func RedactEffectiveSettings(settings map[string]any) map[string]any {
	return commonconfig.RedactSettings(settings, sensitiveConfigPaths)
}

// DefaultConfig 返回 user-service 的完整默认配置初值。
func DefaultConfig() Config {
	return Config{
		Config: commonconfig.DefaultConfig(),
		Resources: ResourcesConfig{
			Redis: commonresources.RedisConfigs{
				serviceresources.NameCacheRedis: {
					Mode:    commonresources.RedisModeCluster,
					Timeout: commonresources.DefaultRedisTimeout,
					Cluster: commonresources.RedisClusterConfig{
						MaxRedirects: commonresources.DefaultRedisClusterMaxRedirects,
					},
				},
			},
			Postgres: commonresources.PostgresConfigs{
				serviceresources.NamePrimaryDB: {
					SSLMode: commonresources.DefaultPostgresSSLMode,
					Pool: commonresources.PostgresPoolConfig{
						MaxOpenConns:    commonresources.DefaultPostgresMaxOpenConns,
						MaxIdleConns:    commonresources.DefaultPostgresMaxIdleConns,
						ConnMaxLifetime: commonresources.DefaultPostgresConnMaxLifetime,
						ConnMaxIdleTime: commonresources.DefaultPostgresConnMaxIdleTime,
					},
				},
			},
		},
		Auth: AuthConfig{
			// 默认接受 logout-all 和改密在正常 Redis 更新成功后最多 1 秒的跨副本撤销收敛窗口。
			TokenVersionCache: DefaultFeatureCacheConfig(100000, time.Second, 300*time.Millisecond),
		},
		RBAC: RBACConfig{
			UserRoleCache:    DefaultFeatureCacheConfig(100000, 5*time.Second, 500*time.Millisecond),
			OutboxDispatcher: DefaultOutboxDispatcherConfig(),
			PolicyWatcher:    DefaultPolicyWatcherConfig(),
		},
		APIRateLimit: APIRateLimitConfig{
			Anonymous:     DefaultRateLimitPolicyConfig(1, 5, 10*time.Minute, 30*time.Second, 64),
			Authenticated: DefaultRateLimitPolicyConfig(5, 20, 10*time.Minute, 30*time.Second, 128),
		},
		HTTP: HTTPConfig{RequestBodyMaxBytes: DefaultHTTPRequestBodyMaxBytes},
		Ent: EntConfig{
			Plugins: EntPluginsConfig{
				SQLLog: EntSQLLogPluginConfig{
					SlowThreshold: DefaultEntSlowQueryThreshold,
				},
				Tracing: EntTracingPluginConfig{Enabled: true},
			},
		},
	}
}

// normalizeConfig 按 raw settings 保留实际声明的固定资源，并补齐新增动态具名资源的通用默认值。
func normalizeConfig(c *Config, settings map[string]any) {
	if c == nil {
		return
	}
	if !hasRawNamedResource(settings, "redis", serviceresources.NameCacheRedis) {
		delete(c.Resources.Redis, serviceresources.NameCacheRedis)
	}
	if !hasRawNamedResource(settings, "postgres", serviceresources.NamePrimaryDB) {
		delete(c.Resources.Postgres, serviceresources.NamePrimaryDB)
	}
	c.Resources.Redis.ApplyDefaults()
	c.Resources.Postgres.ApplyDefaults()
}

func hasRawNamedResource(settings map[string]any, resourceType string, name string) bool {
	resources, ok := settings["resources"].(map[string]any)
	if !ok {
		return false
	}
	namedResources, ok := resources[resourceType].(map[string]any)
	if !ok {
		return false
	}
	value, exists := namedResources[name]
	return exists && value != nil
}

// Validate 在 user-service 启动前拒绝结构非法的服务配置。
// 先复用 common runtime 校验，再追加 user-service 认证约束；返回聚合错误，便于一次性展示所有字段问题。
func (c Config) Validate() error {
	var errs []error
	if err := c.Config.Validate(); err != nil {
		errs = append(errs, err)
	}
	if err := c.Resources.Redis.Validate("resources.redis"); err != nil {
		errs = append(errs, err)
	}
	if err := c.Resources.Postgres.Validate("resources.postgres"); err != nil {
		errs = append(errs, err)
	}
	if _, ok := c.Resources.Redis[serviceresources.NameCacheRedis]; !ok {
		errs = append(errs, commonconfig.FieldError("resources.redis."+serviceresources.NameCacheRedis, "is required"))
	}
	if _, ok := c.Resources.Postgres[serviceresources.NamePrimaryDB]; !ok {
		errs = append(errs, commonconfig.FieldError("resources.postgres."+serviceresources.NamePrimaryDB, "is required"))
	}
	errs = append(errs, c.validateAuth()...)
	errs = append(errs, c.RBAC.UserRoleCache.Validate("rbac.user_role_cache")...)
	errs = append(errs, c.RBAC.OutboxDispatcher.Validate("rbac.outbox_dispatcher")...)
	errs = append(errs, c.RBAC.PolicyWatcher.Validate("rbac.policy_watcher")...)
	errs = append(errs, c.APIRateLimit.Anonymous.Validate("api_rate_limit.anonymous")...)
	errs = append(errs, c.APIRateLimit.Authenticated.Validate("api_rate_limit.authenticated")...)
	if c.HTTP.RequestBodyMaxBytes <= 0 {
		errs = append(errs, commonconfig.FieldError("http.request_body_max_bytes", "must be > 0"))
	}
	if len(errs) == 0 {
		return nil
	}
	return commonconfig.NewValidationError(errs)
}

func (c Config) validateAuth() []error {
	// production-like 环境对 JWT secret 额外加固；0 个活跃会话上限表示不裁剪。
	var errs []error
	secret := strings.TrimSpace(c.Auth.JWT.Secret)
	if secret == "" {
		errs = append(errs, commonconfig.FieldError("auth.jwt.secret", "is required"))
	} else if c.isProductionLike() {
		if isInsecureJWTSecret(strings.ToLower(secret)) {
			errs = append(errs, commonconfig.FieldError("auth.jwt.secret", "must not use a development default in production-like environments"))
		} else if len([]byte(secret)) < minProductionJWTBytes {
			errs = append(errs, commonconfig.FieldError("auth.jwt.secret", "must be at least 32 bytes in production-like environments"))
		}
	}
	errs = append(errs, commonconfig.ValidatePositiveDuration("auth.jwt.access_token_ttl", c.Auth.JWT.AccessTokenTTL)...)
	errs = append(errs, commonconfig.ValidatePositiveDuration("auth.jwt.refresh_token_ttl", c.Auth.JWT.RefreshTokenTTL)...)
	errs = append(errs, commonconfig.ValidateNonNegativeInt("auth.max_active_sessions_per_user", c.Auth.MaxActiveSessionsPerUser)...)
	errs = append(errs, c.Auth.TokenVersionCache.Validate("auth.token_version_cache")...)
	return errs
}

func (c Config) isProductionLike() bool {
	// staging 按生产级安全策略校验，避免预发环境使用开发默认 secret。
	switch strings.ToLower(strings.TrimSpace(c.App.Environment)) {
	case "prod", "production", "staging":
		return true
	default:
		return false
	}
}

func isInsecureJWTSecret(value string) bool {
	switch value {
	case "changeme", "local-development-secret", "secret", "test-secret":
		return true
	default:
		return false
	}
}
