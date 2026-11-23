package config

import (
	"reflect"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	platformconfig "github.com/aegiscore/platform/runtime/config"
	platformresources "github.com/aegiscore/platform/runtime/resources"
)

func TestNarrowSettingsDeriveOwnedConfiguration(t *testing.T) {
	cfg := &Config{
		Config: platformconfig.Config{
			App:     platformconfig.AppConfig{Name: "user-service"},
			Runtime: platformconfig.RuntimeConfig{Timezone: "Asia/Shanghai"},
		},
		Auth: AuthConfig{
			JWT:                      JWTConfig{Secret: "secret", Issuer: "issuer"},
			TokenVersionCache:        FeatureCacheConfig{Enabled: true, TTL: time.Second},
			TokenVersionCacheTTL:     3 * time.Minute,
			RefreshTokenRotation:     true,
			MaxActiveSessionsPerUser: 7,
			LoginLockout:             LoginLockoutConfig{Enabled: true, MaxFailures: 5, FailureWindow: 15 * time.Minute, LockoutTTL: 15 * time.Minute},
		},
		RBAC: RBACConfig{
			UserRoleCache: FeatureCacheConfig{Enabled: true, TTL: 5 * time.Second},
			PolicyWatcher: PolicyWatcherConfig{CheckInterval: 3 * time.Second, MaxStaleness: 10 * time.Second},
		},
		APIRateLimit: APIRateLimitConfig{Anonymous: RateLimitPolicyConfig{Enabled: true, RatePerSecond: 2, Burst: 3}},
		HTTP:         HTTPConfig{RequestBodyMaxBytes: 12345},
		Ent:          EntConfig{Plugins: EntPluginsConfig{Metrics: EntMetricsPluginConfig{Enabled: true}}},
		Resources: ResourcesConfig{
			Redis:    platformresources.RedisConfigs{"cache_redis": {Mode: platformresources.RedisModeCluster, Addrs: []string{"redis:6379"}}},
			Postgres: platformresources.PostgresConfigs{"primary_db": {Host: "postgres"}},
		},
	}

	require.Equal(t, AuthSettings{
		AppName:                  "user-service",
		JWT:                      cfg.Auth.JWT,
		TokenVersionCache:        cfg.Auth.TokenVersionCache,
		TokenVersionCacheTTL:     3 * time.Minute,
		RefreshTokenRotation:     true,
		MaxActiveSessionsPerUser: 7,
		LoginLockout:             cfg.Auth.LoginLockout,
	}, NewAuthSettings(cfg))
	require.Equal(t, RBACSettings{
		AppName:       "user-service",
		UserRoleCache: cfg.RBAC.UserRoleCache,
		PolicyWatcher: cfg.RBAC.PolicyWatcher,
	}, NewRBACSettings(cfg))
	require.Equal(t, EntSettings{Plugins: cfg.Ent.Plugins}, NewEntSettings(cfg))
	require.Equal(t, RateLimitSettings{APIRateLimit: cfg.APIRateLimit}, NewRateLimitSettings(cfg))
	require.Equal(t, HTTPSettings{RequestBodyMaxBytes: 12345, CORS: cfg.HTTP.CORS}, NewHTTPSettings(cfg))
	require.Equal(t, TimezoneSettings{Name: "Asia/Shanghai"}, NewTimezoneSettings(cfg))
	require.Equal(t, ResourceSettings{Redis: cfg.Resources.Redis, Postgres: cfg.Resources.Postgres}, NewResourceSettings(cfg))
}

func TestNarrowSettingsFieldOwnership(t *testing.T) {
	tests := []struct {
		name   string
		value  any
		fields []string
	}{
		{name: "auth", value: AuthSettings{}, fields: []string{"AppName", "JWT", "TokenVersionCache", "TokenVersionCacheTTL", "RefreshTokenRotation", "MaxActiveSessionsPerUser", "LoginLockout"}},
		{name: "rbac", value: RBACSettings{}, fields: []string{"AppName", "UserRoleCache", "PolicyWatcher"}},
		{name: "ent", value: EntSettings{}, fields: []string{"Plugins"}},
		{name: "rate limit", value: RateLimitSettings{}, fields: []string{"APIRateLimit"}},
		{name: "http", value: HTTPSettings{}, fields: []string{"RequestBodyMaxBytes", "CORS"}},
		{name: "timezone", value: TimezoneSettings{}, fields: []string{"Name"}},
		{name: "resources", value: ResourceSettings{}, fields: []string{"Redis", "Postgres"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			typeOf := reflect.TypeOf(tt.value)
			fields := make([]string, 0, typeOf.NumField())
			for index := range typeOf.NumField() {
				fields = append(fields, typeOf.Field(index).Name)
			}
			require.Equal(t, tt.fields, fields)
		})
	}
}
