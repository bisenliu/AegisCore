package nacos

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestLoadEnvDefaultsDataIDs(t *testing.T) {
	env, err := loadEnv(mapLookup(map[string]string{
		EnvService:   " user-service ",
		EnvAddr:      "nacos:8848",
		EnvNamespace: "local",
		EnvGroup:     "AEGISCORE",
	}))
	require.NoError(t, err)
	require.Equal(t, []string{"base.yaml", "resources.yaml", "user-service.yaml"}, env.DataIDs)
	require.Equal(t, defaultTimeout, env.Timeout)
}

func TestLoadEnvRejectsMissingRequired(t *testing.T) {
	_, err := loadEnv(mapLookup(map[string]string{
		EnvService:   "user-service",
		EnvNamespace: "local",
		EnvGroup:     "AEGISCORE",
	}))
	require.ErrorContains(t, err, "read config env: AEGISCORE_NACOS_ADDR is required")
}

func TestLoadEnvParsesExplicitDataIDsAndTimeout(t *testing.T) {
	env, err := loadEnv(mapLookup(map[string]string{
		EnvService:   "user-service",
		EnvAddr:      "nacos:8848",
		EnvNamespace: "local",
		EnvGroup:     "AEGISCORE",
		EnvDataIDs:   "base.yaml, resources.yaml , override.yaml",
		EnvTimeout:   "2s",
	}))
	require.NoError(t, err)
	require.Equal(t, []string{"base.yaml", "resources.yaml", "override.yaml"}, env.DataIDs)
	require.Equal(t, 2*time.Second, env.Timeout)
}

func TestLoadEnvRejectsPartialCredentials(t *testing.T) {
	_, err := loadEnv(mapLookup(map[string]string{
		EnvService:   "user-service",
		EnvAddr:      "nacos:8848",
		EnvNamespace: "local",
		EnvGroup:     "AEGISCORE",
		EnvUsername:  "nacos",
	}))
	require.ErrorContains(t, err, "AEGISCORE_NACOS_USERNAME and AEGISCORE_NACOS_PASSWORD must be set together")
}

func TestLoadEnvPreservesPasswordWhitespace(t *testing.T) {
	env, err := loadEnv(mapLookup(map[string]string{
		EnvService:   "user-service",
		EnvAddr:      "nacos:8848",
		EnvNamespace: "local",
		EnvGroup:     "AEGISCORE",
		EnvUsername:  " nacos ",
		EnvPassword:  " secret-password ",
	}))
	require.NoError(t, err)
	require.Equal(t, "nacos", env.Username)
	require.Equal(t, " secret-password ", env.Password)
}

func mapLookup(values map[string]string) func(string) (string, bool) {
	return func(name string) (string, bool) {
		value, ok := values[name]
		return value, ok
	}
}
