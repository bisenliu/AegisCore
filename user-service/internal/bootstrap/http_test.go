package bootstrap

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"go.uber.org/fx"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"go.uber.org/zap/zaptest/observer"

	"github.com/aegiscore/common/runtime/config"
)

type lifecycleRecorder struct {
	hooks []fx.Hook
}

func (r *lifecycleRecorder) Append(hook fx.Hook) {
	r.hooks = append(r.hooks, hook)
}

type shutdownRecorder struct {
	calls int
	err   error
}

func (r *shutdownRecorder) Shutdown(...fx.ShutdownOption) error {
	r.calls++
	return r.err
}

func TestDefaultConfigHTTPTimeouts(t *testing.T) {
	cfg, err := config.Load("../../configs/config.yaml")
	if err != nil {
		t.Fatalf("Load default config: %v", err)
	}

	if cfg.HTTP.ReadTimeout != 30*time.Second || cfg.HTTP.WriteTimeout != 60*time.Second || cfg.HTTP.IdleTimeout != 120*time.Second || cfg.HTTP.ShutdownTimeout != 25*time.Second {
		t.Fatalf("HTTP timeouts = (%s,%s,%s,%s), want (30s,60s,120s,25s)", cfg.HTTP.ReadTimeout, cfg.HTTP.WriteTimeout, cfg.HTTP.IdleTimeout, cfg.HTTP.ShutdownTimeout)
	}
	if cfg.Auth.JWT.Secret == "" || cfg.Auth.JWT.Issuer != "aegiscore-user-services" || cfg.Auth.JWT.Audience != "aegiscore-users" {
		t.Fatalf("Auth.JWT = %#v, want default auth config", cfg.Auth.JWT)
	}
	if cfg.Auth.JWT.AccessTokenTTL != 15*time.Minute || cfg.Auth.JWT.RefreshTokenTTL != 168*time.Hour || cfg.Auth.TokenVersionCacheTTL != 30*time.Second {
		t.Fatalf("auth TTLs = (%s,%s,%s), want (15m,168h,30s)", cfg.Auth.JWT.AccessTokenTTL, cfg.Auth.JWT.RefreshTokenTTL, cfg.Auth.TokenVersionCacheTTL)
	}
	if cfg.Observability.Metrics.Enabled || cfg.Observability.Metrics.Path != "/metrics" || !cfg.Observability.Metrics.IncludeRuntime {
		t.Fatalf("observability metrics = %#v, want disabled /metrics with runtime metrics", cfg.Observability.Metrics)
	}
	if !cfg.Observability.Tracing.Enabled || cfg.Observability.Tracing.SampleRatio != 1.0 || cfg.Observability.Tracing.Exporter != "none" || cfg.Observability.Tracing.OTLPEndpoint != "" || cfg.Observability.Tracing.Insecure {
		t.Fatalf("observability tracing = %#v, want local none exporter default", cfg.Observability.Tracing)
	}
}

func TestHTTPServerUsesConfiguredTimeouts(t *testing.T) {
	lifecycle := &lifecycleRecorder{}
	cfg := &config.Config{
		HTTP: config.HTTPConfig{
			Host:            "127.0.0.1",
			Port:            18080,
			ReadTimeout:     30 * time.Second,
			WriteTimeout:    60 * time.Second,
			IdleTimeout:     120 * time.Second,
			ShutdownTimeout: 25 * time.Second,
		},
	}
	server := NewHTTPServer(HTTPServerParams{
		Lifecycle: lifecycle,
		Config:    cfg,
		Log:       zap.NewNop(),
		Engine:    gin.New(),
	})

	if server.ReadTimeout != cfg.HTTP.ReadTimeout || server.WriteTimeout != cfg.HTTP.WriteTimeout || server.IdleTimeout != cfg.HTTP.IdleTimeout {
		t.Fatalf("server timeouts = (%s,%s,%s), want (%s,%s,%s)", server.ReadTimeout, server.WriteTimeout, server.IdleTimeout, cfg.HTTP.ReadTimeout, cfg.HTTP.WriteTimeout, cfg.HTTP.IdleTimeout)
	}
	if len(lifecycle.hooks) != 1 || lifecycle.hooks[0].OnStop == nil {
		t.Fatalf("lifecycle hooks = %#v, want one shutdown hook", lifecycle.hooks)
	}
}

