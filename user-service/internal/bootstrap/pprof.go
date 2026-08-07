package bootstrap

import (
	"context"
	"fmt"

	"go.uber.org/fx"
	"go.uber.org/zap"

	commonpprof "github.com/aegiscore/common/http/pprof"
	commonconfig "github.com/aegiscore/common/runtime/config"
	"github.com/aegiscore/common/runtime/httpserver"
	"github.com/aegiscore/common/runtime/logger"
)

// PprofRuntime 表达独立 pprof server 的启用状态和可选运行实例。
type PprofRuntime struct {
	Enabled bool
	Managed *httpserver.Managed
}

// PprofServerParams 包含诊断 server 的已解析配置和生命周期依赖。
type PprofServerParams struct {
	fx.In

	Lifecycle  fx.Lifecycle
	Shutdowner fx.Shutdowner
	Config     *commonconfig.Config
	Log        *zap.Logger
}

// NewPprofServer 根据已解析配置创建 pprof runtime，并在启用时注册 Fx hook。
func NewPprofServer(params PprofServerParams) (*PprofRuntime, error) {
	if params.Config == nil {
		return nil, fmt.Errorf("create pprof runtime: config is required")
	}
	pprofCfg := params.Config.Observability.Pprof
	runtime := &PprofRuntime{Enabled: pprofCfg.Enabled}
	if !runtime.Enabled {
		return runtime, nil
	}
	if params.Lifecycle == nil {
		return nil, fmt.Errorf("create pprof runtime: lifecycle is required")
	}

	pprofLog := logger.NamedComponent(params.Log, "pprof", "diagnostics")
	managed, err := httpserver.New(httpserver.Options{
		Name:            "pprof",
		Addr:            pprofCfg.Addr,
		Handler:         commonpprof.Handler(commonpprof.Options{}),
		ShutdownTimeout: params.Config.Server.HTTP.ShutdownTimeout,
		OnServeError:    newRuntimeServerFailureHandler(pprofLog, params.Shutdowner, "pprof"),
	})
	if err != nil {
		return nil, fmt.Errorf("create pprof runtime: %w", err)
	}
	runtime.Managed = managed

	params.Lifecycle.Append(fx.Hook{
		OnStart: func(ctx context.Context) error {
			logger.WithContext(ctx, pprofLog).Info("starting pprof server", zap.String("addr", pprofCfg.Addr))
			return managed.Start(ctx)
		},
		OnStop: func(ctx context.Context) error {
			logger.WithContext(ctx, pprofLog).Info("stopping pprof server")
			return managed.Stop(ctx)
		},
	})
	return runtime, nil
}
