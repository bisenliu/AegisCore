package bootstrap

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"

	"go.uber.org/fx"
	"go.uber.org/zap"

	commonpprof "github.com/aegiscore/common/http/pprof"
	commonconfig "github.com/aegiscore/common/runtime/config"
	"github.com/aegiscore/common/runtime/logger"
)

// PprofServer 持有独立于业务 Gin router 的诊断 HTTP server。
type PprofServer struct {
	Server  *http.Server
	Enabled bool
}

// PprofServerParams 包含诊断 server 的已解析配置和生命周期依赖。
type PprofServerParams struct {
	fx.In

	Lifecycle  fx.Lifecycle
	Shutdowner fx.Shutdowner
	Config     *commonconfig.Config
	Log        *zap.Logger
}

// NewPprofServer 根据已解析配置创建诊断监听，并仅在启用时注册独立生命周期。
func NewPprofServer(params PprofServerParams) (*PprofServer, error) {
	if params.Config == nil {
		return nil, fmt.Errorf("config is required")
	}
	pprofCfg := params.Config.Observability.Pprof

	server := &http.Server{
		Addr:    pprofCfg.Addr,
		Handler: commonpprof.Handler(commonpprof.Options{}),
	}
	result := &PprofServer{Server: server, Enabled: pprofCfg.Enabled}
	if !pprofCfg.Enabled {
		return result, nil
	}

	pprofLog := logger.NamedComponent(params.Log, "pprof", "diagnostics")
	params.Lifecycle.Append(fx.Hook{
		OnStart: func(ctx context.Context) error {
			listener, err := net.Listen("tcp", pprofCfg.Addr)
			if err != nil {
				return fmt.Errorf("listen pprof server on %s: %w", pprofCfg.Addr, err)
			}
			logger.WithContext(ctx, pprofLog).Info("starting pprof server", zap.String("addr", pprofCfg.Addr))
			go servePprofServer(pprofLog, params.Shutdowner, server, listener)
			return nil
		},
		OnStop: func(ctx context.Context) error {
			logger.WithContext(ctx, pprofLog).Info("stopping pprof server")
			shutdownErr := server.Shutdown(ctx)
			if shutdownErr == nil {
				return nil
			}
			return errors.Join(
				fmt.Errorf("shutdown pprof server: %w", shutdownErr),
				server.Close(),
			)
		},
	})
	return result, nil
}

func servePprofServer(log *zap.Logger, shutdowner fx.Shutdowner, server *http.Server, listener net.Listener) {
	handlePprofServeExit(log, shutdowner, server.Serve(listener))
}

func handlePprofServeExit(log *zap.Logger, shutdowner fx.Shutdowner, err error) {
	if err == nil || errors.Is(err, http.ErrServerClosed) {
		return
	}
	log.Error("pprof server failed", logger.StackTrace(zap.Error(err))...)
	if shutdowner != nil {
		if shutdownErr := shutdowner.Shutdown(fx.ExitCode(1)); shutdownErr != nil {
			log.Error("shutdown after pprof server failure failed", logger.StackTrace(zap.Error(shutdownErr))...)
		}
	}
}
