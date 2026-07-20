package bootstrap

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"go.uber.org/fx"
	"go.uber.org/fx/fxtest"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"go.uber.org/zap/zaptest/observer"

	"github.com/aegiscore/common/runtime/config"
	commonlogger "github.com/aegiscore/common/runtime/logger"
)

type lifecycleRecorder struct {
	hooks []fx.Hook
}

func (r *lifecycleRecorder) Append(hook fx.Hook) {
	r.hooks = append(r.hooks, hook)
}

type shutdownRecorder struct {
	calls    int
	err      error
	delegate fx.Shutdowner
}

func (r *shutdownRecorder) Shutdown(options ...fx.ShutdownOption) error {
	r.calls++
	if r.err != nil {
		return r.err
	}
	if r.delegate != nil {
		return r.delegate.Shutdown(options...)
	}
	return nil
}

func newShutdownSignalRecorder(t testing.TB) (*shutdownRecorder, <-chan fx.ShutdownSignal) {
	t.Helper()
	var shutdowner fx.Shutdowner
	app := fx.New(fxtest.WithTestLogger(t), fx.Populate(&shutdowner))
	require.NoError(t, app.Start(context.Background()))
	t.Cleanup(func() {
		require.NoError(t, app.Stop(context.Background()))
	})
	return &shutdownRecorder{delegate: shutdowner}, app.Wait()
}

func requireShutdownSignal(t testing.TB, signals <-chan fx.ShutdownSignal) fx.ShutdownSignal {
	t.Helper()
	select {
	case signal := <-signals:
		return signal
	case <-time.After(time.Second):
		t.Fatal("shutdown signal was not received")
		return fx.ShutdownSignal{}
	}
}

func TestDefaultConfigHTTPTimeouts(t *testing.T) {
	cfg := config.DefaultConfig()

	require.True(t, cfg.Server.HTTP.Enabled)
	require.Equal(t, 30*time.Second, cfg.Server.HTTP.ReadTimeout)
	require.Equal(t, 60*time.Second, cfg.Server.HTTP.WriteTimeout)
	require.Equal(t, 120*time.Second, cfg.Server.HTTP.IdleTimeout)
	require.Equal(t, 10*time.Second, cfg.Server.HTTP.ShutdownTimeout)
}

func TestHTTPServerUsesConfiguredTimeouts(t *testing.T) {
	lifecycle := &lifecycleRecorder{}
	cfg := httpServerTestRuntimeConfig(config.HTTPServerConfig{
		Host:            "127.0.0.1",
		Port:            18080,
		ReadTimeout:     30 * time.Second,
		WriteTimeout:    60 * time.Second,
		IdleTimeout:     120 * time.Second,
		ShutdownTimeout: 25 * time.Second,
	})
	server := NewHTTPServer(HTTPServerParams{
		Lifecycle: lifecycle,
		Config:    cfg,
		Log:       zap.NewNop(),
		Engine:    gin.New(),
	})

	require.Equal(t, cfg.Server.HTTP.ReadTimeout, server.ReadTimeout)
	require.Equal(t, cfg.Server.HTTP.WriteTimeout, server.WriteTimeout)
	require.Equal(t, cfg.Server.HTTP.IdleTimeout, server.IdleTimeout)
	require.Len(t, lifecycle.hooks, 1)
	require.NotNil(t, lifecycle.hooks[0].OnStop)
}

func TestHTTPServerStartReturnsListenError(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	defer listener.Close()

	addr := listener.Addr().(*net.TCPAddr)
	lifecycle := &lifecycleRecorder{}
	NewHTTPServer(HTTPServerParams{
		Lifecycle: lifecycle,
		Config: httpServerTestRuntimeConfig(config.HTTPServerConfig{
			Host: "127.0.0.1",
			Port: addr.Port,
		}),
		Log:    zap.NewNop(),
		Engine: gin.New(),
	})

	require.Len(t, lifecycle.hooks, 1)
	require.NotNil(t, lifecycle.hooks[0].OnStart)
	err = lifecycle.hooks[0].OnStart(context.Background())
	require.ErrorContains(t, err, "listen http server")
}

