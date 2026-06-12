package bootstrap

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"go.uber.org/fx"
	"go.uber.org/zap"

	"github.com/aegiscore/common/runtime/config"
	"github.com/aegiscore/common/runtime/logger"
)

// defaultHTTPShutdownTimeout 是配置缺省 http.shutdown_timeout 时使用的关闭超时。
const defaultHTTPShutdownTimeout = 10 * time.Second

// HTTPServerParams 包含注册 HTTP server 生命周期所需的 Fx 输入。
type HTTPServerParams struct {
	fx.In

	Lifecycle  fx.Lifecycle
	Shutdowner fx.Shutdowner
	Config     *config.Config
	Log        *zap.Logger
	Engine     *gin.Engine
}

// NewHTTPServer 创建 HTTP server，并注册 Fx start/stop 生命周期 hook。
func NewHTTPServer(params HTTPServerParams) *http.Server {
	addr := fmt.Sprintf("%s:%d", params.Config.HTTP.Host, params.Config.HTTP.Port)
	server := &http.Server{
		Addr:         addr,
		Handler:      params.Engine,
		ReadTimeout:  params.Config.HTTP.ReadTimeout,
		WriteTimeout: params.Config.HTTP.WriteTimeout,
		IdleTimeout:  params.Config.HTTP.IdleTimeout,
	}
	cancelServe := func() {}

	params.Lifecycle.Append(fx.Hook{
		OnStart: func(ctx context.Context) error {
			logger.WithContext(ctx, params.Log).Info("starting http server",
				zap.String("addr", addr),
				zap.String("service", params.Config.App.Name),
				zap.String("environment", params.Config.App.Environment),
				zap.String("timezone", params.Config.System.Timezone),
			)
			listener, err := net.Listen("tcp", addr)
			if err != nil {
				return fmt.Errorf("listen http server on %s: %w", addr, err)
			}
			serveCtx, cancel := context.WithCancel(context.Background())
			cancelServe = cancel
			go serveHTTPWithLifecycle(serveCtx, params.Log, params.Shutdowner, server, listener)
			return nil
		},
		OnStop: func(ctx context.Context) error {
			cancelServe()
			shutdownTimeout := params.Config.HTTP.ShutdownTimeout
			if shutdownTimeout == 0 {
				// 0 表示配置未填写超时，应使用服务默认值而不是取消关闭边界。
				shutdownTimeout = defaultHTTPShutdownTimeout
			}
			shutdownCtx, cancel := context.WithTimeout(ctx, shutdownTimeout)
			defer cancel()
			logger.WithContext(ctx, params.Log).Info("stopping http server")
			return server.Shutdown(shutdownCtx)
		},
	})

	return server
}

func serveHTTPWithLifecycle(ctx context.Context, log *zap.Logger, shutdowner fx.Shutdowner, server *http.Server, listener net.Listener) {
	stopCancelListener := context.AfterFunc(ctx, func() {
		logger.WithContext(ctx, log).Debug("http server lifecycle context canceled")
		if err := listener.Close(); err != nil && !isExpectedHTTPServeCloseError(err) {
			log.Warn("close http listener after lifecycle cancel failed", zap.Error(err))
		}
	})
	err := server.Serve(listener)
	stopCancelListener()
	handleHTTPServeExit(ctx, log, shutdowner, err)
}

func handleHTTPServeExit(ctx context.Context, log *zap.Logger, shutdowner fx.Shutdowner, err error) {
	if err == nil {
		log.Debug("http server goroutine stopped", zap.String("reason", "serve_returned"))
		return
	}
	if errors.Is(err, http.ErrServerClosed) {
		log.Debug("http server goroutine stopped", zap.String("reason", "server_closed"))
		return
	}
	if ctx.Err() != nil && isExpectedHTTPServeCloseError(err) {
		log.Debug("http server goroutine stopped", zap.String("reason", "lifecycle_canceled"), zap.Error(ctx.Err()))
		return
	}
	shutdownOnHTTPServeError(log, shutdowner, err)
}

func shutdownOnHTTPServeError(log *zap.Logger, shutdowner fx.Shutdowner, err error) {
	if err == nil || errors.Is(err, http.ErrServerClosed) {
		// http.ErrServerClosed 是正常优雅关闭过程产生的错误，不应再次停止 Fx app。
		return
	}

	log.Error("http server failed", logger.StackTrace(zap.Error(err))...)
	if shutdowner == nil {
		return
	}
	if shutdownErr := shutdowner.Shutdown(); shutdownErr != nil {
		log.Error("shutdown after http server failure failed", logger.StackTrace(zap.Error(shutdownErr))...)
	}
}

func isExpectedHTTPServeCloseError(err error) bool {
	return errors.Is(err, net.ErrClosed) || errors.Is(err, context.Canceled)
}
