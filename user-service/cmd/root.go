package main

import (
	"context"

	"github.com/spf13/cobra"

	runtimefxgraph "github.com/aegiscore/common/runtime/fxgraph"
	serviceconfig "github.com/aegiscore/user-service/internal/config"
)

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
		configLoader:              serviceconfig.Load,
		seedRunner:                newRBACSeedRunner(defaultRBACSeedDependencies),
		bootstrapSuperAdminRunner: newRBACBootstrapSuperAdminRunner(defaultRBACSeedDependencies),
		fxGraphWriter:             runtimefxgraph.WriteDOT,
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
