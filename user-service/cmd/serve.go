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
	"go.uber.org/zap"

	commonlogger "github.com/aegiscore/common/runtime/logger"
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

func newServeCommand(appFactory lifecycleAppFactory, loadConfig configLoader) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "serve",
		Short: "Start the AegisCore user services HTTP server",
		RunE: func(cmd *cobra.Command, _ []string) error {
			return runServe(cmd.Context(), appFactory, loadConfig)
		},
	}
	return cmd
}

func runServe(ctx context.Context, appFactory lifecycleAppFactory, loadConfig configLoader) error {
	loaded, err := loadConfig(ctx)
	if err != nil {
		return err
	}
	cfg := loaded.Config
	lifecycleCfg := cfg.Runtime.Lifecycle
	logConfigSource(loaded)

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

func logConfigSource(loaded *serviceconfig.LoadResult) {
	if loaded == nil || loaded.Config == nil {
		return
	}
	runtimeCfg := loaded.Config.RuntimeConfig()
	log, err := commonlogger.New(&runtimeCfg)
	if err != nil {
		return
	}
	defer func() { _ = log.Sync() }()
	source := loaded.Source
	log.Info("runtime config loaded",
		zap.String("config_provider", source.Provider),
		zap.String("config_namespace", source.Namespace),
		zap.String("config_group", source.Group),
		zap.String("config_data_ids", source.DataIDsCSV()),
		zap.String("config_service", source.Service),
		zap.String("config_digest", source.Digest),
	)
}
