package bootstrap

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"sync"
	"sync/atomic"
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
	calls    atomic.Int32
	err      error
	delegate fx.Shutdowner
}

func (r *shutdownRecorder) Shutdown(options ...fx.ShutdownOption) error {
	r.calls.Add(1)
	if r.err != nil {
		return r.err
	}
	if r.delegate != nil {
		return r.delegate.Shutdown(options...)
	}
	return nil
}

func (r *shutdownRecorder) Calls() int32 {
	return r.calls.Load()
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
	t.Parallel()

	cfg := config.DefaultConfig()
	require.True(t, cfg.Server.HTTP.Enabled)
	require.Equal(t, 30*time.Second, cfg.Server.HTTP.ReadTimeout)
	require.Equal(t, 60*time.Second, cfg.Server.HTTP.WriteTimeout)
	require.Equal(t, 120*time.Second, cfg.Server.HTTP.IdleTimeout)
	require.Equal(t, 10*time.Second, cfg.Server.HTTP.ShutdownTimeout)
	require.GreaterOrEqual(t, cfg.Runtime.Lifecycle.StopTimeout, cfg.Server.HTTP.ShutdownTimeout)
}

func TestHTTPRuntimeUsesConfiguredOptions(t *testing.T) {
	t.Parallel()

	lifecycle := &lifecycleRecorder{}
	cfg := httpServerTestRuntimeConfig(config.HTTPServerConfig{
		Enabled:         true,
		Host:            "127.0.0.1",
		Port:            18080,
		ReadTimeout:     30 * time.Second,
		WriteTimeout:    60 * time.Second,
		IdleTimeout:     120 * time.Second,
		ShutdownTimeout: 25 * time.Second,
	})
	runtime, err := NewHTTPServer(HTTPServerParams{
		Lifecycle: lifecycle,
		Config:    cfg,
		Log:       zap.NewNop(),
		Engine:    gin.New(),
	})
	require.NoError(t, err)
	require.True(t, runtime.Enabled)
	require.NotNil(t, runtime.Managed)

	server := runtime.Managed.HTTPServer()
	require.Equal(t, "127.0.0.1:18080", server.Addr)
	require.Equal(t, cfg.Server.HTTP.ReadTimeout, server.ReadTimeout)
	require.Equal(t, cfg.Server.HTTP.WriteTimeout, server.WriteTimeout)
	require.Equal(t, cfg.Server.HTTP.IdleTimeout, server.IdleTimeout)
	require.Len(t, lifecycle.hooks, 1)
	require.NotNil(t, lifecycle.hooks[0].OnStart)
	require.NotNil(t, lifecycle.hooks[0].OnStop)
}

func TestHTTPRuntimeRejectsInvalidManagedOptions(t *testing.T) {
	t.Parallel()

	_, err := NewHTTPServer(HTTPServerParams{
		Lifecycle: &lifecycleRecorder{},
		Config: httpServerTestRuntimeConfig(config.HTTPServerConfig{
			Enabled: true,
			Host:    "127.0.0.1",
			Port:    8080,
		}),
		Log:    zap.NewNop(),
		Engine: gin.New(),
	})
	require.ErrorContains(t, err, "shutdown timeout must be positive")
}

func TestHTTPFxOnStartReturnsListenError(t *testing.T) {
	occupied, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, occupied.Close()) })
	port := occupied.Addr().(*net.TCPAddr).Port
	cfg := httpServerTestRuntimeConfig(config.HTTPServerConfig{
		Enabled:         true,
		Host:            "127.0.0.1",
		Port:            port,
		ShutdownTimeout: time.Second,
	})
	app := fx.New(
		fxtest.WithTestLogger(t),
		fx.Supply(cfg, zap.NewNop(), gin.New()),
		fx.Provide(NewHTTPServer),
		fx.Invoke(func(*HTTPRuntime) {}),
	)
	require.NoError(t, app.Err())

	err = app.Start(context.Background())
	require.ErrorContains(t, err, "listen http server")
	require.ErrorContains(t, err, occupied.Addr().String())
}

func TestHTTPDisabledDoesNotConstructManagedOrRegisterLifecycle(t *testing.T) {
	t.Parallel()

	port := reserveHTTPTestPort(t)
	lifecycle := &lifecycleRecorder{}
	runtime, err := NewHTTPServer(HTTPServerParams{
		Lifecycle: lifecycle,
		Config: &config.Config{Server: config.ServerConfig{
			HTTP: config.HTTPServerConfig{Enabled: false, Host: "127.0.0.1", Port: port},
			GRPC: config.GRPCServerConfig{Enabled: true},
		}},
		Log:    zap.NewNop(),
		Engine: gin.New(),
	})
	require.NoError(t, err)
	require.False(t, runtime.Enabled)
	require.Nil(t, runtime.Managed)
	require.Empty(t, lifecycle.hooks)

	listener, err := net.Listen("tcp", fmt.Sprintf("127.0.0.1:%d", port))
	require.NoError(t, err)
	require.NoError(t, listener.Close())
}

