package bootstrap

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"go.uber.org/zap/zaptest/observer"

	"github.com/aegiscore/common/runtime/config"
)

func TestLoadPprofSettingsDefaultsToDisabledLoopback(t *testing.T) {
	settings, err := loadPprofSettings(func(string) (string, bool) { return "", false })
	require.NoError(t, err)
	require.False(t, settings.enabled)
	require.Equal(t, defaultPprofAddr, settings.addr)
}

func TestLoadPprofSettingsUsesExplicitEnvironment(t *testing.T) {
	values := map[string]string{
		pprofEnabledEnv: "true",
		pprofAddrEnv:    "localhost:16060",
	}
	settings, err := loadPprofSettings(func(key string) (string, bool) {
		value, ok := values[key]
		return value, ok
	})
	require.NoError(t, err)
	require.True(t, settings.enabled)
	require.Equal(t, "localhost:16060", settings.addr)
}

func TestLoadPprofSettingsRejectsInvalidBool(t *testing.T) {
	_, err := loadPprofSettings(func(key string) (string, bool) {
		if key == pprofEnabledEnv {
			return "sometimes", true
		}
		return "", false
	})
	require.ErrorContains(t, err, pprofEnabledEnv)
}

func TestPprofServerDisabledDoesNotRegisterLifecycle(t *testing.T) {
	t.Setenv(pprofEnabledEnv, "false")
	t.Setenv(pprofAddrEnv, defaultPprofAddr)
	lifecycle := &lifecycleRecorder{}
	server, err := NewPprofServer(PprofServerParams{
		Lifecycle: lifecycle,
		Config:    &config.Config{App: config.AppConfig{Environment: "production"}},
		Log:       zap.NewNop(),
	})
	require.NoError(t, err)
	require.False(t, server.Enabled)
	require.Empty(t, lifecycle.hooks)
}

func TestPprofServerRejectsNonLoopbackProductionAddresses(t *testing.T) {
	tests := []string{
		"0.0.0.0:6060",
		"[::]:6060",
		"192.0.2.10:6060",
		"diagnostics.internal:6060",
	}
	for _, addr := range tests {
		t.Run(addr, func(t *testing.T) {
			t.Setenv(pprofEnabledEnv, "true")
			t.Setenv(pprofAddrEnv, addr)
			_, err := NewPprofServer(PprofServerParams{
				Lifecycle: &lifecycleRecorder{},
				Config:    &config.Config{App: config.AppConfig{Environment: "staging"}},
				Log:       zap.NewNop(),
			})
			require.ErrorContains(t, err, "loopback")
		})
	}
}

func TestPprofServerAllowsLoopbackProductionAddresses(t *testing.T) {
	for _, addr := range []string{"127.0.0.1:6060", "[::1]:6060", "localhost:6060"} {
		t.Run(addr, func(t *testing.T) {
			t.Setenv(pprofEnabledEnv, "true")
			t.Setenv(pprofAddrEnv, addr)
			server, err := NewPprofServer(PprofServerParams{
				Lifecycle: &lifecycleRecorder{},
				Config:    &config.Config{App: config.AppConfig{Environment: "production"}},
				Log:       zap.NewNop(),
			})
			require.NoError(t, err)
			require.True(t, server.Enabled)
		})
	}
}

func TestPprofServerUsesIndependentHandler(t *testing.T) {
	t.Setenv(pprofEnabledEnv, "false")
	t.Setenv(pprofAddrEnv, defaultPprofAddr)
	server, err := NewPprofServer(PprofServerParams{
		Lifecycle: &lifecycleRecorder{},
		Config:    &config.Config{App: config.AppConfig{Environment: "test"}},
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
	t.Setenv(pprofEnabledEnv, "true")
	t.Setenv(pprofAddrEnv, fmt.Sprintf("127.0.0.1:%d", port))
	lifecycle := &lifecycleRecorder{}
	shutdowner := &shutdownRecorder{}
	server, err := NewPprofServer(PprofServerParams{
		Lifecycle:  lifecycle,
		Shutdowner: shutdowner,
		Config:     &config.Config{App: config.AppConfig{Environment: "test"}},
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