func TestHTTPServerStartReturnsListenError(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("reserve listener: %v", err)
	}
	defer listener.Close()

	addr := listener.Addr().(*net.TCPAddr)
	lifecycle := &lifecycleRecorder{}
	NewHTTPServer(HTTPServerParams{
		Lifecycle: lifecycle,
		Config: &config.Config{HTTP: config.HTTPConfig{
			Host: "127.0.0.1",
			Port: addr.Port,
		}},
		Log:    zap.NewNop(),
		Engine: gin.New(),
	})

	if len(lifecycle.hooks) != 1 || lifecycle.hooks[0].OnStart == nil {
		t.Fatalf("lifecycle hooks = %#v, want one start hook", lifecycle.hooks)
	}
	err = lifecycle.hooks[0].OnStart(context.Background())
	if err == nil {
		t.Fatal("OnStart error = nil, want listen error")
	}
	if !strings.Contains(err.Error(), "listen http server") {
		t.Fatalf("OnStart error = %q, want listen http server context", err.Error())
	}
}

func TestHTTPServerUnexpectedServeErrorTriggersShutdown(t *testing.T) {
	core, logs := observer.New(zapcore.ErrorLevel)
	shutdowner := &shutdownRecorder{}
	serveErr := errors.New("serve failed")

	shutdownOnHTTPServeError(zap.New(core), shutdowner, serveErr)

	if shutdowner.calls != 1 {
		t.Fatalf("shutdown calls = %d, want 1", shutdowner.calls)
	}
	entries := logs.FilterMessage("http server failed").All()
	if len(entries) != 1 {
		t.Fatalf("http server failed logs = %d, want 1", len(entries))
	}
	if loggedErr, ok := entries[0].ContextMap()["error"].(string); !ok || loggedErr != serveErr.Error() {
		t.Fatalf("logged error = %#v, want %q", entries[0].ContextMap()["error"], serveErr.Error())
	}
}

func TestHTTPServerUnexpectedServeErrorLogsShutdownFailure(t *testing.T) {
	core, logs := observer.New(zapcore.ErrorLevel)
	shutdownErr := errors.New("shutdown failed")
	shutdowner := &shutdownRecorder{err: shutdownErr}

	shutdownOnHTTPServeError(zap.New(core), shutdowner, errors.New("serve failed"))

	if shutdowner.calls != 1 {
		t.Fatalf("shutdown calls = %d, want 1", shutdowner.calls)
	}
	entries := logs.FilterMessage("shutdown after http server failure failed").All()
	if len(entries) != 1 {
		t.Fatalf("shutdown failure logs = %d, want 1", len(entries))
	}
	if loggedErr, ok := entries[0].ContextMap()["error"].(string); !ok || loggedErr != shutdownErr.Error() {
		t.Fatalf("logged shutdown error = %#v, want %q", entries[0].ContextMap()["error"], shutdownErr.Error())
	}
}

func TestHTTPServerClosedServeErrorDoesNotTriggerShutdown(t *testing.T) {
	core, logs := observer.New(zapcore.ErrorLevel)
	shutdowner := &shutdownRecorder{}

	shutdownOnHTTPServeError(zap.New(core), shutdowner, http.ErrServerClosed)
	shutdownOnHTTPServeError(zap.New(core), shutdowner, nil)

	if shutdowner.calls != 0 {
		t.Fatalf("shutdown calls = %d, want 0", shutdowner.calls)
	}
	if logs.Len() != 0 {
		t.Fatalf("error logs = %d, want 0", logs.Len())
	}
}

