package middleware

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"go.opentelemetry.io/otel/codes"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
	"go.opentelemetry.io/otel/trace"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"go.uber.org/zap/zaptest/observer"

	"github.com/aegiscore/common/runtime/logger"
	"github.com/aegiscore/common/security/auth"
)

func TestContextLoggerUsesOTelTraceAndSpanID(t *testing.T) {
	core, logs := observer.New(zap.InfoLevel)
	log := zap.New(core)
	ctx := logger.ToContext(contextWithSpanContext(context.Background(), t, "00112233445566778899aabbccddeeff", "0102030405060708"), log)

	logger.Info(ctx, "context logger used")

	entries := logs.FilterMessage("context logger used").All()
	if len(entries) != 1 {
		t.Fatalf("context log count = %d, want 1", len(entries))
	}
	if got := entries[0].ContextMap()[logger.TraceIDField]; got != "00112233445566778899aabbccddeeff" {
		t.Fatalf("%s = %q, want OTel trace id", logger.TraceIDField, got)
	}
	if got := entries[0].ContextMap()[logger.SpanIDField]; got != "0102030405060708" {
		t.Fatalf("%s = %q, want OTel span id", logger.SpanIDField, got)
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

func TestRequestLoggerIncludesTraceAndSpanIDAndRequestFields(t *testing.T) {
	gin.SetMode(gin.TestMode)
	core, logs := observer.New(zap.InfoLevel)
	log := zap.New(core)
	engine := gin.New()
	engine.Use(otelTraceMiddleware(t, "00112233445566778899aabbccddeeff", "0102030405060708"), RequestLogger(log))
	engine.GET("/ok", func(c *gin.Context) { c.Status(http.StatusAccepted) })

	req := httptest.NewRequest(http.MethodGet, "/ok", nil)
	engine.ServeHTTP(httptest.NewRecorder(), req)

	entries := logs.FilterMessage("http request completed").All()
	if len(entries) != 1 {
		t.Fatalf("request log count = %d, want 1", len(entries))
	}
	fields := entries[0].ContextMap()
	if fields[logger.TraceIDField] != "00112233445566778899aabbccddeeff" || fields[logger.SpanIDField] != "0102030405060708" || fields["method"] != http.MethodGet || fields["path"] != "/ok" || fields["status"] != int64(http.StatusAccepted) || fields[auth.UserIDKey] != anonymousUserID {
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

func TestRequestLoggerMarksOnlyServerErrorSpanStatus(t *testing.T) {
	gin.SetMode(gin.TestMode)
	tests := []struct {
		name       string
		status     int
		wantStatus codes.Code
	}{
		{name: "success", status: http.StatusOK, wantStatus: codes.Unset},
		{name: "client error", status: http.StatusNotFound, wantStatus: codes.Unset},
		{name: "server error", status: http.StatusInternalServerError, wantStatus: codes.Error},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			core, logs := observer.New(zap.DebugLevel)
			spanRecorder := tracetest.NewSpanRecorder()
			provider := sdktrace.NewTracerProvider(sdktrace.WithSpanProcessor(spanRecorder))
			defer shutdownTracerProvider(t, provider)
			engine := gin.New()
			engine.Use(recordingSpanMiddleware(provider), RequestLogger(zap.New(core)))
			engine.GET("/status", func(c *gin.Context) { c.Status(tt.status) })

			req := httptest.NewRequest(http.MethodGet, "/status", nil)
			engine.ServeHTTP(httptest.NewRecorder(), req)

			entries := logs.FilterMessage("http request completed").All()
			if len(entries) != 1 {
				t.Fatalf("request log count = %d, want 1", len(entries))
			}
			span := endedSpan(t, spanRecorder)
			if got := span.Status().Code; got != tt.wantStatus {
				t.Fatalf("span status = %s, want %s", got, tt.wantStatus)
			}
			if tt.status >= http.StatusInternalServerError {
				assertSpanIntAttribute(t, span, spanAttrHTTPStatus, tt.status)
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

func TestRecoveryIncludesTraceAndSpanIDAndEnvelope(t *testing.T) {
	gin.SetMode(gin.TestMode)
	core, logs := observer.New(zap.ErrorLevel)
	log := zap.New(core)
	engine := gin.New()
	engine.Use(otelTraceMiddleware(t, "00112233445566778899aabbccddeeff", "0102030405060708"), Recovery(log))
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
	if fields[logger.TraceIDField] != "00112233445566778899aabbccddeeff" || fields[logger.SpanIDField] != "0102030405060708" || fields["panic"] != "boom" {
		t.Fatalf("recovery log fields = %#v", fields)
	}
	if _, ok := fields["stack"]; !ok {
		t.Fatalf("recovery log missing stack: %#v", fields)
	}
}

func TestRecoveryRecordsPanicOnSpan(t *testing.T) {
	gin.SetMode(gin.TestMode)
	core, logs := observer.New(zap.ErrorLevel)
	spanRecorder := tracetest.NewSpanRecorder()
	provider := sdktrace.NewTracerProvider(sdktrace.WithSpanProcessor(spanRecorder))
	defer shutdownTracerProvider(t, provider)
	engine := gin.New()
	engine.Use(recordingSpanMiddleware(provider), Recovery(zap.New(core)))
	engine.GET("/panic", func(_ *gin.Context) { panic("boom stacktrace token password") })

	req := httptest.NewRequest(http.MethodGet, "/panic", nil)
	rec := httptest.NewRecorder()
	engine.ServeHTTP(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want 500", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), `"success":false`) {
		t.Fatalf("response body = %s, want failure envelope", rec.Body.String())
	}
	if entries := logs.FilterMessage("panic recovered").All(); len(entries) != 1 {
		t.Fatalf("panic recovered logs = %d, want 1", len(entries))
	}
	span := endedSpan(t, spanRecorder)
	if got := span.Status().Code; got != codes.Error {
		t.Fatalf("span status = %s, want Error", got)
	}
	event := findSpanEvent(t, span, "exception")
	assertSpanEventStringAttribute(t, event, spanAttrErrorType, spanErrorTypePanic)
	for _, attr := range event.Attributes {
		text := attr.Value.Emit()
		if strings.Contains(text, "stacktrace") || strings.Contains(text, "token") || strings.Contains(text, "password") {
			t.Fatalf("panic span event leaked sensitive text: %#v", event.Attributes)
		}
	}
}

func otelTraceMiddleware(t *testing.T, traceIDHex string, spanIDHex string) gin.HandlerFunc {
	t.Helper()
	return func(c *gin.Context) {
		c.Request = c.Request.WithContext(contextWithSpanContext(c.Request.Context(), t, traceIDHex, spanIDHex))
		c.Next()
	}
}

func contextWithSpanContext(ctx context.Context, t *testing.T, traceIDHex string, spanIDHex string) context.Context {
	t.Helper()
	traceID, err := trace.TraceIDFromHex(traceIDHex)
	if err != nil {
		t.Fatalf("TraceIDFromHex: %v", err)
	}
	spanID, err := trace.SpanIDFromHex(spanIDHex)
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

func recordingSpanMiddleware(provider *sdktrace.TracerProvider) gin.HandlerFunc {
	tracer := provider.Tracer("common-http-middleware-test")
	return func(c *gin.Context) {
		ctx, span := tracer.Start(c.Request.Context(), c.Request.Method+" "+c.Request.URL.Path)
		c.Request = c.Request.WithContext(ctx)
		defer span.End()
		c.Next()
	}
}

func shutdownTracerProvider(t *testing.T, provider *sdktrace.TracerProvider) {
	t.Helper()
	if err := provider.Shutdown(context.Background()); err != nil {
		t.Fatalf("Shutdown tracer provider: %v", err)
	}
}

func endedSpan(t *testing.T, recorder *tracetest.SpanRecorder) sdktrace.ReadOnlySpan {
	t.Helper()
	spans := recorder.Ended()
	if len(spans) != 1 {
		t.Fatalf("ended span count = %d, want 1", len(spans))
	}
	return spans[0]
}

func assertSpanIntAttribute(t *testing.T, span sdktrace.ReadOnlySpan, key string, want int) {
	t.Helper()
	for _, attr := range span.Attributes() {
		if string(attr.Key) == key {
			if got := int(attr.Value.AsInt64()); got != want {
				t.Fatalf("span attribute %s = %d, want %d", key, got, want)
			}
			return
		}
	}
	t.Fatalf("span attribute %s missing in %#v", key, span.Attributes())
}

func findSpanEvent(t *testing.T, span sdktrace.ReadOnlySpan, name string) sdktrace.Event {
	t.Helper()
	for _, event := range span.Events() {
		if event.Name == name {
			return event
		}
	}
	t.Fatalf("span event %q missing in %#v", name, span.Events())
	return sdktrace.Event{}
}

func assertSpanEventStringAttribute(t *testing.T, event sdktrace.Event, key string, want string) {
	t.Helper()
	for _, attr := range event.Attributes {
		if string(attr.Key) == key {
			if got := attr.Value.AsString(); got != want {
				t.Fatalf("span event attribute %s = %q, want %q", key, got, want)
			}
			return
		}
	}
	t.Fatalf("span event attribute %s missing in %#v", key, event.Attributes)
}
