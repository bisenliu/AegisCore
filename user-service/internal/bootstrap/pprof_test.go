package bootstrap

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"

	"github.com/aegiscore/common/runtime/config"
)

func TestPprofDisabledDoesNotConstructManagedRegisterLifecycleOrListen(t *testing.T) {
	t.Parallel()

	port := reserveHTTPTestPort(t)
	addr := fmt.Sprintf("127.0.0.1:%d", port)
	lifecycle := &lifecycleRecorder{}
	runtime, err := NewPprofServer(PprofServerParams{
		Lifecycle: lifecycle,
		Config:    newPprofRuntimeConfig(false, addr),
		Log:       zap.NewNop(),
	})
	require.NoError(t, err)
	require.False(t, runtime.Enabled)
	require.Nil(t, runtime.Managed)
	require.Empty(t, lifecycle.hooks)

	listener, err := net.Listen("tcp", addr)
	require.NoError(t, err)
	require.NoError(t, listener.Close())
}

func TestPprofRuntimeUsesConfiguredIndependentHandlerAndAddress(t *testing.T) {
	t.Parallel()

	addr := "127.0.0.1:16060"
	runtime, err := NewPprofServer(PprofServerParams{
		Lifecycle: &lifecycleRecorder{},
		Config:    newPprofRuntimeConfig(true, addr),
		Log:       zap.NewNop(),
	})
	require.NoError(t, err)
	require.True(t, runtime.Enabled)
	require.NotNil(t, runtime.Managed)
	require.Equal(t, addr, runtime.Managed.HTTPServer().Addr)

	pprofResponse := httptest.NewRecorder()
	runtime.Managed.HTTPServer().Handler.ServeHTTP(
		pprofResponse,
		httptest.NewRequest(http.MethodGet, "/debug/pprof/", nil),
	)
	require.Equal(t, http.StatusOK, pprofResponse.Code)
	businessResponse := httptest.NewRecorder()
	runtime.Managed.HTTPServer().Handler.ServeHTTP(
		businessResponse,
		httptest.NewRequest(http.MethodGet, "/livez", nil),
	)
	require.Equal(t, http.StatusNotFound, businessResponse.Code)
}

func TestPprofLifecycleStartsAndStopsIndependentListener(t *testing.T) {
	t.Parallel()

	port := reserveHTTPTestPort(t)
	addr := fmt.Sprintf("127.0.0.1:%d", port)
	lifecycle := &lifecycleRecorder{}
	shutdowner := &shutdownRecorder{}
	runtime, err := NewPprofServer(PprofServerParams{
		Lifecycle:  lifecycle,
		Shutdowner: shutdowner,
		Config:     newPprofRuntimeConfig(true, addr),
		Log:        zap.NewNop(),
	})
	require.NoError(t, err)
	require.True(t, runtime.Enabled)
	require.Len(t, lifecycle.hooks, 1)
	require.NoError(t, lifecycle.hooks[0].OnStart(context.Background()))

	response, err := http.Get("http://" + addr + "/debug/pprof/")
	require.NoError(t, err)
	require.NoError(t, response.Body.Close())
	require.Equal(t, http.StatusOK, response.StatusCode)
	require.NoError(t, lifecycle.hooks[0].OnStop(context.Background()))
	require.NoError(t, lifecycle.hooks[0].OnStop(context.Background()))
	require.Equal(t, int32(0), shutdowner.Calls())
}

func TestPprofOnStartReturnsListenError(t *testing.T) {
	t.Parallel()

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, listener.Close()) })
	lifecycle := &lifecycleRecorder{}
	_, err = NewPprofServer(PprofServerParams{
		Lifecycle: lifecycle,
		Config:    newPprofRuntimeConfig(true, listener.Addr().String()),
		Log:       zap.NewNop(),
	})
	require.NoError(t, err)

	err = lifecycle.hooks[0].OnStart(context.Background())
	require.ErrorContains(t, err, "listen http server")
	require.ErrorContains(t, err, listener.Addr().String())
}