func TestHTTPServerLifecycleCancelStopsServeGoroutine(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}

	core, logs := observer.New(zapcore.DebugLevel)
	shutdowner := &shutdownRecorder{}
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		defer close(done)
		serveHTTPWithLifecycle(ctx, zap.New(core), shutdowner, &http.Server{}, listener)
	}()

	cancel()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("serve goroutine did not exit after lifecycle context cancellation")
	}

	if shutdowner.calls != 0 {
		t.Fatalf("shutdown calls = %d, want 0", shutdowner.calls)
	}
	if entries := logs.FilterMessage("http server failed").All(); len(entries) != 0 {
		t.Fatalf("http server failed logs = %d, want 0", len(entries))
	}
	entries := logs.FilterMessage("http server goroutine stopped").All()
	if len(entries) != 1 {
		t.Fatalf("http server goroutine stopped logs = %d, want 1", len(entries))
	}
	if reason := entries[0].ContextMap()["reason"]; reason != "lifecycle_canceled" {
		t.Fatalf("goroutine stop reason = %#v, want lifecycle_canceled", reason)
	}
}

func TestHTTPServerStartAndStop(t *testing.T) {
	lifecycle := &lifecycleRecorder{}
	NewHTTPServer(HTTPServerParams{
		Lifecycle: lifecycle,
		Config: &config.Config{HTTP: config.HTTPConfig{
			Host:            "127.0.0.1",
			Port:            0,
			ShutdownTimeout: time.Second,
		}},
		Log:    zap.NewNop(),
		Engine: gin.New(),
	})

	if len(lifecycle.hooks) != 1 || lifecycle.hooks[0].OnStart == nil || lifecycle.hooks[0].OnStop == nil {
		t.Fatalf("lifecycle hooks = %#v, want one start/stop hook", lifecycle.hooks)
	}
	if err := lifecycle.hooks[0].OnStart(context.Background()); err != nil {
		t.Fatalf("OnStart: %v", err)
	}
	stopCtx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	if err := lifecycle.hooks[0].OnStop(stopCtx); err != nil {
		t.Fatalf("OnStop: %v", err)
	}
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
		Config: &config.Config{HTTP: config.HTTPConfig{
			Host:            "127.0.0.1",
			Port:            port,
			ShutdownTimeout: time.Second,
		}},
		Log:    zap.NewNop(),
		Engine: engine,
	})

	if err := lifecycle.hooks[0].OnStart(context.Background()); err != nil {
		t.Fatalf("OnStart: %v", err)
	}
	defer func() {
		stopCtx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		_ = lifecycle.hooks[0].OnStop(stopCtx)
	}()

	responseDone := make(chan error, 1)
	go func() {
		client := &http.Client{Timeout: 2 * time.Second}
		resp, err := client.Get(fmt.Sprintf("http://127.0.0.1:%d/slow", port))
		if err != nil {
			responseDone <- err
			return
		}
		defer resp.Body.Close()
		_, err = io.ReadAll(resp.Body)
		if err != nil {
			responseDone <- err
			return
		}
		if resp.StatusCode != http.StatusOK {
			responseDone <- fmt.Errorf("status = %d, want %d", resp.StatusCode, http.StatusOK)
			return
		}
		responseDone <- nil
	}()

	select {
	case <-started:
	case <-time.After(time.Second):
		t.Fatal("request handler did not start")
	}

	stopDone := make(chan error, 1)
	go func() {
		stopCtx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		stopDone <- lifecycle.hooks[0].OnStop(stopCtx)
	}()

	select {
	case err := <-stopDone:
		t.Fatalf("OnStop returned before active request finished: %v", err)
	case <-time.After(100 * time.Millisecond):
	}

	releaseRequest()
	if err := <-stopDone; err != nil {
		t.Fatalf("OnStop: %v", err)
	}
	if err := <-responseDone; err != nil {
		t.Fatalf("GET /slow: %v", err)
	}
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
		Config: &config.Config{HTTP: config.HTTPConfig{
			Host:            "127.0.0.1",
			Port:            port,
			ShutdownTimeout: 20 * time.Millisecond,
		}},
		Log:    zap.NewNop(),
		Engine: engine,
	})

	if err := lifecycle.hooks[0].OnStart(context.Background()); err != nil {
		t.Fatalf("OnStart: %v", err)
	}
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

	select {
	case <-started:
	case <-time.After(time.Second):
		t.Fatal("request handler did not start")
	}

	stopCtx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	err := lifecycle.hooks[0].OnStop(stopCtx)
	if err == nil {
		t.Fatal("OnStop error = nil, want graceful shutdown timeout")
	}
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("OnStop error = %v, want context deadline exceeded", err)
	}

	select {
	case <-exited:
	default:
		t.Fatal("OnStop returned before blocked handler exited")
	}
	select {
	case <-responseDone:
	case <-time.After(time.Second):
		t.Fatal("client request did not finish after forced HTTP close")
	}
}

