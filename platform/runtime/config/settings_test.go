package config

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestEncodeSettingsUsesMapstructureTagsAndReadableDurations(t *testing.T) {
	type cacheConfig struct {
		Enabled bool          `mapstructure:"enabled"`
		TTL     time.Duration `mapstructure:"ttl"`
	}
	type serviceConfig struct {
		Config `mapstructure:",squash"`
		Caches map[string]cacheConfig `mapstructure:"caches"`
	}
	cfg := serviceConfig{
		Config: DefaultConfig(),
		Caches: map[string]cacheConfig{
			"sessions": {Enabled: true, TTL: 1500 * time.Millisecond},
		},
	}

	settings, err := EncodeSettings(&cfg)
	require.NoError(t, err)
	require.Equal(t, DefaultAppName, settings["app"].(map[string]any)["name"])
	require.Equal(t, "1m0s", settings["runtime"].(map[string]any)["lifecycle"].(map[string]any)["start_timeout"])
	require.Equal(t, "1.5s", settings["caches"].(map[string]any)["sessions"].(map[string]any)["ttl"])
}
