package bootstrap

import (
	"context"
	"fmt"
	"net"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"go.uber.org/fx"
	"go.uber.org/zap"

	"github.com/aegiscore/common/runtime/config"
	"github.com/aegiscore/common/runtime/httpserver"
	"github.com/aegiscore/common/runtime/logger"
)

// HTTPRuntime 表达业务 HTTP server 的启用状态和可选运行实例。
type HTTPRuntime struct {
	Enabled bool
	Managed *httpserver.Managed
}

// HTTPServerParams 包含注册业务 HTTP server 生命周期所需的 Fx 输入。
type HTTPServerParams struct {
	fx.In

	Lifecycle  fx.Lifecycle
	Shutdowner fx.Shutdowner
	Config     *config.Config
	Log        *zap.Logger
	Engine     *gin.Engine
}

// NewHTTPServer 根据已解析配置创建业务 HTTP runtime，并在启用时注册 Fx hook。
func NewHTTPServer(params HTTPServerParams) (*HTTPRuntime, error) {
	if params.Config == nil {
		return nil, fmt.Errorf("create http runtime: config is required")
	}
	httpCfg := params.Config.Server.HTTP
	runtime := &HTTPRuntime{Enabled: httpCfg.Enabled}
	if !runtime.Enabled {
		return runtime, nil
	}
	if params.Lifecycle == nil {
		return nil, fmt.Errorf("create http runtime: lifecycle is required")
	}
	if params.Engine == nil {
		return nil, fmt.Errorf("create http runtime: gin engine is required")
	}

	addr := net.JoinHostPort(httpCfg.Host, strconv.Itoa(httpCfg.Port))
	httpLog := logger.NamedComponent(params.Log, "http", "http-server")
	managed, err := httpserver.New(httpserver.Options{
		Name:            "http",
		Addr:            addr,
		Handler:         params.Engine,
		ReadTimeout:     httpCfg.ReadTimeout,
		WriteTimeout:    httpCfg.WriteTimeout,
		IdleTimeout:     httpCfg.IdleTimeout,
		ShutdownTimeout: httpCfg.ShutdownTimeout,
		OnServeError:    newRuntimeServerFailureHandler(httpLog, params.Shutdowner, "http"),
	})
	if err != nil {
		return nil, fmt.Errorf("create http runtime: %w", err)
	}
	runtime.Managed = managed

	params.Lifecycle.Append(fx.Hook{
		OnStart: func(ctx context.Context) error {
			logger.WithContext(ctx, httpLog).Info("starting http server",
				zap.String("addr", addr),
				zap.String("service", params.Config.App.Name),
				zap.String("environment", params.Config.App.Environment),
				zap.String("timezone", time.Local.String()),
			)
			return managed.Start(ctx)
		},
		OnStop: func(ctx context.Context) error {
			logger.WithContext(ctx, httpLog).Info("stopping http server")
			return managed.Stop(ctx)
		},
	})
	return runtime, nil
}
