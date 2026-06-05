package bootstrap

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"time"

	"github.com/aegiscore/common/runtime/config"
	"github.com/aegiscore/common/runtime/logger"
	"github.com/gin-gonic/gin"
	"go.uber.org/fx"
	"go.uber.org/zap"
)

const defaultHTTPShutdownTimeout = 10 * time.Second

type HTTPServerParams struct {
	fx.In

	Lifecycle fx.Lifecycle
	Config    *config.Config
	Log       *zap.Logger
	Engine    *gin.Engine
}

func NewHTTPServer(params HTTPServerParams) *http.Server {
	addr := fmt.Sprintf("%s:%d", params.Config.HTTP.Host, params.Config.HTTP.Port)
	server := &http.Server{
		Addr:         addr,
		Handler:      params.Engine,
		ReadTimeout:  params.Config.HTTP.ReadTimeout,
		WriteTimeout: params.Config.HTTP.WriteTimeout,
		IdleTimeout:  params.Config.HTTP.IdleTimeout,
	}

	params.Lifecycle.Append(fx.Hook{
		OnStart: func(ctx context.Context) error {
			logger.WithContext(params.Log, ctx).Info("starting http server",
				zap.String("addr", addr),
				zap.String("service", params.Config.App.Name),
				zap.String("environment", params.Config.App.Environment),
				zap.String("timezone", params.Config.System.Timezone),
			)
			listener, err := net.Listen("tcp", addr)
			if err != nil {
				return fmt.Errorf("listen http server on %s: %w", addr, err)
			}
			go func() {
				if err := server.Serve(listener); err != nil && !errors.Is(err, http.ErrServerClosed) {
					params.Log.Error("http server failed", logger.StackTrace(zap.Error(err))...)
				}
			}()
			return nil
		},
		OnStop: func(ctx context.Context) error {
			shutdownTimeout := params.Config.HTTP.ShutdownTimeout
			if shutdownTimeout == 0 {
				shutdownTimeout = defaultHTTPShutdownTimeout
			}
			shutdownCtx, cancel := context.WithTimeout(ctx, shutdownTimeout)
			defer cancel()
			logger.WithContext(params.Log, ctx).Info("stopping http server")
			return server.Shutdown(shutdownCtx)
		},
	})

	return server
}
