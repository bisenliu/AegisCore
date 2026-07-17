package bootstrap

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
	"go.uber.org/fx"

	serviceconfig "github.com/aegiscore/user-service/internal/config"
)

func TestAppModuleValidatesFullRuntimeGraph(t *testing.T) {
	cfg, err := serviceconfig.NewConfig(serviceconfig.ConfigPath(filepath.Join("..", "..", "configs", "config.yaml")))
	require.NoError(t, err)

	require.NoError(t, fx.ValidateApp(AppOptions(cfg, AppModule)...))
}
