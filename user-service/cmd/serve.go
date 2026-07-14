package main

import (
	"context"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/spf13/cobra"

	"github.com/aegiscore/user-service/internal/bootstrap"
)

const (
	// fxAppStartTimeout 限制 Fx 启动时长，避免依赖失败导致 CLI 无限挂起。
	fxAppStartTimeout = 15 * time.Second
	// fxAppStopTimeout 限制 SIGINT 或 SIGTERM 后的优雅关闭时长。
	fxAppStopTimeout = 30 * time.Second
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
	upstreamCtx := ctx
	ctx, stop := signal.NotifyContext(ctx, os.Interrupt, syscall.SIGTERM)
	defer stop()

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
