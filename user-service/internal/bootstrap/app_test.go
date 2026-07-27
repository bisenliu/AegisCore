package bootstrap

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
	"go.uber.org/fx"
	"go.uber.org/zap"
	"go.uber.org/zap/zaptest/observer"

	commonconfig "github.com/aegiscore/common/runtime/config"
	serviceconfig "github.com/aegiscore/user-service/internal/config"
)

func TestAppModuleValidatesFullRuntimeGraph(t *testing.T) {
	var docs []commonconfig.ConfigDocument
	for _, dataID := range []string{"base.yaml", "resources.yaml", "user-service.yaml"} {
		content, err := os.ReadFile(filepath.Join("..", "..", "..", "deployments", "nacos", "local-docker", dataID))
		require.NoError(t, err)
		docs = append(docs, commonconfig.ConfigDocument{DataID: dataID, Content: content})
	}
	result, err := serviceconfig.LoadFromDocuments(docs)
	require.NoError(t, err)
	cfg := result.Config

	require.NoError(t, fx.ValidateApp(AppOptions(cfg, AppModule)...))
}

func TestAppOptionsWiresFxEventsToInjectedLogger(t *testing.T) {
	core, logs := observer.New(zap.DebugLevel)
	app := fx.New(AppOptions(
		appModuleValidationTestConfig(),
		fx.Replace(zap.New(core)),
		fx.Invoke(func() {}),
	)...)

	require.NoError(t, app.Err())
	require.NoError(t, app.Start(context.Background()))
	require.NoError(t, app.Stop(context.Background()))

	entries := logs.FilterLoggerName("fx").AllUntimed()
	require.NotEmpty(t, entries)
	for _, entry := range entries {
		require.LessOrEqual(t, entry.Level, zap.DebugLevel)
	}
}
