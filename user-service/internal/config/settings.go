package config

import (
	"time"

	commonresources "github.com/aegiscore/common/runtime/resources"
)

// AuthSettings 是认证 feature 构造运行时组件所需的最小配置视图。
type AuthSettings struct {
	AppName                  string
	JWT                      JWTConfig
	TokenVersionCache        FeatureCacheConfig
	TokenVersionCacheTTL     time.Duration
	RefreshTokenRotation     bool
	MaxActiveSessionsPerUser int
}

// RBACSettings 是 permission/RBAC 构造运行时组件所需的最小配置视图。
type RBACSettings struct {
	AppName          string
	UserRoleCache    FeatureCacheConfig
	OutboxDispatcher OutboxDispatcherConfig
}

// EntSettings 是 Ent client 构造插件所需的最小配置视图。
type EntSettings struct {
	Plugins EntPluginsConfig
}

// RateLimitSettings 是 user-service 构造 API 限流资源所需的最小配置视图。
type RateLimitSettings struct {
	APIRateLimit APIRateLimitConfig
}

// HTTPSettings 是 user-service HTTP transport 所需的私有配置视图。
type HTTPSettings struct {
	RequestBodyMaxBytes int64
}

// ResourceSettings 是 user-service 具名资源 provider 所需的配置视图。
type ResourceSettings struct {
	Redis    commonresources.RedisConfigs
	Postgres commonresources.PostgresConfigs
}

// NewAuthSettings 从根配置派生认证 feature settings。
func NewAuthSettings(cfg *Config) AuthSettings {
	return AuthSettings{
		AppName:                  cfg.App.Name,
		JWT:                      cfg.Auth.JWT,
		TokenVersionCache:        cfg.Auth.TokenVersionCache,
		TokenVersionCacheTTL:     cfg.Auth.TokenVersionCacheTTL,
		RefreshTokenRotation:     cfg.Auth.RefreshTokenRotation,
		MaxActiveSessionsPerUser: cfg.Auth.MaxActiveSessionsPerUser,
	}
}

// NewRBACSettings 从根配置派生 permission/RBAC settings。
func NewRBACSettings(cfg *Config) RBACSettings {
	return RBACSettings{
		AppName:          cfg.App.Name,
		UserRoleCache:    cfg.RBAC.UserRoleCache,
		OutboxDispatcher: cfg.RBAC.OutboxDispatcher,
	}
}

// NewEntSettings 从根配置派生 Ent settings。
func NewEntSettings(cfg *Config) EntSettings {
	return EntSettings{Plugins: cfg.Ent.Plugins}
}

// NewRateLimitSettings 从根配置派生 API 限流 settings。
func NewRateLimitSettings(cfg *Config) RateLimitSettings {
	return RateLimitSettings{APIRateLimit: cfg.APIRateLimit}
}

// NewHTTPSettings 从根配置派生 user-service HTTP transport settings。
func NewHTTPSettings(cfg *Config) HTTPSettings {
	return HTTPSettings{RequestBodyMaxBytes: cfg.HTTP.RequestBodyMaxBytes}
}

// NewResourceSettings 从根配置派生具名资源 settings。
func NewResourceSettings(cfg *Config) ResourceSettings {
	return ResourceSettings{Redis: cfg.Resources.Redis, Postgres: cfg.Resources.Postgres}
}