func TestBusinessAndPprofUseIndependentManagedInstances(t *testing.T) {
	t.Parallel()

	cfg := config.DefaultConfig()
	cfg.Server.HTTP.Host = "127.0.0.1"
	cfg.Server.HTTP.Port = 0
	cfg.Observability.Pprof.Enabled = true
	cfg.Observability.Pprof.Addr = "127.0.0.1:0"
	lifecycle := &lifecycleRecorder{}
	httpRuntime, err := NewHTTPServer(HTTPServerParams{
		Lifecycle: lifecycle,
		Config:    &cfg,
		Log:       zap.NewNop(),
		Engine:    gin.New(),
	})
	require.NoError(t, err)
	pprofRuntime, err := NewPprofServer(PprofServerParams{
		Lifecycle: lifecycle,
		Config:    &cfg,
		Log:       zap.NewNop(),
	})
	require.NoError(t, err)

	require.NotNil(t, httpRuntime.Managed)
	require.NotNil(t, pprofRuntime.Managed)
	require.NotSame(t, httpRuntime.Managed, pprofRuntime.Managed)
	require.NotSame(t, httpRuntime.Managed.HTTPServer(), pprofRuntime.Managed.HTTPServer())
	require.Len(t, lifecycle.hooks, 2)
}

func TestPprofUsesHTTPShutdownBudgetCoveredByFxStopBudget(t *testing.T) {
	t.Parallel()

	cfg := newPprofRuntimeConfig(true, "127.0.0.1:0")
	require.Positive(t, cfg.Server.HTTP.ShutdownTimeout)
	require.GreaterOrEqual(t, cfg.Runtime.Lifecycle.StopTimeout, cfg.Server.HTTP.ShutdownTimeout)
}

func newPprofRuntimeConfig(enabled bool, addr string) *config.Config {
	cfg := config.DefaultConfig()
	cfg.App.Environment = "test"
	cfg.Observability.Pprof = config.PprofConfig{Enabled: enabled, Addr: addr}
	return &cfg
}

func TestPprofStopCallerCancellationCanBeRetried(t *testing.T) {
	t.Parallel()

	lifecycle := &lifecycleRecorder{}
	runtime, err := NewPprofServer(PprofServerParams{
		Lifecycle: lifecycle,
		Config:    newPprofRuntimeConfig(true, "127.0.0.1:0"),
		Log:       zap.NewNop(),
	})
	require.NoError(t, err)
	require.NoError(t, lifecycle.hooks[0].OnStart(context.Background()))

	canceled, cancel := context.WithCancel(context.Background())
	cancel()
	firstErr := lifecycle.hooks[0].OnStop(canceled)
	if firstErr != nil {
		require.ErrorIs(t, firstErr, context.Canceled)
	}
	require.NoError(t, lifecycle.hooks[0].OnStop(context.Background()))
	require.NoError(t, runtime.Managed.Stop(context.Background()))
}

func TestPprofRuntimeRejectsMissingShutdownTimeout(t *testing.T) {
	t.Parallel()

	cfg := newPprofRuntimeConfig(true, "127.0.0.1:0")
	cfg.Server.HTTP.ShutdownTimeout = 0
	_, err := NewPprofServer(PprofServerParams{
		Lifecycle: &lifecycleRecorder{},
		Config:    cfg,
		Log:       zap.NewNop(),
	})
	require.ErrorContains(t, err, "shutdown timeout must be positive")
}

func TestPprofAddressRemainsAvailableAfterStop(t *testing.T) {
	t.Parallel()

	port := reserveHTTPTestPort(t)
	addr := fmt.Sprintf("127.0.0.1:%d", port)
	lifecycle := &lifecycleRecorder{}
	_, err := NewPprofServer(PprofServerParams{
		Lifecycle: lifecycle,
		Config:    newPprofRuntimeConfig(true, addr),
		Log:       zap.NewNop(),
	})
	require.NoError(t, err)
	require.NoError(t, lifecycle.hooks[0].OnStart(context.Background()))
	require.NoError(t, lifecycle.hooks[0].OnStop(context.Background()))

	require.Eventually(t, func() bool {
		listener, listenErr := net.Listen("tcp", addr)
		if listenErr != nil {
			return false
		}
		require.NoError(t, listener.Close())
		return true
	}, time.Second, 10*time.Millisecond)
}
