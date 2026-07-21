package config

import (
	"strings"
	"time"

	commonconfig "github.com/aegiscore/common/runtime/config"
	commonresources "github.com/aegiscore/common/runtime/resources"
	serviceresources "github.com/aegiscore/user-service/internal/resources"
)

const minProductionJWTBytes = 32

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
	Enabled     *bool          `mapstructure:"enabled"`
	Size        *int64         `mapstructure:"size"`
	TTL         *time.Duration `mapstructure:"ttl"`
	LoadTimeout *time.Duration `mapstructure:"load_timeout"`
}

// IsEnabled 返回缓存是否启用；未显式配置时按默认启用处理。
func (c FeatureCacheConfig) IsEnabled() bool {
	return c.Enabled == nil || *c.Enabled
}

// SizeValue 返回配置的最大条目数；字段缺失时返回零。
func (c FeatureCacheConfig) SizeValue() int64 {
	if c.Size == nil {
		return 0
	}
	return *c.Size
}

// CapacityValue 返回可安全传递给 localcache 的容量；非正数返回零并由构造器拒绝。
func (c FeatureCacheConfig) CapacityValue() uint64 {
	size := c.SizeValue()
	if size <= 0 {
		return 0
	}
	return uint64(size)
}

// TTLValue 返回缓存条目的生命周期；字段缺失时返回零。
func (c FeatureCacheConfig) TTLValue() time.Duration {
	if c.TTL == nil {
		return 0
	}
	return *c.TTL
}

// LoadTimeoutValue 返回单次回源上限；字段缺失时返回零。
func (c FeatureCacheConfig) LoadTimeoutValue() time.Duration {
	if c.LoadTimeout == nil {
		return 0
	}
	return *c.LoadTimeout
}

// EntConfig 控制 user-service Ent 运行时行为。
type EntConfig struct {
	SQLDebug bool `mapstructure:"sql_debug"`
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

// ApplyDefaults 在服务配置校验前补齐所有具名资源默认值。
func (c *Config) ApplyDefaults() {
	if c == nil {
		return
	}
	c.Resources.Redis.ApplyDefaults()
	c.Resources.Postgres.ApplyDefaults()
	c.Auth.TokenVersionCache.applyDefaults(100000, time.Second, 300*time.Millisecond)
	c.RBAC.UserRoleCache.applyDefaults(100000, 5*time.Second, 500*time.Millisecond)
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
	errs = append(errs, validateFeatureCache("rbac.user_role_cache", c.RBAC.UserRoleCache)...)
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
	errs = append(errs, validateFeatureCache("auth.token_version_cache", c.Auth.TokenVersionCache)...)
	return errs
}

func (c *FeatureCacheConfig) applyDefaults(size int64, ttl time.Duration, loadTimeout time.Duration) {
	if c.Enabled == nil {
		enabled := true
		c.Enabled = &enabled
	}
	if !c.IsEnabled() {
		return
	}
	if c.Size == nil {
		c.Size = &size
	}
	if c.TTL == nil {
		c.TTL = &ttl
	}
	if c.LoadTimeout == nil {
		c.LoadTimeout = &loadTimeout
	}
}

func validateFeatureCache(path string, cfg FeatureCacheConfig) []error {
	if !cfg.IsEnabled() {
		return nil
	}
	var errs []error
	if cfg.SizeValue() <= 0 {
		errs = append(errs, commonconfig.FieldError(path+".size", "must be > 0 when enabled"))
	}
	errs = append(errs, commonconfig.ValidatePositiveDuration(path+".ttl", cfg.TTLValue())...)
	errs = append(errs, commonconfig.ValidatePositiveDuration(path+".load_timeout", cfg.LoadTimeoutValue())...)
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
