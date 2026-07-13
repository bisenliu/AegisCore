package main

// @title AegisCore User Services API
// @version 1.0.0
// @description AegisCore 用户服务 API 文档，覆盖认证会话、用户资料、角色管理、权限目录、RBAC 授权保护的业务接口和服务健康检查。
// @host localhost:8080
// @BasePath /api/v1
// @schemes http https
// @accept json
// @produce json
// @securityDefinitions.apikey BearerAuth
// @in header
// @name Authorization
// @description 输入 Bearer token，格式为：Bearer <token>

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/google/uuid"
	"github.com/spf13/cobra"

	runtimefxgraph "github.com/aegiscore/common/runtime/fxgraph"
	"github.com/aegiscore/user-service/internal/bootstrap"
)

const (
	// fxAppStartTimeout 限制 Fx 启动时长，避免依赖失败导致 CLI 无限挂起。
	fxAppStartTimeout = 15 * time.Second
	// fxAppStopTimeout 限制 SIGINT 或 SIGTERM 后的优雅关闭时长。
	fxAppStopTimeout = 30 * time.Second
)

// lifecycleApp 是 runServe 和测试所需的最小 Fx 生命周期接口。
type lifecycleApp interface {
	Start(context.Context) error
	Stop(context.Context) error
}

type lifecycleAppFactory func(configPath string) lifecycleApp

type rbacSeedRunner func(context.Context, string, rbacSeedOptions) error

type rbacAssignSuperAdminRunner func(context.Context, string, uuid.UUID) error

type rbacCreateSuperAdminRunner func(context.Context, string, rbacCreateSuperAdminOptions) error

// rootCommandDependencies 是 CLI 命令树的依赖替换入口，用于测试和离线命令注入 runner，不承载服务运行时 provider 组装。
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

func (deps rootCommandDependencies) withDefaults() rootCommandDependencies {
	defaults := defaultRootCommandDependencies()
	if deps.appFactory == nil {
		deps.appFactory = defaults.appFactory
	}
	if deps.seedRunner == nil {
		deps.seedRunner = defaults.seedRunner
	}
	if deps.assignSuperAdminRunner == nil {
		deps.assignSuperAdminRunner = defaults.assignSuperAdminRunner
	}
	if deps.createSuperAdminRunner == nil {
		deps.createSuperAdminRunner = defaults.createSuperAdminRunner
	}
	if deps.fxGraphWriter == nil {
		deps.fxGraphWriter = defaults.fxGraphWriter
	}
	return deps
}

func newBootstrapLifecycleApp(configPath string) lifecycleApp {
	return bootstrap.NewApp(configPath)
}

func newRBACSeedRunner(newDependencies rbacSeedDependencyFactory) rbacSeedRunner {
	return func(ctx context.Context, configPath string, opts rbacSeedOptions) error {
		return runRBACSeedCommand(ctx, configPath, opts, newDependencies)
	}
}

func newRBACAssignSuperAdminRunner(newDependencies rbacSeedDependencyFactory) rbacAssignSuperAdminRunner {
	return func(ctx context.Context, configPath string, userID uuid.UUID) error {
		return runAssignSuperAdminCommand(ctx, configPath, userID, newDependencies)
	}
}

func newRBACCreateSuperAdminRunner(newDependencies rbacSeedDependencyFactory) rbacCreateSuperAdminRunner {
	return func(ctx context.Context, configPath string, opts rbacCreateSuperAdminOptions) error {
		return runCreateSuperAdminCommand(ctx, configPath, opts, newDependencies)
	}
}

