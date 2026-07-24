package bootstrap

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"go.uber.org/zap/zaptest/observer"

	"github.com/aegiscore/common/runtime/config"
)

func TestPprofServerDisabledDoesNotRegisterLifecycle(t *testing.T) {
	lifecycle := &lifecycleRecorder{}
	server, err := NewPprofServer(PprofServerParams{
		Lifecycle: lifecycle,
		Config:    newPprofRuntimeConfig(false, config.DefaultPprofAddr),
		Log:       zap.NewNop(),
	})
	require.NoError(t, err)
	require.False(t, server.Enabled)
	require.Empty(t, lifecycle.hooks)
}

func TestPprofServerUsesParsedConfigInsteadOfProcessEnvironment(t *testing.T) {
	lifecycle := &lifecycleRecorder{}
	server, err := NewPprofServer(PprofServerParams{
		Lifecycle: lifecycle,
		Config:    newPprofRuntimeConfig(false, config.DefaultPprofAddr),
		Log:       zap.NewNop(),
	})
	require.NoError(t, err)
	require.False(t, server.Enabled)
	require.Equal(t, config.DefaultPprofAddr, server.Server.Addr)
	require.Empty(t, lifecycle.hooks)
}

func TestPprofServerUsesIndependentHandler(t *testing.T) {
	server, err := NewPprofServer(PprofServerParams{
		Lifecycle: &lifecycleRecorder{},
		Config:    newPprofRuntimeConfig(false, config.DefaultPprofAddr),
		Log:       zap.NewNop(),
	})
	require.NoError(t, err)

	pprofResponse := httptest.NewRecorder()
	server.Server.Handler.ServeHTTP(pprofResponse, httptest.NewRequest(http.MethodGet, "/debug/pprof/", nil))
	require.Equal(t, http.StatusOK, pprofResponse.Code)

	businessResponse := httptest.NewRecorder()
	server.Server.Handler.ServeHTTP(businessResponse, httptest.NewRequest(http.MethodGet, "/livez", nil))
	require.Equal(t, http.StatusNotFound, businessResponse.Code)
}

func TestPprofServerLifecycleStartsAndStopsIndependentListener(t *testing.T) {
	port := reserveHTTPTestPort(t)
	addr := fmt.Sprintf("127.0.0.1:%d", port)
	lifecycle := &lifecycleRecorder{}
	shutdowner := &shutdownRecorder{}
	server, err := NewPprofServer(PprofServerParams{
		Lifecycle:  lifecycle,
		Shutdowner: shutdowner,
		Config:     newPprofRuntimeConfig(true, addr),
		Log:        zap.NewNop(),
	})
	require.NoError(t, err)
	require.True(t, server.Enabled)
	require.Len(t, lifecycle.hooks, 1)
	require.NoError(t, lifecycle.hooks[0].OnStart(context.Background()))

	response, err := http.Get(fmt.Sprintf("http://127.0.0.1:%d/debug/pprof/", port))
	require.NoError(t, err)
	require.NoError(t, response.Body.Close())
	require.Equal(t, http.StatusOK, response.StatusCode)

	require.NoError(t, lifecycle.hooks[0].OnStop(context.Background()))
	require.Equal(t, 0, shutdowner.calls)
}

func TestPprofServerStopClosesServerAfterCanceledShutdown(t *testing.T) {
	port := reserveHTTPTestPort(t)
	addr := fmt.Sprintf("127.0.0.1:%d", port)
	lifecycle := &lifecycleRecorder{}
	shutdowner := &shutdownRecorder{}
	server, err := NewPprofServer(PprofServerParams{
		Lifecycle:  lifecycle,
		Shutdowner: shutdowner,
		Config:     newPprofRuntimeConfig(true, addr),
		Log:        zap.NewNop(),
	})
	require.NoError(t, err)
	require.Len(t, lifecycle.hooks, 1)
	entered := make(chan struct{})
	server.Server.Handler = http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		close(entered)
		<-r.Context().Done()
	})
	require.NoError(t, lifecycle.hooks[0].OnStart(context.Background()))
	requestDone := startBlockedPprofRequest(t, server.Server.Addr, entered)

	stopCtx, cancel := context.WithCancel(context.Background())
	cancel()
	stopDone := make(chan error, 1)
	go func() { stopDone <- lifecycle.hooks[0].OnStop(stopCtx) }()
	select {
	case err = <-stopDone:
	case <-time.After(time.Second):
		t.Fatal("pprof stop blocked after canceled shutdown")
	}
	require.ErrorIs(t, err, context.Canceled)
	requireEventuallyReceives(t, requestDone, time.Second)
	require.Eventually(t, func() bool {
		conn, dialErr := net.DialTimeout("tcp", server.Server.Addr, 10*time.Millisecond)
		if dialErr != nil {
			return true
		}
		require.NoError(t, conn.Close())
		return false
	}, time.Second, 10*time.Millisecond)
	require.Equal(t, 0, shutdowner.calls)
}