func TestHTTPDisabledAllowsFxAppStartAndStop(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{Server: config.ServerConfig{
		HTTP: config.HTTPServerConfig{Enabled: false},
		GRPC: config.GRPCServerConfig{Enabled: true},
	}}
	app := fx.New(
		fx.Supply(cfg, zap.NewNop(), gin.New()),
		fx.Provide(NewHTTPServer),
		fx.Invoke(func(runtime *HTTPRuntime) {
			require.False(t, runtime.Enabled)
			require.Nil(t, runtime.Managed)
		}),
	)
	require.NoError(t, app.Err())
	require.NoError(t, app.Start(context.Background()))
	require.NoError(t, app.Stop(context.Background()))
}

func TestRuntimeServerFailureHandlerTriggersExitCodeOne(t *testing.T) {
	t.Parallel()

	for _, serverName := range []string{"http", "pprof"} {
		serverName := serverName
		t.Run(serverName, func(t *testing.T) {
			core, logs := observer.New(zapcore.ErrorLevel)
			shutdowner, signals := newShutdownSignalRecorder(t)
			serveErr := errors.New("serve failed")

			newRuntimeServerFailureHandler(zap.New(core), shutdowner, serverName)(serveErr)

			require.Equal(t, int32(1), shutdowner.Calls())
			require.Equal(t, 1, requireShutdownSignal(t, signals).ExitCode)
			entries := logs.FilterMessage(serverName + " server failed").All()
			require.Len(t, entries, 1)
			require.Equal(t, serveErr.Error(), entries[0].ContextMap()["error"])
		})
	}
}

func TestRuntimeServerFailureHandlerLogsShutdownFailure(t *testing.T) {
	t.Parallel()

	core, logs := observer.New(zapcore.ErrorLevel)
	shutdownErr := errors.New("shutdown failed")
	shutdowner := &shutdownRecorder{err: shutdownErr}

	newRuntimeServerFailureHandler(zap.New(core), shutdowner, "http")(errors.New("serve failed"))

	require.Equal(t, int32(1), shutdowner.Calls())
	entries := logs.FilterMessage("shutdown after http server failure failed").All()
	require.Len(t, entries, 1)
	require.Equal(t, shutdownErr.Error(), entries[0].ContextMap()["error"])
}

func TestHTTPRuntimeNormalStartAndStopDoesNotTriggerShutdown(t *testing.T) {
	t.Parallel()

	lifecycle := &lifecycleRecorder{}
	shutdowner := &shutdownRecorder{}
	runtime, err := NewHTTPServer(HTTPServerParams{
		Lifecycle:  lifecycle,
		Shutdowner: shutdowner,
		Config: httpServerTestRuntimeConfig(config.HTTPServerConfig{
			Enabled:         true,
			Host:            "127.0.0.1",
			Port:            0,
			ShutdownTimeout: time.Second,
		}),
		Log:    zap.NewNop(),
		Engine: gin.New(),
	})
	require.NoError(t, err)
	require.NotNil(t, runtime.Managed)
	require.NoError(t, lifecycle.hooks[0].OnStart(context.Background()))
	require.NoError(t, lifecycle.hooks[0].OnStop(context.Background()))
	require.Equal(t, int32(0), shutdowner.Calls())
}