func main() {
	if err := newRootCommand(defaultRootCommandDependencies()).Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func newRootCommand(deps rootCommandDependencies) *cobra.Command {
	deps = deps.withDefaults()

	var configPath string
	var rbacConfigPath string
	var reactivateSystem bool
	var syncSystemBindings bool
	var superAdminUserID string
	var fxGraphConfigPath string
	var fxGraphOutputPath string
	healthcheckOpts := healthcheckOptions{url: defaultHealthcheckURL, timeout: defaultHealthcheckTimeout}
	createSuperAdminOpts := rbacCreateSuperAdminOptions{username: defaultCreateSuperAdminUsername, nickname: defaultCreateSuperAdminNickname, passwordEnv: defaultCreateSuperAdminPasswordEnv}

	// serve 使用完整 Fx app；rbac 子命令使用最小离线依赖；healthcheck 和 fxgraph 只服务运维/开发辅助流程。
	root := &cobra.Command{
		Use:   "aegiscore-user-services",
		Short: "AegisCore user services runtime",
	}

	serve := &cobra.Command{
		Use:   "serve",
		Short: "Start the AegisCore user services HTTP server",
		RunE: func(cmd *cobra.Command, _ []string) error {
			return runServe(cmd.Context(), configPath, deps.appFactory)
		},
	}
	serve.Flags().StringVar(&configPath, "config", "./configs/config.yaml", "path to YAML configuration file")
	root.AddCommand(serve)

	rbac := &cobra.Command{
		Use:   "rbac",
		Short: "Manage RBAC seed data and bootstrap bindings",
	}
	rbac.PersistentFlags().StringVar(&rbacConfigPath, "config", "./configs/config.yaml", "path to YAML configuration file")

	seed := &cobra.Command{
		Use:   "seed",
		Short: "Seed default RBAC system roles, permissions, and bindings",
		RunE: func(cmd *cobra.Command, _ []string) error {
			return deps.seedRunner(cmd.Context(), rbacConfigPath, rbacSeedOptions{reactivateSystem: reactivateSystem, syncSystemBindings: syncSystemBindings})
		},
	}
	seed.Flags().BoolVar(&reactivateSystem, "reactivate-system", false, "reactivate catalog-managed system roles and permissions")
	seed.Flags().BoolVar(&syncSystemBindings, "sync-system-bindings", false, "synchronize catalog-managed system role permission bindings exactly")
	rbac.AddCommand(seed)

	assignSuperAdmin := &cobra.Command{
		Use:   "assign-super-admin --user-id <uuid>",
		Short: "Assign the built-in super admin role to a user",
		RunE: func(cmd *cobra.Command, _ []string) error {
			userID, err := uuid.Parse(superAdminUserID)
			if err != nil {
				return fmt.Errorf("invalid --user-id: %w", err)
			}
			return deps.assignSuperAdminRunner(cmd.Context(), rbacConfigPath, userID)
		},
	}
	assignSuperAdmin.Flags().StringVar(&superAdminUserID, "user-id", "", "user UUID to receive the built-in super admin role")
	_ = assignSuperAdmin.MarkFlagRequired("user-id")
	rbac.AddCommand(assignSuperAdmin)

	createSuperAdmin := &cobra.Command{
		Use:   "create-super-admin",
		Short: "Create the default admin user and assign the built-in super admin role",
		RunE: func(cmd *cobra.Command, _ []string) error {
			return deps.createSuperAdminRunner(cmd.Context(), rbacConfigPath, createSuperAdminOpts)
		},
	}
	createSuperAdmin.Flags().StringVar(&createSuperAdminOpts.username, "username", defaultCreateSuperAdminUsername, "admin username to create or bind")
	createSuperAdmin.Flags().StringVar(&createSuperAdminOpts.nickname, "nickname", defaultCreateSuperAdminNickname, "admin display nickname")
	createSuperAdmin.Flags().StringVar(&createSuperAdminOpts.passwordEnv, "password-env", defaultCreateSuperAdminPasswordEnv, "environment variable containing the admin password")
	createSuperAdmin.Flags().BoolVar(&createSuperAdminOpts.resetPassword, "reset-password", false, "reset password when the admin user already exists")
	rbac.AddCommand(createSuperAdmin)
	root.AddCommand(rbac)

	fxGraph := &cobra.Command{
		Use:   "fxgraph",
		Short: "Generate the user-service Fx dependency graph",
		RunE: func(_ *cobra.Command, _ []string) error {
			return runFxGraphCommand(fxGraphConfigPath, fxGraphOutputPath, deps.fxGraphWriter)
		},
	}
	fxGraph.Flags().StringVar(&fxGraphConfigPath, "config", "./configs/config.yaml", "path to YAML configuration file")
	fxGraph.Flags().StringVar(&fxGraphOutputPath, "output", defaultFxGraphOutputPath, "path to write DOT dependency graph")
	root.AddCommand(fxGraph)

	healthcheck := &cobra.Command{
		Use:   "healthcheck",
		Short: "Check user-service readiness without external runtime tools",
		RunE: func(cmd *cobra.Command, _ []string) error {
			return runHealthcheck(cmd.Context(), healthcheckOpts)
		},
	}
	healthcheck.Flags().StringVar(&healthcheckOpts.url, "url", defaultHealthcheckURL, "HTTP health endpoint URL")
	healthcheck.Flags().DurationVar(&healthcheckOpts.timeout, "timeout", defaultHealthcheckTimeout, "healthcheck request timeout")
	root.AddCommand(healthcheck)

	return root
}

func runServe(ctx context.Context, configPath string, appFactory lifecycleAppFactory) error {
	upstreamCtx := ctx
	ctx, stop := signal.NotifyContext(ctx, os.Interrupt, syscall.SIGTERM)
	defer stop()

	if appFactory == nil {
		appFactory = newBootstrapLifecycleApp
	}
	app := appFactory(configPath)
	startCtx, cancelStart := context.WithTimeout(ctx, fxAppStartTimeout)
	defer cancelStart()
	if err := app.Start(startCtx); err != nil {
		return err
	}

	<-ctx.Done()

	// 使用未被取消的父 context，使信号触发后优雅关闭仍能获得完整预算。
	stopCtx, cancelStop := context.WithTimeout(context.WithoutCancel(upstreamCtx), fxAppStopTimeout)
	defer cancelStop()
	return app.Stop(stopCtx)
}
