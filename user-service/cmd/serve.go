package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/spf13/cobra"
	"go.uber.org/fx"

	"github.com/aegiscore/user-service/internal/bootstrap"
	serviceconfig "github.com/aegiscore/user-service/internal/config"
)

// lifecycleApp 是 runServe 启停服务所需的最小生命周期接口。
type lifecycleApp interface {
	Start(context.Context) error
	Wait() <-chan fx.ShutdownSignal
	Stop(context.Context) error
}

type lifecycleAppFactory func(cfg *serviceconfig.Config) lifecycleApp

func newBootstrapLifecycleApp(cfg *serviceconfig.Config) lifecycleApp {
	return bootstrap.NewApp(cfg)
}

func newServeCommand(appFactory lifecycleAppFactory) *cobra.Command {
	configPath := "./configs/config.yaml"
	cmd := &cobra.Command{
		Use:   "serve",
		Short: "Start the AegisCore user services HTTP server",
		RunE: func(cmd *cobra.Command, _ []string) error {
			return runServe(cmd.Context(), configPath, appFactory)
		},
	}
	cmd.Flags().StringVar(&configPath, "config", configPath, "path to the complete YAML configuration file")
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

	app := appFactory(cfg)
	// 手动调用 Start 时由此 context 提供实际 deadline，不会与 App 顶层 fx.StartTimeout 累加。
	startCtx, cancelStart := context.WithTimeout(ctx, lifecycleCfg.StartTimeout)
	defer cancelStart()
	if err := app.Start(startCtx); err != nil {
		return err
	}

	var shutdownSignal fx.ShutdownSignal
	select {
	case <-ctx.Done():
	case shutdownSignal = <-app.Wait():
	}

	// 使用未被取消的父 context，使信号触发后优雅关闭仍能获得完整预算；该 deadline 不会与 fx.StopTimeout 累加。
	stopCtx, cancelStop := context.WithTimeout(context.WithoutCancel(upstreamCtx), lifecycleCfg.StopTimeout)
	defer cancelStop()
	stopErr := app.Stop(stopCtx)

	var exitCodeErr error
	if shutdownSignal.ExitCode != 0 {
		exitCodeErr = fmt.Errorf("application shutdown requested with exit code %d", shutdownSignal.ExitCode)
	}
	if stopErr != nil {
		stopErr = fmt.Errorf("stop application: %w", stopErr)
	}
	return errors.Join(exitCodeErr, stopErr)
}
