package main

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
	"go.uber.org/fx"

	runtimefxgraph "github.com/aegiscore/common/runtime/fxgraph"
	commonlogger "github.com/aegiscore/common/runtime/logger"

	"github.com/aegiscore/user-service/internal/bootstrap"
	serviceconfig "github.com/aegiscore/user-service/internal/config"
)

func TestFxGraphCommandWritesGraph(t *testing.T) {
	called := 0
	deps := testRootCommandDependencies(t)
	deps.fxGraphWriter = func(path string, opts ...fx.Option) (string, error) {
		called++
		require.Equal(t, "docs/test.dot", path)
		dot, err := runtimefxgraph.RenderDOT(opts...)
		require.NoError(t, err)
		assertFxGraphContainsAppNodes(t, dot)
		return dot, nil
	}

	root := newRootCommand(deps)
	root.SetArgs([]string{"fxgraph", "--config", filepath.Join("..", "configs", "config.yaml"), "--output", "docs/test.dot"})
	require.NoError(t, root.Execute())
	require.Equal(t, 1, called)
}

func TestFxGraphOptionsRenderDOT(t *testing.T) {
	cfg, err := serviceconfig.NewConfig(serviceconfig.ConfigPath(filepath.Join("..", "configs", "config.yaml")))
	require.NoError(t, err)

	dot, err := runtimefxgraph.RenderDOT(fxGraphOptions(cfg)...)
	require.NoError(t, err)
	assertFxGraphContainsAppNodes(t, dot)
}

func TestFxGraphRenderDOTFailsWithoutServiceConfig(t *testing.T) {
	cfg, err := serviceconfig.NewConfig(serviceconfig.ConfigPath(filepath.Join("..", "configs", "config.yaml")))
	require.NoError(t, err)

	_, err = runtimefxgraph.RenderDOT(
		// Fx 分类：开发工具 - 只提供共享 runtime config，模拟缺失 user-service 私有配置的错误图。
		fx.Supply(serviceconfig.NewRuntimeConfig(cfg)),
		fx.Provide(
			// Fx 分类：基础运行时 - 与正式 AppOptions 相同的日志 provider。
			commonlogger.NewLogger,
		),
		// Fx 分类：开发工具 - 复用正式 composition root 校验缺失输入会失败。
		bootstrap.AppModule,
	)
	require.Error(t, err)
	require.ErrorContains(t, err, "missing type: *config.Config")
}

func assertFxGraphContainsAppNodes(t *testing.T, dot string) {
	t.Helper()
	require.NotEmpty(t, dot)
	for _, expected := range []string{
		"github.com/aegiscore/user-service/internal/bootstrap",
		"github.com/aegiscore/user-service/internal/features/auth",
		"github.com/aegiscore/user-service/internal/features/permission",
		"github.com/aegiscore/user-service/internal/features/role",
		"github.com/aegiscore/user-service/internal/features/user",
		"*config.Config",
		"*gin.Engine",
	} {
		require.Contains(t, dot, expected)
	}
	require.Contains(t, dot, "constructor_")
	require.Contains(t, dot, " -> ")
	require.NotContains(t, dot, "config.ConfigPath")
}
