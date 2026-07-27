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
	// DefaultEntSlowQueryThreshold 是 Ent SQL log 插件的默认慢查询阈值。
	DefaultEntSlowQueryThreshold = 500 * time.Millisecond
)

// Config 是 user-service 的根配置对象。
type Config struct {
	commonconfig.Config `mapstructure:",squash"`
	Resources           ResourcesConfig `mapstructure:"resources"`
	Auth                AuthConfig      `mapstructure:"auth"`
	RBAC                RBACConfig      `mapstructure:"rbac"`
	Ent                 EntConfig       `mapstructure:"ent"`
}

// ResourcesConfig 包含 user-service 按名称声明的外部运行时资源。
type ResourcesConfig struct {
	Redis    commonresources.RedisConfigs    `mapstructure:"redis"`
	Postgres commonresources.PostgresConfigs `mapstructure:"postgres"`
}

// AuthConfig 包含 user-service 认证 token 与会话校验设置。
type AuthConfig struct {
	JWT                      JWTConfig          `mapstructure:"jwt"`
	TokenVersionCache        FeatureCacheConfig `mapstructure:"token_version_cache"`
	TokenVersionCacheTTL     time.Duration      `mapstructure:"token_version_cache_ttl"`
	RefreshTokenRotation     bool               `mapstructure:"refresh_token_rotation"`
	MaxActiveSessionsPerUser int                `mapstructure:"max_active_sessions_per_user"`
}

// RBACConfig 包含 user-service RBAC 热路径的服务私有设置。
type RBACConfig struct {
	UserRoleCache FeatureCacheConfig `mapstructure:"user_role_cache"`
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
			TokenVersionCache: DefaultFeatureCacheConfig(100000, time.Second, 300*time.Millisecond),
		},
		RBAC: RBACConfig{
			UserRoleCache: DefaultFeatureCacheConfig(100000, 5*time.Second, 500*time.Millisecond),
		},
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
