package config

import (
	"reflect"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	commonconfig "github.com/aegiscore/common/runtime/config"
	commonresources "github.com/aegiscore/common/runtime/resources"
)

func TestNarrowSettingsDeriveOwnedConfiguration(t *testing.T) {
	cfg := &Config{
		Config: commonconfig.Config{App: commonconfig.AppConfig{Name: "user-service"}},
		Auth: AuthConfig{
			JWT:                      JWTConfig{Secret: "secret", Issuer: "issuer"},
			TokenVersionCache:        FeatureCacheConfig{Enabled: true, Size: 11},
			TokenVersionCacheTTL:     3 * time.Minute,
			RefreshTokenRotation:     true,
			MaxActiveSessionsPerUser: 7,
		},
		RBAC:         RBACConfig{UserRoleCache: FeatureCacheConfig{Enabled: true, Size: 13}},
		APIRateLimit: APIRateLimitConfig{Anonymous: RateLimitPolicyConfig{Enabled: true, RatePerSecond: 2, Burst: 3}},
		Ent:          EntConfig{Plugins: EntPluginsConfig{Metrics: EntMetricsPluginConfig{Enabled: true}}},
		Resources: ResourcesConfig{
			Redis:    commonresources.RedisConfigs{"cache_redis": {Mode: commonresources.RedisModeCluster, Addrs: []string{"redis:6379"}}},
			Postgres: commonresources.PostgresConfigs{"primary_db": {Host: "postgres"}},
		},
	}

	require.Equal(t, AuthSettings{
		AppName:                  "user-service",
		JWT:                      cfg.Auth.JWT,
		TokenVersionCache:        cfg.Auth.TokenVersionCache,
		TokenVersionCacheTTL:     3 * time.Minute,
		RefreshTokenRotation:     true,
		MaxActiveSessionsPerUser: 7,
	}, NewAuthSettings(cfg))
	require.Equal(t, RBACSettings{AppName: "user-service", UserRoleCache: cfg.RBAC.UserRoleCache}, NewRBACSettings(cfg))
	require.Equal(t, EntSettings{Plugins: cfg.Ent.Plugins}, NewEntSettings(cfg))
	require.Equal(t, RateLimitSettings{APIRateLimit: cfg.APIRateLimit}, NewRateLimitSettings(cfg))
	require.Equal(t, ResourceSettings{Redis: cfg.Resources.Redis, Postgres: cfg.Resources.Postgres}, NewResourceSettings(cfg))
}

func TestNarrowSettingsFieldOwnership(t *testing.T) {
	tests := []struct {
		name   string
		value  any
		fields []string
	}{
		{name: "auth", value: AuthSettings{}, fields: []string{"AppName", "JWT", "TokenVersionCache", "TokenVersionCacheTTL", "RefreshTokenRotation", "MaxActiveSessionsPerUser"}},
		{name: "rbac", value: RBACSettings{}, fields: []string{"AppName", "UserRoleCache"}},
		{name: "ent", value: EntSettings{}, fields: []string{"Plugins"}},
		{name: "rate limit", value: RateLimitSettings{}, fields: []string{"APIRateLimit"}},
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
