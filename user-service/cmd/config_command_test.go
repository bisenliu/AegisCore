package main

import (
	"bytes"
	"context"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"

	serviceconfig "github.com/aegiscore/user-service/internal/config"
)

func TestConfigSourcesCommandUsesDefaultDataIDs(t *testing.T) {
	t.Setenv("AEGISCORE_SERVICE", "user-service")
	t.Setenv("AEGISCORE_NACOS_ADDR", "nacos:8848")
	t.Setenv("AEGISCORE_NACOS_NAMESPACE", "local")
	t.Setenv("AEGISCORE_NACOS_GROUP", "AEGISCORE")

	root := newRootCommand(testRootCommandDependencies(t))
	var out bytes.Buffer
	root.SetOut(&out)
	root.SetArgs([]string{"config", "sources"})
	require.NoError(t, root.Execute())
	require.Contains(t, out.String(), "config_provider: nacos")
	require.Contains(t, out.String(), "config_data_ids: base.yaml,resources.yaml,user-service.yaml")
}

func TestConfigRenderRedactsSecrets(t *testing.T) {
	setTestNacosEnv(t)
	docs := readRepositoryConfigDocList(t)
	for index := range docs {
		if docs[index].DataID != "user-service.yaml" {
			if docs[index].DataID == "resources.yaml" {
				content := strings.Replace(string(docs[index].Content), `      password: ""`, `      password: redis-render-secret`, 1)
				content = strings.Replace(content, `      password: ""`, `      password: postgres-render-secret`, 1)
				docs[index].Content = []byte(content)
			}
			continue
		}
		content := strings.Replace(string(docs[index].Content), `  token_version_cache:
    enabled: true
    size: 100000
    ttl: 1s
    load_timeout: 300ms
`, "", 1)
		docs[index].Content = []byte(content)
	}
	deps := testRootCommandDependencies(t)
	deps.configLoader = func(context.Context) (*serviceconfig.LoadResult, error) {
		return serviceconfig.LoadFromDocuments(docs)
	}
	root := newRootCommand(deps)
	var out bytes.Buffer
	root.SetOut(&out)
	root.SetArgs([]string{"config", "render"})
	require.NoError(t, root.Execute())
	require.NotContains(t, out.String(), "local-development-secret")
	require.NotContains(t, out.String(), "redis-render-secret")
	require.NotContains(t, out.String(), "postgres-render-secret")
	require.Contains(t, out.String(), "***")

	var rendered struct {
		Auth struct {
			JWT struct {
				Secret string `yaml:"secret"`
			} `yaml:"jwt"`
			TokenVersionCache struct {
				Enabled     bool   `yaml:"enabled"`
				Size        int64  `yaml:"size"`
				TTL         string `yaml:"ttl"`
				LoadTimeout string `yaml:"load_timeout"`
			} `yaml:"token_version_cache"`
		} `yaml:"auth"`
		Resources struct {
			Redis map[string]struct {
				Password string `yaml:"password"`
			} `yaml:"redis"`
			Postgres map[string]struct {
				Password string `yaml:"password"`
			} `yaml:"postgres"`
		} `yaml:"resources"`
	}
	require.NoError(t, yaml.Unmarshal(out.Bytes(), &rendered))
	require.True(t, rendered.Auth.TokenVersionCache.Enabled)
	require.EqualValues(t, 100000, rendered.Auth.TokenVersionCache.Size)
	require.Equal(t, "1s", rendered.Auth.TokenVersionCache.TTL)
	require.Equal(t, "300ms", rendered.Auth.TokenVersionCache.LoadTimeout)
	require.Equal(t, "***", rendered.Auth.JWT.Secret)
	require.Equal(t, "***", rendered.Resources.Redis["cache_redis"].Password)
	require.Equal(t, "***", rendered.Resources.Postgres["primary_db"].Password)
}

func TestLegacyConfigFlagRejected(t *testing.T) {
	root := newRootCommand(testRootCommandDependencies(t))
	root.SetArgs([]string{"serve", "--config", "./configs/config.yaml"})
	require.ErrorContains(t, root.Execute(), "unknown flag: --config")

	root = newRootCommand(testRootCommandDependencies(t))
	root.SetArgs([]string{"rbac", "--config", "./configs/config.yaml", "seed"})
	require.ErrorContains(t, root.Execute(), "unknown flag: --config")
}