func TestPprofServerRepeatedStopAfterForcedCloseDoesNotBlock(t *testing.T) {
	port := reserveHTTPTestPort(t)
	addr := fmt.Sprintf("127.0.0.1:%d", port)
	lifecycle := &lifecycleRecorder{}
	server, err := NewPprofServer(PprofServerParams{
		Lifecycle:  lifecycle,
		Shutdowner: &shutdownRecorder{},
		Config:     newPprofRuntimeConfig(true, addr),
		Log:        zap.NewNop(),
	})
	require.NoError(t, err)
	require.Len(t, lifecycle.hooks, 1)
	entered := make(chan struct{})
	server.Server.Handler = http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		close(entered)
		<-r.Context().Done()
	})
	require.NoError(t, lifecycle.hooks[0].OnStart(context.Background()))
	requestDone := startBlockedPprofRequest(t, server.Server.Addr, entered)

	stopCtx, cancel := context.WithCancel(context.Background())
	cancel()
	stopDone := make(chan error, 1)
	go func() { stopDone <- lifecycle.hooks[0].OnStop(stopCtx) }()
	select {
	case err := <-stopDone:
		require.ErrorIs(t, err, context.Canceled)
	case <-time.After(time.Second):
		t.Fatal("pprof stop blocked after canceled shutdown")
	}
	requireEventuallyReceives(t, requestDone, time.Second)

	done := make(chan error, 1)
	go func() {
		done <- lifecycle.hooks[0].OnStop(context.Background())
	}()
	select {
	case err := <-done:
		require.NoError(t, err)
	case <-time.After(time.Second):
		t.Fatal("repeated pprof stop blocked")
	}

	conn, err := net.DialTimeout("tcp", server.Server.Addr, 10*time.Millisecond)
	if err == nil {
		require.NoError(t, conn.Close())
		t.Fatal("pprof server still accepts connections after repeated stop")
	}
}

func newPprofRuntimeConfig(enabled bool, addr string) *config.Config {
	cfg := config.DefaultConfig()
	cfg.App.Environment = "test"
	cfg.Observability.Pprof = config.PprofConfig{Enabled: enabled, Addr: addr}
	return &cfg
}

func startBlockedPprofRequest(t testing.TB, addr string, entered <-chan struct{}) <-chan error {
	t.Helper()
	done := make(chan error, 1)
	go func() {
		response, err := http.Get("http://" + addr + "/debug/pprof/")
		if err != nil {
			done <- err
			return
		}
		done <- response.Body.Close()
	}()
	require.Eventually(t, func() bool {
		select {
		case <-entered:
			return true
		default:
			return false
		}
	}, time.Second, 10*time.Millisecond)
	return done
}

func TestPprofServeReturnsAfterServerCloseWithoutShutdownSignal(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	server := &http.Server{Handler: http.HandlerFunc(func(http.ResponseWriter, *http.Request) {})}
	shutdowner := &shutdownRecorder{}
	done := make(chan struct{})

	go func() {
		servePprofServer(zap.NewNop(), shutdowner, server, listener)
		close(done)
	}()

	require.NoError(t, server.Close())
	requireEventuallyClosed(t, done, time.Second)
	require.Equal(t, 0, shutdowner.calls)
}

func TestPprofServerUnexpectedListenerCloseTriggersShutdown(t *testing.T) {
	core, logs := observer.New(zapcore.ErrorLevel)
	shutdowner, signals := newShutdownSignalRecorder(t)

	handlePprofServeExit(zap.New(core), shutdowner, net.ErrClosed)

	require.Equal(t, 1, shutdowner.calls)
	require.Equal(t, 1, requireShutdownSignal(t, signals).ExitCode)
	entries := logs.FilterMessage("pprof server failed").All()
	require.Len(t, entries, 1)
	require.Equal(t, net.ErrClosed.Error(), entries[0].ContextMap()["error"])
}

func TestPprofServerExpectedServeExitDoesNotTriggerShutdown(t *testing.T) {
	core, logs := observer.New(zapcore.ErrorLevel)
	shutdowner := &shutdownRecorder{}

	handlePprofServeExit(zap.New(core), shutdowner, nil)
	handlePprofServeExit(zap.New(core), shutdowner, http.ErrServerClosed)

	require.Equal(t, 0, shutdowner.calls)
	require.Equal(t, 0, logs.Len())
}

func TestPprofServerUnexpectedServeErrorLogsShutdownFailure(t *testing.T) {
	core, logs := observer.New(zapcore.ErrorLevel)
	shutdownErr := errors.New("shutdown failed")
	shutdowner := &shutdownRecorder{err: shutdownErr}

	handlePprofServeExit(zap.New(core), shutdowner, errors.New("serve failed"))

	require.Equal(t, 1, shutdowner.calls)
	entries := logs.FilterMessage("shutdown after pprof server failure failed").All()
	require.Len(t, entries, 1)
	require.Equal(t, shutdownErr.Error(), entries[0].ContextMap()["error"])
}
