package main

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"

	platformconfig "github.com/aegiscore/platform/runtime/config"
	platformnacos "github.com/aegiscore/platform/runtime/config/nacos"
	serviceconfig "github.com/aegiscore/user-service/internal/config"
	configtestkit "github.com/aegiscore/user-service/internal/config/testkit"
)

func setTestNacosEnv(t *testing.T) {
	t.Helper()
	t.Setenv(platformnacos.EnvService, "user-service")
	t.Setenv(platformnacos.EnvAddr, "127.0.0.1:8848")
	t.Setenv(platformnacos.EnvNamespace, "test")
	t.Setenv(platformnacos.EnvGroup, "AEGISCORE")
}

func loadRepositoryConfigForTest(t *testing.T) *serviceconfig.Config {
	t.Helper()
	result, err := configtestkit.LoadFromDocuments(readRepositoryConfigDocList(t))
	require.NoError(t, err)
	return result.Config
}

func readRepositoryConfigDocuments(t testing.TB) map[string][]byte {
	t.Helper()
	docs := make(map[string][]byte)
	for _, dataID := range []string{"base.yaml", "resources.yaml", "user-service.yaml"} {
		content, err := os.ReadFile(filepath.Join("..", "..", "deployments", "nacos", "local-docker", dataID))
		require.NoError(t, err)
		docs[dataID] = content
	}
	return docs
}

func readRepositoryConfigDocList(t testing.TB) []platformconfig.ConfigDocument {
	t.Helper()
	docs := readRepositoryConfigDocuments(t)
	return []platformconfig.ConfigDocument{
		{DataID: "base.yaml", Content: docs["base.yaml"]},
		{DataID: "resources.yaml", Content: docs["resources.yaml"]},
		{DataID: "user-service.yaml", Content: docs["user-service.yaml"]},
	}
}
