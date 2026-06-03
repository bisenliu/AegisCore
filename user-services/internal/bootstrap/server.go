package bootstrap

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/aegiscore/common/config"
	"github.com/aegiscore/common/logger"
	"github.com/gin-gonic/gin"
	"go.uber.org/fx"
	"go.uber.org/zap"
)

const defaultShutdownTimeout = 10 * time.Second

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
			logger.WithContext(params.Log, ctx).Info("starting http server", zap.String("addr", addr))
			go func() {
				if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
					params.Log.Error("http server failed", logger.StackTrace(zap.Error(err))...)
				}
			}()
			return nil
		},
		OnStop: func(ctx context.Context) error {
			shutdownTimeout := params.Config.HTTP.ShutdownTimeout
			if shutdownTimeout == 0 {
				shutdownTimeout = defaultShutdownTimeout
			}
			shutdownCtx, cancel := context.WithTimeout(ctx, shutdownTimeout)
			defer cancel()
			logger.WithContext(params.Log, ctx).Info("stopping http server")
			return server.Shutdown(shutdownCtx)
		},
	})

	return server
}
