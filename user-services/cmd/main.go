package main

// @title AegisCore User Services API
// @version 1.0.0
// @description AegisCore 用户服务 API 文档，覆盖用户资料查询、用户创建和服务健康检查。
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

	"github.com/aegiscore/user-services/internal/bootstrap"
	"github.com/spf13/cobra"
)

const (
	fxAppStartTimeout = 15 * time.Second
	fxAppStopTimeout  = 30 * time.Second
)

type lifecycleApp interface {
	Start(context.Context) error
	Stop(context.Context) error
}

var newLifecycleApp = func(configPath string) lifecycleApp {
	return bootstrap.NewApp(configPath)
}

func main() {
	if err := newRootCommand().Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func newRootCommand() *cobra.Command {
	var configPath string

	root := &cobra.Command{
		Use:   "aegiscore-user-services",
		Short: "AegisCore user services runtime",
	}

	serve := &cobra.Command{
		Use:   "serve",
		Short: "Start the AegisCore user services HTTP server",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runServe(cmd.Context(), configPath)
		},
	}
	serve.Flags().StringVar(&configPath, "config", "./configs/config.yaml", "path to YAML configuration file")
	root.AddCommand(serve)

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

	stopCtx, cancelStop := context.WithTimeout(context.WithoutCancel(upstreamCtx), fxAppStopTimeout)
	defer cancelStop()
	return app.Stop(stopCtx)
}