func TestHTTPServerDisabledDoesNotRegisterLifecycleOrListen(t *testing.T) {
	port := reserveHTTPTestPort(t)
	lifecycle := &lifecycleRecorder{}
	server := NewHTTPServer(HTTPServerParams{
		Lifecycle: lifecycle,
		Config: &config.Config{Server: config.ServerConfig{
			HTTP: config.HTTPServerConfig{Enabled: false, Host: "127.0.0.1", Port: port},
			GRPC: config.GRPCServerConfig{Enabled: true},
		}},
		Log:    zap.NewNop(),
		Engine: gin.New(),
	})

	require.Equal(t, fmt.Sprintf("127.0.0.1:%d", port), server.Addr)
	require.Empty(t, lifecycle.hooks)
	listener, err := net.Listen("tcp", server.Addr)
	require.NoError(t, err)
	require.NoError(t, listener.Close())
}

func TestHTTPServerDisabledAllowsFxAppStartAndStop(t *testing.T) {
	cfg := &config.Config{Server: config.ServerConfig{
		HTTP: config.HTTPServerConfig{Enabled: false},
		GRPC: config.GRPCServerConfig{Enabled: true},
	}}
	app := fx.New(
		fx.Supply(cfg, zap.NewNop(), gin.New()),
		fx.Provide(NewHTTPServer),
		fx.Invoke(func(*http.Server) {}),
	)

	require.NoError(t, app.Err())
	require.NoError(t, app.Start(context.Background()))
	require.NoError(t, app.Stop(context.Background()))
}

func TestHTTPServerUnexpectedServeErrorTriggersShutdown(t *testing.T) {
	core, logs := observer.New(zapcore.ErrorLevel)
	shutdowner, signals := newShutdownSignalRecorder(t)
	serveErr := errors.New("serve failed")

	shutdownOnHTTPServeError(zap.New(core), shutdowner, serveErr)

	require.Equal(t, 1, shutdowner.calls)
	require.Equal(t, 1, requireShutdownSignal(t, signals).ExitCode)
	entries := logs.FilterMessage("http server failed").All()
	require.Len(t, entries, 1)
	if loggedErr, ok := entries[0].ContextMap()["error"].(string); !ok || loggedErr != serveErr.Error() {
		require.Equal(t, serveErr.Error(), loggedErr)
	}
}

func TestHTTPServerUnexpectedServeErrorLogsShutdownFailure(t *testing.T) {
	core, logs := observer.New(zapcore.ErrorLevel)
	shutdownErr := errors.New("shutdown failed")
	shutdowner := &shutdownRecorder{err: shutdownErr}

	shutdownOnHTTPServeError(zap.New(core), shutdowner, errors.New("serve failed"))

	require.Equal(t, 1, shutdowner.calls)
	entries := logs.FilterMessage("shutdown after http server failure failed").All()
	require.Len(t, entries, 1)
	if loggedErr, ok := entries[0].ContextMap()["error"].(string); !ok || loggedErr != shutdownErr.Error() {
		require.Equal(t, shutdownErr.Error(), loggedErr)
	}
}

func TestHTTPServerClosedServeErrorDoesNotTriggerShutdown(t *testing.T) {
	core, logs := observer.New(zapcore.ErrorLevel)
	shutdowner := &shutdownRecorder{}

	shutdownOnHTTPServeError(zap.New(core), shutdowner, http.ErrServerClosed)
	shutdownOnHTTPServeError(zap.New(core), shutdowner, nil)

	require.Equal(t, 0, shutdowner.calls)
	require.Equal(t, 0, logs.Len())
}

func TestHTTPServerLifecycleCancelStopsServeGoroutine(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)

	core, logs := observer.New(zapcore.DebugLevel)
	shutdowner := &shutdownRecorder{}
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		defer close(done)
		serveHTTPWithLifecycle(ctx, zap.New(core), shutdowner, &http.Server{}, listener)
	}()

	cancel()
	requireEventuallyClosed(t, done, time.Second)

	require.Equal(t, 0, shutdowner.calls)
	require.Empty(t, logs.FilterMessage("http server failed").All())
	entries := logs.FilterMessage("http server goroutine stopped").All()
	require.Len(t, entries, 1)
	require.Equal(t, "lifecycle_canceled", entries[0].ContextMap()["reason"])
}

