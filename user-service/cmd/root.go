package main

import (
	"context"
	"time"

	"github.com/spf13/cobra"

	runtimefxgraph "github.com/aegiscore/common/runtime/fxgraph"
	serviceconfig "github.com/aegiscore/user-service/internal/config"
)

const defaultConfigLoadTimeout = 30 * time.Second

// rootCommandDependencies 包含 CLI 命令树执行各子命令所需的运行时依赖。
type rootCommandDependencies struct {
	appFactory                lifecycleAppFactory
	configLoader              configLoader
	seedRunner                rbacSeedRunner
	bootstrapSuperAdminRunner rbacBootstrapSuperAdminRunner
	fxGraphWriter             fxGraphWriter
}

func defaultRootCommandDependencies() rootCommandDependencies {
	return rootCommandDependencies{
		appFactory:                newBootstrapLifecycleApp,
		configLoader:              withConfigLoadTimeout(serviceconfig.Load, defaultConfigLoadTimeout),
		seedRunner:                newRBACSeedRunner(defaultRBACSeedDependencies),
		bootstrapSuperAdminRunner: newRBACBootstrapSuperAdminRunner(defaultRBACSeedDependencies),
		fxGraphWriter:             runtimefxgraph.WriteDOT,
	}
}

// withConfigLoadTimeout 为 CLI 构造阶段的远程配置加载施加总预算。
// 该预算不进入 common/runtime/config.Config，避免把 user-service 启动策略扩散到共享配置结构。
func withConfigLoadTimeout(loadConfig configLoader, timeout time.Duration) configLoader {
	return func(ctx context.Context) (*serviceconfig.LoadResult, error) {
		if timeout <= 0 {
			return loadConfig(ctx)
		}
		loadCtx, cancel := context.WithTimeout(ctx, timeout)
		defer cancel()
		return loadConfig(loadCtx)
	}
}

func newRootCommand(deps rootCommandDependencies) *cobra.Command {
	root := &cobra.Command{
		Use:   "aegiscore-user-service",
		Short: "AegisCore user services runtime",
	}

	root.AddCommand(
		newServeCommand(deps.appFactory, deps.configLoader),
		newRBACCommand(deps.seedRunner, deps.bootstrapSuperAdminRunner),
		newConfigCommand(deps.configLoader),
		newFxGraphCommand(deps.fxGraphWriter, deps.configLoader),
		newHealthcheckCommand(),
	)
	return root
}

type configLoader func(context.Context) (*serviceconfig.LoadResult, error)
