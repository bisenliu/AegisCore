package main

import (
	"context"
	"os"
	"os/signal"
	"syscall"

	"github.com/spf13/cobra"

	"github.com/aegiscore/user-service/internal/bootstrap"
	serviceconfig "github.com/aegiscore/user-service/internal/config"
)

// lifecycleApp 是 runServe 启停服务所需的最小生命周期接口。
type lifecycleApp interface {
	Start(context.Context) error
	Stop(context.Context) error
}

type lifecycleAppFactory func(configPath string) lifecycleApp

func newBootstrapLifecycleApp(configPath string) lifecycleApp {
	return bootstrap.NewApp(configPath)
}

func newServeCommand(appFactory lifecycleAppFactory) *cobra.Command {
	var configPath string
	cmd := &cobra.Command{
		Use:   "serve",
		Short: "Start the AegisCore user services HTTP server",
		RunE: func(cmd *cobra.Command, _ []string) error {
			return runServe(cmd.Context(), configPath, appFactory)
		},
	}
	cmd.Flags().StringVar(&configPath, "config", "./configs/config.yaml", "path to YAML configuration file")
	return cmd
}

func runServe(ctx context.Context, configPath string, appFactory lifecycleAppFactory) error {
	cfg, err := serviceconfig.NewConfig(serviceconfig.ConfigPath(configPath))
	if err != nil {
		return err
	}
	lifecycleCfg := cfg.Runtime.Lifecycle

	upstreamCtx := ctx
	ctx, stop := signal.NotifyContext(ctx, os.Interrupt, syscall.SIGTERM)
	defer stop()

	app := appFactory(configPath)
	startCtx, cancelStart := context.WithTimeout(ctx, lifecycleCfg.StartTimeout)
	defer cancelStart()
	if err := app.Start(startCtx); err != nil {
		return err
	}

	<-ctx.Done()

	// 使用未被取消的父 context，使信号触发后优雅关闭仍能获得完整预算。
	stopCtx, cancelStop := context.WithTimeout(context.WithoutCancel(upstreamCtx), lifecycleCfg.StopTimeout)
	defer cancelStop()
	return app.Stop(stopCtx)
}