func TestHTTPRuntimeStopWaitsForActiveRequest(t *testing.T) {
	t.Parallel()

	port := reserveHTTPTestPort(t)
	lifecycle := &lifecycleRecorder{}
	started := make(chan struct{})
	release := make(chan struct{})
	var releaseOnce sync.Once
	t.Cleanup(func() { releaseOnce.Do(func() { close(release) }) })
	engine := gin.New()
	engine.GET("/slow", func(ctx *gin.Context) {
		close(started)
		<-release
		ctx.String(http.StatusOK, "ok")
	})
	_, err := NewHTTPServer(HTTPServerParams{
		Lifecycle: lifecycle,
		Config: httpServerTestRuntimeConfig(config.HTTPServerConfig{
			Enabled:         true,
			Host:            "127.0.0.1",
			Port:            port,
			ShutdownTimeout: time.Second,
		}),
		Log:    zap.NewNop(),
		Engine: engine,
	})
	require.NoError(t, err)
	require.NoError(t, lifecycle.hooks[0].OnStart(context.Background()))

	responseDone := startBootstrapHTTPRequest(fmt.Sprintf("http://127.0.0.1:%d/slow", port))
	requireEventuallyClosed(t, started, time.Second)
	stopDone := make(chan error, 1)
	go func() { stopDone <- lifecycle.hooks[0].OnStop(context.Background()) }()
	require.Never(t, func() bool {
		select {
		case <-stopDone:
			return true
		default:
			return false
		}
	}, 50*time.Millisecond, 5*time.Millisecond)
	releaseOnce.Do(func() { close(release) })
	require.NoError(t, <-stopDone)
	require.NoError(t, <-responseDone)
}

func TestHTTPRuntimeForcesCloseAfterShutdownTimeout(t *testing.T) {
	t.Parallel()

	port := reserveHTTPTestPort(t)
	lifecycle := &lifecycleRecorder{}
	started := make(chan struct{})
	exited := make(chan struct{})
	engine := gin.New()
	engine.GET("/blocked", func(ctx *gin.Context) {
		close(started)
		<-ctx.Request.Context().Done()
		close(exited)
	})
	_, err := NewHTTPServer(HTTPServerParams{
		Lifecycle: lifecycle,
		Config: httpServerTestRuntimeConfig(config.HTTPServerConfig{
			Enabled:         true,
			Host:            "127.0.0.1",
			Port:            port,
			ShutdownTimeout: 30 * time.Millisecond,
		}),
		Log:    zap.NewNop(),
		Engine: engine,
	})
	require.NoError(t, err)
	require.NoError(t, lifecycle.hooks[0].OnStart(context.Background()))
	responseDone := startBootstrapHTTPRequest(fmt.Sprintf("http://127.0.0.1:%d/blocked", port))
	requireEventuallyClosed(t, started, time.Second)

	err = lifecycle.hooks[0].OnStop(context.Background())
	require.ErrorIs(t, err, context.DeadlineExceeded)
	requireEventuallyClosed(t, exited, time.Second)
	requireEventuallyReceives(t, responseDone, time.Second)
}

func TestHTTPRuntimeStartLogIncludesRuntimeIdentity(t *testing.T) {
	t.Parallel()

	lifecycle := &lifecycleRecorder{}
	core, logs := observer.New(zapcore.InfoLevel)
	cfg := httpServerTestRuntimeConfig(config.HTTPServerConfig{
		Enabled:         true,
		Host:            "127.0.0.1",
		Port:            0,
		ShutdownTimeout: time.Second,
	})
	cfg.App = config.AppConfig{Name: "aegiscore-user-service", Environment: "local"}
	_, err := NewHTTPServer(HTTPServerParams{
		Lifecycle: lifecycle,
		Config:    cfg,
		Log:       zap.New(core),
		Engine:    gin.New(),
	})
	require.NoError(t, err)
	require.NoError(t, lifecycle.hooks[0].OnStart(context.Background()))
	require.NoError(t, lifecycle.hooks[0].OnStop(context.Background()))

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

func reserveHTTPTestPort(t testing.TB) int {
	t.Helper()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	port := listener.Addr().(*net.TCPAddr).Port
	require.NoError(t, listener.Close())
	return port
}

func httpServerTestRuntimeConfig(httpCfg config.HTTPServerConfig) *config.Config {
	return &config.Config{Server: config.ServerConfig{HTTP: httpCfg}}
}

func startBootstrapHTTPRequest(url string) <-chan error {
	done := make(chan error, 1)
	go func() {
		client := &http.Client{Timeout: 2 * time.Second}
		response, err := client.Get(url)
		if err != nil {
			done <- err
			return
		}
		defer response.Body.Close()
		_, err = io.Copy(io.Discard, response.Body)
		done <- err
	}()
	return done
}

func requireEventuallyClosed(t testing.TB, channel <-chan struct{}, waitFor time.Duration) {
	t.Helper()
	require.Eventually(t, func() bool {
		select {
		case <-channel:
			return true
		default:
			return false
		}
	}, waitFor, 10*time.Millisecond)
}

func requireEventuallyReceives[T any](t testing.TB, channel <-chan T, waitFor time.Duration) {
	t.Helper()
	require.Eventually(t, func() bool {
		select {
		case <-channel:
			return true
		default:
			return false
		}
	}, waitFor, 10*time.Millisecond)
}
