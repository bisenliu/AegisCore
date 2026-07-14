package bootstrap

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"go.uber.org/fx"
	"go.uber.org/zap"

	"github.com/aegiscore/common/runtime/config"
	"github.com/aegiscore/common/runtime/logger"
)

// defaultHTTPShutdownTimeout 是配置缺省 server.http.shutdown_timeout 时使用的关闭超时。
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

// httpDrainTracker 跟踪正在执行的 HTTP handler，保护后续资源关闭顺序。
// 它只统计已经进入 Gin handler 的请求，不代表底层连接状态；Shutdown 失败后 Close 连接时用它等待 handler 退出。
type httpDrainTracker struct {
	handler http.Handler
	mu      sync.Mutex
	cond    *sync.Cond
	active  int
}

// NewHTTPServer 创建 HTTP server；仅在 server.http.enabled=true 时注册 Fx start/stop 生命周期 hook。
// OnStart 先完成 net.Listen 再异步 Serve，确保端口绑定失败能同步阻断启动；Serve 异常会通过 Shutdowner 触发全局停止。
func NewHTTPServer(params HTTPServerParams) *http.Server {
	httpCfg := params.Config.Server.HTTP
	addr := fmt.Sprintf("%s:%d", httpCfg.Host, httpCfg.Port)
	httpLog := logger.NamedComponent(params.Log, "http", "http-server")
	drainTracker := newHTTPDrainTracker(params.Engine)
	server := &http.Server{
		Addr:         addr,
		Handler:      drainTracker,
		ReadTimeout:  httpCfg.ReadTimeout,
		WriteTimeout: httpCfg.WriteTimeout,
		IdleTimeout:  httpCfg.IdleTimeout,
	}
	if !httpCfg.Enabled {
		return server
	}

	var cancelServe context.CancelFunc

	params.Lifecycle.Append(fx.Hook{
		OnStart: func(ctx context.Context) error {
			logger.WithContext(ctx, httpLog).Info("starting http server",
				zap.String("addr", addr),
				zap.String("service", params.Config.App.Name),
				zap.String("environment", params.Config.App.Environment),
				zap.String("timezone", time.Local.String()),
			)
			listener, err := net.Listen("tcp", addr)
			if err != nil {
				return fmt.Errorf("listen http server on %s: %w", addr, err)
			}
			serveCtx, cancel := context.WithCancel(context.Background())
			cancelServe = cancel
			go serveHTTPWithLifecycle(serveCtx, httpLog, params.Shutdowner, server, listener)
			return nil
		},
		OnStop: func(ctx context.Context) error {
			if cancelServe != nil {
				defer cancelServe()
			}
			shutdownTimeout := httpCfg.ShutdownTimeout
			if shutdownTimeout == 0 {
				// 0 表示配置未填写超时，应使用服务默认值而不是取消关闭边界。
				shutdownTimeout = defaultHTTPShutdownTimeout
			}
			shutdownCtx, cancel := context.WithTimeout(ctx, shutdownTimeout)
			defer cancel()
			logger.WithContext(ctx, httpLog).Info("stopping http server")
			if err := server.Shutdown(shutdownCtx); err != nil {
				return closeHTTPServerAfterShutdownError(ctx, httpLog, server, drainTracker, err)
			}
			return nil
		},
	})

	return server
}

// newHTTPDrainTracker 创建可等待活跃 handler 归零的 HTTP handler wrapper。
func newHTTPDrainTracker(handler http.Handler) *httpDrainTracker {
	tracker := &httpDrainTracker{handler: handler}
	tracker.cond = sync.NewCond(&tracker.mu)
	return tracker
}

// ServeHTTP 在进入真实 handler 前登记活跃请求，并在返回后解除登记。
func (t *httpDrainTracker) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	t.mu.Lock()
	t.active++
	t.mu.Unlock()
	defer func() {
		t.mu.Lock()
		t.active--
		if t.active == 0 {
			t.cond.Broadcast()
		}
		t.mu.Unlock()
	}()
	t.handler.ServeHTTP(w, r)
}

// Wait 等待所有已进入 handler 的请求完成，或在 context 到期时返回。
func (t *httpDrainTracker) Wait(ctx context.Context) error {
	stopContextWakeup := context.AfterFunc(ctx, func() {
		t.mu.Lock()
		t.cond.Broadcast()
		t.mu.Unlock()
	})
	defer stopContextWakeup()

	t.mu.Lock()
	defer t.mu.Unlock()
	waited := false
	for t.active > 0 {
		waited = true
		if err := ctx.Err(); err != nil {
			// 外层 Fx stop context 已到期时必须返回；Fx 会停止后续资源关闭 hook。
			return err
		}
		t.cond.Wait()
	}
	if waited {
		if err := ctx.Err(); err != nil {
			return err
		}
	}
	return nil
}

// closeHTTPServerAfterShutdownError 在优雅关闭失败后关闭活跃连接，并等待 handler 退出。
func closeHTTPServerAfterShutdownError(ctx context.Context, log *zap.Logger, server *http.Server, drainTracker *httpDrainTracker, shutdownErr error) error {
	if ctx.Err() != nil {
		return shutdownErr
	}

	logger.WithContext(ctx, log).Warn("http graceful shutdown failed; closing active connections", zap.Error(shutdownErr))
	closeErr := server.Close()
	waitErr := drainTracker.Wait(ctx)
	return errors.Join(
		fmt.Errorf("shutdown http server: %w", shutdownErr),
		wrapHTTPServerCloseError(closeErr),
		wrapHTTPDrainWaitError(waitErr),
	)
}

// wrapHTTPServerCloseError 过滤关闭 HTTP server 时的预期错误。
func wrapHTTPServerCloseError(err error) error {
	if err == nil || errors.Is(err, http.ErrServerClosed) || isExpectedHTTPServeCloseError(err) {
		return nil
	}
	return fmt.Errorf("close http server after shutdown failure: %w", err)
}

// wrapHTTPDrainWaitError 为等待活跃 HTTP handler 的错误补充上下文。
func wrapHTTPDrainWaitError(err error) error {
	if err == nil {
		return nil
	}
	return fmt.Errorf("wait http handlers after shutdown failure: %w", err)
}

func serveHTTPWithLifecycle(ctx context.Context, log *zap.Logger, shutdowner fx.Shutdowner, server *http.Server, listener net.Listener) {
	// lifecycle ctx 取消时主动关闭 listener，用于唤醒阻塞的 Serve；正常 Shutdown 产生的关闭错误由 handleHTTPServeExit 过滤。
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
