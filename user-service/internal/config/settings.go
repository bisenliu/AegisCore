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
	AppName       string
	UserRoleCache FeatureCacheConfig
}

// EntSettings 是 Ent client 构造插件所需的最小配置视图。
type EntSettings struct {
	Plugins EntPluginsConfig
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
	return RBACSettings{AppName: cfg.App.Name, UserRoleCache: cfg.RBAC.UserRoleCache}
}

// NewEntSettings 从根配置派生 Ent settings。
func NewEntSettings(cfg *Config) EntSettings {
	return EntSettings{Plugins: cfg.Ent.Plugins}
}

// NewResourceSettings 从根配置派生具名资源 settings。
func NewResourceSettings(cfg *Config) ResourceSettings {
	return ResourceSettings{Redis: cfg.Resources.Redis, Postgres: cfg.Resources.Postgres}
}
