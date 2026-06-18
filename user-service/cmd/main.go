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

// newLifecycleApp 构造服务生命周期 app，并允许测试替换。
var newLifecycleApp = func(configPath string) lifecycleApp {
	return bootstrap.NewApp(configPath)
}

var runRBACSeed = runRBACSeedCommand

var runAssignSuperAdmin = runAssignSuperAdminCommand

var runCreateSuperAdmin = runCreateSuperAdminCommand

func main() {
	if err := newRootCommand().Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func newRootCommand() *cobra.Command {
	var configPath string
	var rbacConfigPath string
	var reactivateSystem bool
	var syncSystemBindings bool
	var superAdminUserID string
	createSuperAdminOpts := rbacCreateSuperAdminOptions{username: defaultCreateSuperAdminUsername, nickname: defaultCreateSuperAdminNickname, passwordEnv: defaultCreateSuperAdminPasswordEnv}

	root := &cobra.Command{
		Use:   "aegiscore-user-services",
		Short: "AegisCore user services runtime",
	}

	serve := &cobra.Command{
		Use:   "serve",
		Short: "Start the AegisCore user services HTTP server",
		RunE: func(cmd *cobra.Command, _ []string) error {
			return runServe(cmd.Context(), configPath)
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
			return runRBACSeed(cmd.Context(), rbacConfigPath, rbacSeedOptions{reactivateSystem: reactivateSystem, syncSystemBindings: syncSystemBindings})
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
			return runAssignSuperAdmin(cmd.Context(), rbacConfigPath, userID)
		},
	}
	assignSuperAdmin.Flags().StringVar(&superAdminUserID, "user-id", "", "user UUID to receive the built-in super admin role")
	_ = assignSuperAdmin.MarkFlagRequired("user-id")
	rbac.AddCommand(assignSuperAdmin)

	createSuperAdmin := &cobra.Command{
		Use:   "create-super-admin",
		Short: "Create the default admin user and assign the built-in super admin role",
		RunE: func(cmd *cobra.Command, _ []string) error {
			return runCreateSuperAdmin(cmd.Context(), rbacConfigPath, createSuperAdminOpts)
		},
	}
	createSuperAdmin.Flags().StringVar(&createSuperAdminOpts.username, "username", defaultCreateSuperAdminUsername, "admin username to create or bind")
	createSuperAdmin.Flags().StringVar(&createSuperAdminOpts.nickname, "nickname", defaultCreateSuperAdminNickname, "admin display nickname")
	createSuperAdmin.Flags().StringVar(&createSuperAdminOpts.passwordEnv, "password-env", defaultCreateSuperAdminPasswordEnv, "environment variable containing the admin password")
	createSuperAdmin.Flags().BoolVar(&createSuperAdminOpts.resetPassword, "reset-password", false, "reset password when the admin user already exists")
	rbac.AddCommand(createSuperAdmin)
	root.AddCommand(rbac)

	return root
}

func runServe(ctx context.Context, configPath string) error {
	upstreamCtx := ctx
	ctx, stop := signal.NotifyContext(ctx, os.Interrupt, syscall.SIGTERM)
	defer stop()

	app := newLifecycleApp(configPath)
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
