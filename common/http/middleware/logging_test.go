package middleware

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"go.opentelemetry.io/otel/trace"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"go.uber.org/zap/zaptest/observer"

	"github.com/aegiscore/common/runtime/logger"
	"github.com/aegiscore/common/security/auth"
)

func TestContextLoggerUsesOTelTraceID(t *testing.T) {
	core, logs := observer.New(zap.InfoLevel)
	log := zap.New(core)
	ctx := logger.ToContext(contextWithTraceID(t, context.Background(), "00112233445566778899aabbccddeeff"), log)

	logger.Info(ctx, "context logger used")

	entries := logs.FilterMessage("context logger used").All()
	if len(entries) != 1 {
		t.Fatalf("context log count = %d, want 1", len(entries))
	}
	if got := entries[0].ContextMap()[logger.TraceIDField]; got != "00112233445566778899aabbccddeeff" {
		t.Fatalf("%s = %q, want OTel trace id", logger.TraceIDField, got)
	}
}

func TestCORSWithOptions(t *testing.T) {
	gin.SetMode(gin.TestMode)
	engine := gin.New()
	engine.Use(CORSWithOptions(CORSOptions{
		AllowedMethods:   []string{http.MethodGet},
		AllowedHeaders:   []string{"Content-Type"},
		AllowCredentials: true,
		MaxAgeSeconds:    600,
		ReflectOrigin:    true,
	}))
	engine.GET("/cors", func(c *gin.Context) { c.Status(http.StatusNoContent) })

	req := httptest.NewRequest(http.MethodGet, "/cors", nil)
	req.Header.Set(HeaderOrigin, "https://example.test")
	rec := httptest.NewRecorder()
	engine.ServeHTTP(rec, req)

	if got := rec.Header().Get(HeaderAccessControlAllowOrigin); got != "https://example.test" {
		t.Fatalf("allow origin = %q", got)
	}
	if got := rec.Header().Get(HeaderVary); got != HeaderOrigin {
		t.Fatalf("vary = %q", got)
	}
	if got := rec.Header().Get(HeaderAccessControlExposeHeaders); got != "" {
		t.Fatalf("expose headers = %q, want empty", got)
	}
	if got := rec.Header().Get(HeaderAccessControlAllowCredentials); got != "true" {
		t.Fatalf("allow credentials = %q", got)
	}
	if got := rec.Header().Get(HeaderAccessControlMaxAge); got != "600" {
		t.Fatalf("max age = %q", got)
	}
}

func TestRequestLoggerIncludesTraceIDAndRequestFields(t *testing.T) {
	gin.SetMode(gin.TestMode)
	core, logs := observer.New(zap.InfoLevel)
	log := zap.New(core)
	engine := gin.New()
	engine.Use(otelTraceMiddleware(t, "00112233445566778899aabbccddeeff"), RequestLogger(log))
	engine.GET("/ok", func(c *gin.Context) { c.Status(http.StatusAccepted) })

	req := httptest.NewRequest(http.MethodGet, "/ok", nil)
	engine.ServeHTTP(httptest.NewRecorder(), req)

	entries := logs.FilterMessage("http request completed").All()
	if len(entries) != 1 {
		t.Fatalf("request log count = %d, want 1", len(entries))
	}
	fields := entries[0].ContextMap()
	if fields[logger.TraceIDField] != "00112233445566778899aabbccddeeff" || fields["method"] != http.MethodGet || fields["path"] != "/ok" || fields["status"] != int64(http.StatusAccepted) || fields[auth.UserIDKey] != anonymousUserID {
		t.Fatalf("request log fields = %#v", fields)
	}
	if _, ok := fields["latency_ms"]; !ok {
		t.Fatalf("request log missing latency_ms: %#v", fields)
	}
	if _, ok := fields["client_ip"]; !ok {
		t.Fatalf("request log missing client_ip: %#v", fields)
	}
}

func TestRequestLoggerUsesSharedClientIPExtraction(t *testing.T) {
	gin.SetMode(gin.TestMode)
	core, logs := observer.New(zap.InfoLevel)
	log := zap.New(core)
	engine := gin.New()
	engine.Use(RequestLogger(log))
	engine.GET("/ok", func(c *gin.Context) { c.Status(http.StatusAccepted) })

	req := httptest.NewRequest(http.MethodGet, "/ok", nil)
	req.Header.Set("X-Forwarded-For", "203.0.113.10, 10.0.0.1")
	engine.ServeHTTP(httptest.NewRecorder(), req)

	entries := logs.FilterMessage("http request completed").All()
	if len(entries) != 1 {
		t.Fatalf("request log count = %d, want 1", len(entries))
	}
	fields := entries[0].ContextMap()
	if fields["client_ip"] != "203.0.113.10" {
		t.Fatalf("client_ip = %q, want 203.0.113.10; fields = %#v", fields["client_ip"], fields)
	}
}

