package main

import (
	"github.com/spf13/cobra"

	runtimefxgraph "github.com/aegiscore/common/runtime/fxgraph"
)

// rootCommandDependencies 包含 CLI 命令树执行各子命令所需的运行时依赖。
type rootCommandDependencies struct {
	appFactory             lifecycleAppFactory
	seedRunner             rbacSeedRunner
	assignSuperAdminRunner rbacAssignSuperAdminRunner
	createSuperAdminRunner rbacCreateSuperAdminRunner
	fxGraphWriter          fxGraphWriter
}

func defaultRootCommandDependencies() rootCommandDependencies {
	return rootCommandDependencies{
		appFactory:             newBootstrapLifecycleApp,
		seedRunner:             newRBACSeedRunner(defaultRBACSeedDependencies),
		assignSuperAdminRunner: newRBACAssignSuperAdminRunner(defaultRBACSeedDependencies),
		createSuperAdminRunner: newRBACCreateSuperAdminRunner(defaultRBACSeedDependencies),
		fxGraphWriter:          runtimefxgraph.WriteDOT,
	}
}

func newRootCommand(deps rootCommandDependencies) *cobra.Command {
	root := &cobra.Command{
		Use:   "aegiscore-user-service",
		Short: "AegisCore user services runtime",
	}

	root.AddCommand(
		newServeCommand(deps.appFactory),
		newRBACCommand(deps.seedRunner, deps.assignSuperAdminRunner, deps.createSuperAdminRunner),
		newFxGraphCommand(deps.fxGraphWriter),
		newHealthcheckCommand(),
	)
	return root
}