func TestHTTPServerStartAndStop(t *testing.T) {
	lifecycle := &lifecycleRecorder{}
	NewHTTPServer(HTTPServerParams{
		Lifecycle: lifecycle,
		Config: httpServerTestRuntimeConfig(config.HTTPServerConfig{
			Host:            "127.0.0.1",
			Port:            0,
			ShutdownTimeout: time.Second,
		}),
		Log:    zap.NewNop(),
		Engine: gin.New(),
	})

	require.Len(t, lifecycle.hooks, 1)
	require.NotNil(t, lifecycle.hooks[0].OnStart)
	require.NotNil(t, lifecycle.hooks[0].OnStop)
	require.NoError(t, lifecycle.hooks[0].OnStart(context.Background()))
	stopCtx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	require.NoError(t, lifecycle.hooks[0].OnStop(stopCtx))
}

func TestHTTPServerStopWaitsForActiveRequest(t *testing.T) {
	port := reserveHTTPTestPort(t)
	lifecycle := &lifecycleRecorder{}
	started := make(chan struct{})
	release := make(chan struct{})
	var releaseOnce sync.Once
	releaseRequest := func() {
		releaseOnce.Do(func() {
			close(release)
		})
	}
	t.Cleanup(releaseRequest)

	engine := gin.New()
	engine.GET("/slow", func(c *gin.Context) {
		close(started)
		<-release
		c.String(http.StatusOK, "ok")
	})
	NewHTTPServer(HTTPServerParams{
		Lifecycle: lifecycle,
		Config: httpServerTestRuntimeConfig(config.HTTPServerConfig{
			Host:            "127.0.0.1",
			Port:            port,
			ShutdownTimeout: time.Second,
		}),
		Log:    zap.NewNop(),
		Engine: engine,
	})

	require.NoError(t, lifecycle.hooks[0].OnStart(context.Background()))
	defer func() {
		stopCtx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		_ = lifecycle.hooks[0].OnStop(stopCtx)
	}()

	type responseResult struct {
		status int
		err    error
	}
	responseDone := make(chan responseResult, 1)
	go func() {
		client := &http.Client{Timeout: 2 * time.Second}
		resp, err := client.Get(fmt.Sprintf("http://127.0.0.1:%d/slow", port))
		if err != nil {
			responseDone <- responseResult{err: err}
			return
		}
		defer resp.Body.Close()
		_, err = io.ReadAll(resp.Body)
		if err != nil {
			responseDone <- responseResult{err: err}
			return
		}
		responseDone <- responseResult{status: resp.StatusCode}
	}()

	requireEventuallyClosed(t, started, time.Second)

	stopDone := make(chan error, 1)
	go func() {
		stopCtx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		stopDone <- lifecycle.hooks[0].OnStop(stopCtx)
	}()

	require.Never(t, func() bool {
		select {
		case <-stopDone:
			return true
		default:
			return false
		}
	}, 100*time.Millisecond, 10*time.Millisecond)

	releaseRequest()
	select {
	case err := <-stopDone:
		require.NoError(t, err)
	case <-time.After(time.Second):
		t.Fatal("HTTP server stop blocked after active request release")
	}
	result := <-responseDone
	require.NoError(t, result.err)
	require.Equal(t, http.StatusOK, result.status)
}

func TestHTTPServerStopClosesAndDrainsActiveRequestAfterShutdownTimeout(t *testing.T) {
	port := reserveHTTPTestPort(t)
	lifecycle := &lifecycleRecorder{}
	started := make(chan struct{})
	exited := make(chan struct{})

	engine := gin.New()
	engine.GET("/blocked", func(c *gin.Context) {
		close(started)
		<-c.Request.Context().Done()
		close(exited)
	})
	NewHTTPServer(HTTPServerParams{
		Lifecycle: lifecycle,
		Config: httpServerTestRuntimeConfig(config.HTTPServerConfig{
			Host:            "127.0.0.1",
			Port:            port,
			ShutdownTimeout: 20 * time.Millisecond,
		}),
		Log:    zap.NewNop(),
		Engine: engine,
	})

	require.NoError(t, lifecycle.hooks[0].OnStart(context.Background()))
	defer func() {
		stopCtx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		_ = lifecycle.hooks[0].OnStop(stopCtx)
	}()

	responseDone := make(chan error, 1)
	go func() {
		client := &http.Client{Timeout: 2 * time.Second}
		resp, err := client.Get(fmt.Sprintf("http://127.0.0.1:%d/blocked", port))
		if err != nil {
			responseDone <- err
			return
		}
		defer resp.Body.Close()
		_, err = io.ReadAll(resp.Body)
		responseDone <- err
	}()

	requireEventuallyClosed(t, started, time.Second)

	stopCtx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	stopDone := make(chan error, 1)
	go func() { stopDone <- lifecycle.hooks[0].OnStop(stopCtx) }()
	var err error
	select {
	case err = <-stopDone:
	case <-time.After(time.Second):
		t.Fatal("HTTP server stop did not honor shutdown timeout")
	}
	require.ErrorIs(t, err, context.DeadlineExceeded)

	requireEventuallyClosed(t, exited, time.Second)
	requireEventuallyReceives(t, responseDone, time.Second)
}

