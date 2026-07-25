package main

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"

	commonconfig "github.com/aegiscore/common/runtime/config"
	serviceconfig "github.com/aegiscore/user-service/internal/config"
)

func setTestNacosEnv(t *testing.T) {
	t.Helper()
	t.Setenv(commonconfig.EnvService, "user-service")
	t.Setenv(commonconfig.EnvNacosAddr, "127.0.0.1:8848")
	t.Setenv(commonconfig.EnvNacosNamespace, "test")
	t.Setenv(commonconfig.EnvNacosGroup, "AEGISCORE")
}

func loadRepositoryConfigForTest(t *testing.T) *serviceconfig.Config {
	t.Helper()
	result, err := serviceconfig.LoadFromDocuments(readRepositoryConfigDocList(t))
	require.NoError(t, err)
	return result.Config
}

func readRepositoryConfigDocuments(t testing.TB) map[string][]byte {
	t.Helper()
	docs := make(map[string][]byte)
	for _, dataID := range []string{"base.yaml", "resources.yaml", "user-service.yaml"} {
		content, err := os.ReadFile(filepath.Join("..", "configs", "examples", dataID))
		require.NoError(t, err)
		docs[dataID] = content
	}
	return docs
}

func readRepositoryConfigDocList(t testing.TB) []commonconfig.ConfigDocument {
	t.Helper()
	docs := readRepositoryConfigDocuments(t)
	return []commonconfig.ConfigDocument{
		{DataID: "base.yaml", Content: docs["base.yaml"]},
		{DataID: "resources.yaml", Content: docs["resources.yaml"]},
		{DataID: "user-service.yaml", Content: docs["user-service.yaml"]},
	}
}