func TestHTTPServerStartLogIncludesRuntimeIdentity(t *testing.T) {
	lifecycle := &lifecycleRecorder{}
	core, logs := observer.New(zapcore.InfoLevel)
	log := zap.New(core)
	NewHTTPServer(HTTPServerParams{
		Lifecycle: lifecycle,
		Config: &config.Config{
			App:    config.AppConfig{Name: "aegiscore-user-services", Environment: "local"},
			System: config.SystemConfig{Timezone: "Asia/Shanghai"},
			HTTP: config.HTTPConfig{
				Host: "127.0.0.1",
				Port: 0,
			},
		},
		Log:    log,
		Engine: gin.New(),
	})

	if len(lifecycle.hooks) != 1 || lifecycle.hooks[0].OnStart == nil || lifecycle.hooks[0].OnStop == nil {
		t.Fatalf("lifecycle hooks = %#v, want one start/stop hook", lifecycle.hooks)
	}
	if err := lifecycle.hooks[0].OnStart(context.Background()); err != nil {
		t.Fatalf("OnStart: %v", err)
	}
	stopCtx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	if err := lifecycle.hooks[0].OnStop(stopCtx); err != nil {
		t.Fatalf("OnStop: %v", err)
	}

	entries := logs.FilterMessage("starting http server").All()
	if len(entries) != 1 {
		t.Fatalf("starting http server logs = %d, want 1", len(entries))
	}
	fields := entries[0].ContextMap()
	if fields["addr"] != "127.0.0.1:0" || fields["service"] != "aegiscore-user-services" || fields["environment"] != "local" || fields["timezone"] != "Asia/Shanghai" {
		t.Fatalf("startup log fields = %#v", fields)
	}
}

func TestDefaultHTTPShutdownTimeout(t *testing.T) {
	if defaultHTTPShutdownTimeout != 10*time.Second {
		t.Fatalf("defaultHTTPShutdownTimeout = %s, want 10s", defaultHTTPShutdownTimeout)
	}
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

	select {
	case <-started:
	case <-time.After(time.Second):
		t.Fatal("tracked handler did not start")
	}

	waitCtx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()
	if err := tracker.Wait(waitCtx); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("Wait error = %v, want context deadline exceeded", err)
	}

	close(release)
	if err := tracker.Wait(context.Background()); err != nil {
		t.Fatalf("Wait after release: %v", err)
	}
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("tracked handler did not exit")
	}
}

func TestHTTPDrainTrackerReturnsContextErrorWithActiveHandlers(t *testing.T) {
	tracker := newHTTPDrainTracker(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}))
	tracker.mu.Lock()
	tracker.active = 1
	tracker.mu.Unlock()
	waitCtx, cancel := context.WithCancel(context.Background())
	cancel()

	if err := tracker.Wait(waitCtx); !errors.Is(err, context.Canceled) {
		t.Fatalf("Wait error = %v, want context canceled", err)
	}
}

func reserveHTTPTestPort(t *testing.T) int {
	t.Helper()

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("reserve listener: %v", err)
	}
	port := listener.Addr().(*net.TCPAddr).Port
	if err := listener.Close(); err != nil {
		t.Fatalf("close reserved listener: %v", err)
	}
	return port
}