func TestHTTPServerStartLogIncludesRuntimeIdentity(t *testing.T) {
	lifecycle := &lifecycleRecorder{}
	core, logs := observer.New(zapcore.InfoLevel)
	log := zap.New(core)
	NewHTTPServer(HTTPServerParams{
		Lifecycle: lifecycle,
		Config: &config.Config{
			App: config.AppConfig{Name: "aegiscore-user-service", Environment: "local"},
			Server: config.ServerConfig{
				HTTP: config.HTTPServerConfig{
					Enabled: true,
					Host:    "127.0.0.1",
					Port:    0,
				},
			},
		},
		Log:    log,
		Engine: gin.New(),
	})

	require.Len(t, lifecycle.hooks, 1)
	require.NotNil(t, lifecycle.hooks[0].OnStart)
	require.NotNil(t, lifecycle.hooks[0].OnStop)
	require.NoError(t, lifecycle.hooks[0].OnStart(context.Background()))
	stopCtx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	require.NoError(t, lifecycle.hooks[0].OnStop(stopCtx))

	entries := logs.FilterMessage("starting http server").All()
	require.Len(t, entries, 1)
	require.Equal(t, "http", entries[0].LoggerName)
	fields := entries[0].ContextMap()
	require.Equal(t, "http-server", fields[commonlogger.ComponentField])
	require.Equal(t, "127.0.0.1:0", fields["addr"])
	require.Equal(t, "aegiscore-user-service", fields["service"])
	require.Equal(t, "local", fields["environment"])
	require.Equal(t, time.Local.String(), fields["timezone"])
}

func TestDefaultHTTPShutdownTimeout(t *testing.T) {
	require.Equal(t, 10*time.Second, defaultHTTPShutdownTimeout)
}

func TestHTTPDrainTrackerWaitsForActiveHandlers(t *testing.T) {
	started := make(chan struct{})
	release := make(chan struct{})
	done := make(chan struct{})
	tracker := newHTTPDrainTracker(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		close(started)
		<-release
	}))

	go func() {
		defer close(done)
		request := httptest.NewRequest(http.MethodGet, "/", nil)
		tracker.ServeHTTP(httptest.NewRecorder(), request)
	}()

	requireEventuallyClosed(t, started, time.Second)

	waitCtx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()
	require.ErrorIs(t, tracker.Wait(waitCtx), context.DeadlineExceeded)

	close(release)
	require.NoError(t, tracker.Wait(context.Background()))
	requireEventuallyClosed(t, done, time.Second)
}

func TestHTTPDrainTrackerReturnsContextErrorWithActiveHandlers(t *testing.T) {
	tracker := newHTTPDrainTracker(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}))
	tracker.mu.Lock()
	tracker.active = 1
	tracker.mu.Unlock()
	waitCtx, cancel := context.WithCancel(context.Background())
	cancel()

	require.ErrorIs(t, tracker.Wait(waitCtx), context.Canceled)
}

func reserveHTTPTestPort(t *testing.T) int {
	t.Helper()

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	port := listener.Addr().(*net.TCPAddr).Port
	require.NoError(t, listener.Close())
	return port
}

func httpServerTestRuntimeConfig(httpCfg config.HTTPServerConfig) *config.Config {
	httpCfg.Enabled = true
	return &config.Config{Server: config.ServerConfig{HTTP: httpCfg}}
}

func requireEventuallyClosed(t *testing.T, ch <-chan struct{}, waitFor time.Duration) {
	t.Helper()
	require.Eventually(t, func() bool {
		select {
		case <-ch:
			return true
		default:
			return false
		}
	}, waitFor, 10*time.Millisecond)
}

func requireEventuallyReceives[T any](t *testing.T, ch <-chan T, waitFor time.Duration) {
	t.Helper()
	require.Eventually(t, func() bool {
		select {
		case <-ch:
			return true
		default:
			return false
		}
	}, waitFor, 10*time.Millisecond)
}
