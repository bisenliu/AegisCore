package bootstrap

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
	"go.uber.org/fx"
	"go.uber.org/zap"
	"go.uber.org/zap/zaptest/observer"

	serviceconfig "github.com/aegiscore/user-service/internal/config"
)

func TestAppModuleValidatesFullRuntimeGraph(t *testing.T) {
	cfg, err := serviceconfig.NewConfig(serviceconfig.ConfigPath(filepath.Join("..", "..", "configs", "config.yaml")))
	require.NoError(t, err)

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