func TestRequestLoggerSelectsLevelByStatus(t *testing.T) {
	gin.SetMode(gin.TestMode)
	tests := []struct {
		name   string
		status int
		level  zapcore.Level
	}{
		{name: "success", status: http.StatusNoContent, level: zap.InfoLevel},
		{name: "redirect", status: http.StatusFound, level: zap.InfoLevel},
		{name: "client error", status: http.StatusNotFound, level: zap.WarnLevel},
		{name: "server error", status: http.StatusInternalServerError, level: zap.ErrorLevel},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			core, logs := observer.New(zap.DebugLevel)
			log := zap.New(core)
			engine := gin.New()
			engine.Use(RequestLogger(log))
			engine.GET("/status", func(c *gin.Context) { c.Status(tt.status) })

			req := httptest.NewRequest(http.MethodGet, "/status", nil)
			engine.ServeHTTP(httptest.NewRecorder(), req)

			entries := logs.FilterMessage("http request completed").All()
			if len(entries) != 1 {
				t.Fatalf("request log count = %d, want 1", len(entries))
			}
			if entries[0].Level != tt.level {
				t.Fatalf("level = %s, want %s", entries[0].Level, tt.level)
			}
		})
	}
}

func TestRequestLoggerIncludesUserIDFromRequestContext(t *testing.T) {
	gin.SetMode(gin.TestMode)
	core, logs := observer.New(zap.InfoLevel)
	log := zap.New(core)
	engine := gin.New()
	engine.Use(RequestLogger(log))
	engine.GET("/me", func(c *gin.Context) {
		c.Request = c.Request.WithContext(auth.WithUserID(c.Request.Context(), "u-123"))
		c.Status(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodGet, "/me", nil)
	engine.ServeHTTP(httptest.NewRecorder(), req)

	entries := logs.FilterMessage("http request completed").All()
	if len(entries) != 1 {
		t.Fatalf("request log count = %d, want 1", len(entries))
	}
	fields := entries[0].ContextMap()
	if fields[auth.UserIDKey] != "u-123" {
		t.Fatalf("user_id = %q, want u-123; fields = %#v", fields[auth.UserIDKey], fields)
	}
}

func TestRequestLoggerWithOptionsSkipsMatchingRequest(t *testing.T) {
	gin.SetMode(gin.TestMode)
	core, logs := observer.New(zap.InfoLevel)
	log := zap.New(core)
	engine := gin.New()
	engine.Use(RequestLoggerWithOptions(log, RequestLoggerOptions{
		Skip: func(c *gin.Context) bool {
			return c.FullPath() == "/skip"
		},
	}))
	engine.GET("/skip", func(c *gin.Context) { c.Status(http.StatusOK) })
	engine.GET("/keep", func(c *gin.Context) { c.Status(http.StatusOK) })

	engine.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/skip", nil))
	engine.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/keep", nil))

	entries := logs.FilterMessage("http request completed").All()
	if len(entries) != 1 {
		t.Fatalf("request log count = %d, want 1", len(entries))
	}
	if got := entries[0].ContextMap()["path"]; got != "/keep" {
		t.Fatalf("logged path = %q, want /keep", got)
	}
}

func TestRecoveryIncludesTraceIDAndEnvelope(t *testing.T) {
	gin.SetMode(gin.TestMode)
	core, logs := observer.New(zap.ErrorLevel)
	log := zap.New(core)
	engine := gin.New()
	engine.Use(otelTraceMiddleware(t, "00112233445566778899aabbccddeeff"), Recovery(log))
	engine.GET("/panic", func(_ *gin.Context) { panic("boom") })

	req := httptest.NewRequest(http.MethodGet, "/panic", nil)
	rec := httptest.NewRecorder()
	engine.ServeHTTP(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want 500", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), `"success":false`) {
		t.Fatalf("response body = %s, want failure envelope", rec.Body.String())
	}
	entries := logs.FilterMessage("panic recovered").All()
	if len(entries) != 1 {
		t.Fatalf("recovery log count = %d, want 1", len(entries))
	}
	fields := entries[0].ContextMap()
	if fields[logger.TraceIDField] != "00112233445566778899aabbccddeeff" || fields["panic"] != "boom" {
		t.Fatalf("recovery log fields = %#v", fields)
	}
	if _, ok := fields["stack"]; !ok {
		t.Fatalf("recovery log missing stack: %#v", fields)
	}
}

func otelTraceMiddleware(t *testing.T, traceIDHex string) gin.HandlerFunc {
	t.Helper()
	return func(c *gin.Context) {
		c.Request = c.Request.WithContext(contextWithTraceID(t, c.Request.Context(), traceIDHex))
		c.Next()
	}
}

func contextWithTraceID(t *testing.T, ctx context.Context, traceIDHex string) context.Context {
	t.Helper()
	traceID, err := trace.TraceIDFromHex(traceIDHex)
	if err != nil {
		t.Fatalf("TraceIDFromHex: %v", err)
	}
	spanID, err := trace.SpanIDFromHex("0102030405060708")
	if err != nil {
		t.Fatalf("SpanIDFromHex: %v", err)
	}
	spanContext := trace.NewSpanContext(trace.SpanContextConfig{
		TraceID: traceID,
		SpanID:  spanID,
		Remote:  true,
	})
	return trace.ContextWithSpanContext(ctx, spanContext)
}
