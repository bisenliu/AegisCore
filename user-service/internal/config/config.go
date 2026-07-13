package config

import (
	"strings"
	"time"

	commonconfig "github.com/aegiscore/common/runtime/config"
)

const minProductionJWTBytes = 32

// Config 是 user-service 的根配置对象。
type Config struct {
	commonconfig.Config `mapstructure:",squash"`
	Auth                AuthConfig `mapstructure:"auth"`
	Ent                 EntConfig  `mapstructure:"ent"`
}

// AuthConfig 包含 user-service 认证 token 与会话校验设置。
type AuthConfig struct {
	JWT                      JWTConfig         `mapstructure:"jwt"`
	PasswordKDF              PasswordKDFConfig `mapstructure:"password_kdf"`
	TokenVersionCacheTTL     time.Duration     `mapstructure:"token_version_cache_ttl"`
	RefreshTokenRotation     bool              `mapstructure:"refresh_token_rotation"`
	MaxActiveSessionsPerUser int               `mapstructure:"max_active_sessions_per_user"`
}

// PasswordKDFConfig 包含密码 Argon2id KDF 的实例级资源预算。
type PasswordKDFConfig struct {
	Argon2Concurrency int `mapstructure:"argon2_concurrency"`
	Argon2QueueSize   int `mapstructure:"argon2_queue_size"`
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

// Validate 在 user-service 启动前拒绝结构非法的服务配置。
// 先复用 common runtime 校验，再追加 user-service 认证约束；返回聚合错误，便于一次性展示所有字段问题。
func (c Config) Validate() error {
	var errs []error
	if err := c.Config.Validate(); err != nil {
		errs = append(errs, err)
	}
	errs = append(errs, c.validateAuth()...)
	if len(errs) == 0 {
		return nil
	}
	return commonconfig.NewValidationError(errs)
}

func (c Config) validateAuth() []error {
	// production-like 环境对 JWT secret 额外加固；Argon2 queue 必须覆盖 concurrency，0 个活跃会话上限表示不裁剪。
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
	errs = append(errs, commonconfig.ValidatePositiveInt("auth.password_kdf.argon2_concurrency", c.Auth.PasswordKDF.Argon2Concurrency)...)
	errs = append(errs, commonconfig.ValidatePositiveInt("auth.password_kdf.argon2_queue_size", c.Auth.PasswordKDF.Argon2QueueSize)...)
	if c.Auth.PasswordKDF.Argon2Concurrency > 0 && c.Auth.PasswordKDF.Argon2QueueSize > 0 && c.Auth.PasswordKDF.Argon2QueueSize < c.Auth.PasswordKDF.Argon2Concurrency {
		errs = append(errs, commonconfig.FieldError("auth.password_kdf.argon2_queue_size", "must be >= auth.password_kdf.argon2_concurrency"))
	}
	errs = append(errs, commonconfig.ValidateNonNegativeInt("auth.max_active_sessions_per_user", c.Auth.MaxActiveSessionsPerUser)...)
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
