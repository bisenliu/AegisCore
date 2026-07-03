package middleware

import (
	"context"
	"net/http"
	"net/http/httptest"
	"regexp"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
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
	require.Len(t, entries, 1)
	require.Equal(t, "00112233445566778899aabbccddeeff", entries[0].ContextMap()[logger.TraceIDField])
	require.Equal(t, "0102030405060708", entries[0].ContextMap()[logger.SpanIDField])
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

	require.Equal(t, "https://example.test", rec.Header().Get(HeaderAccessControlAllowOrigin))
	require.Equal(t, HeaderOrigin, rec.Header().Get(HeaderVary))
	require.Empty(t, rec.Header().Get(HeaderAccessControlExposeHeaders))
	require.Equal(t, "true", rec.Header().Get(HeaderAccessControlAllowCredentials))
	require.Equal(t, "600", rec.Header().Get(HeaderAccessControlMaxAge))
}

func TestRequestLoggerIncludesTraceAndSpanIDAndRequestFields(t *testing.T) {
	gin.SetMode(gin.TestMode)
	core, logs := observer.New(zap.InfoLevel)
	log := zap.New(core)
	engine := gin.New()
	engine.Use(otelTraceMiddleware(t, "00112233445566778899aabbccddeeff", "0102030405060708"), RequestID(), RequestLogger(log))
	engine.GET("/ok", func(c *gin.Context) { c.Status(http.StatusAccepted) })

	req := httptest.NewRequest(http.MethodGet, "/ok", nil)
	req.Header.Set(HeaderRequestID, "client-request-123")
	engine.ServeHTTP(httptest.NewRecorder(), req)

	entries := logs.FilterMessage("http request completed").All()
	require.Len(t, entries, 1)
	fields := entries[0].ContextMap()
	require.Equal(t, "00112233445566778899aabbccddeeff", fields[logger.TraceIDField])
	require.Equal(t, "0102030405060708", fields[logger.SpanIDField])
	require.Equal(t, "client-request-123", fields[RequestIDField])
	require.Equal(t, http.MethodGet, fields["method"])
	require.Equal(t, "/ok", fields["path"])
	require.Equal(t, int64(http.StatusAccepted), fields["status"])
	require.Equal(t, anonymousUserID, fields[auth.UserIDKey])
	require.Contains(t, fields, "latency_ms")
	require.Contains(t, fields, "client_ip")
}

func TestRequestIDPassesThroughHeaderAndContext(t *testing.T) {
	gin.SetMode(gin.TestMode)
	engine := gin.New()
	engine.Use(RequestID())
	engine.GET("/request-id", func(c *gin.Context) {
		requestID, ok := RequestIDFromContext(c.Request.Context())
		require.True(t, ok)
		c.String(http.StatusOK, requestID)
	})

	req := httptest.NewRequest(http.MethodGet, "/request-id", nil)
	req.Header.Set(HeaderRequestID, "client-request-123")
	rec := httptest.NewRecorder()
	engine.ServeHTTP(rec, req)

	require.Equal(t, "client-request-123", rec.Header().Get(HeaderRequestID))
	require.Equal(t, "client-request-123", rec.Body.String())
}

func TestRequestIDGeneratesMissingHeader(t *testing.T) {
	gin.SetMode(gin.TestMode)
	engine := gin.New()
	engine.Use(RequestID())
	engine.GET("/request-id", func(c *gin.Context) {
		requestID, ok := RequestIDFromContext(c.Request.Context())
		require.True(t, ok)
		c.String(http.StatusOK, requestID)
	})

	rec := httptest.NewRecorder()
	engine.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/request-id", nil))

	requestID := rec.Header().Get(HeaderRequestID)
	require.Regexp(t, uuidStringPattern, requestID)
	require.Equal(t, requestID, rec.Body.String())
}

func TestRequestIDRejectsInvalidHeader(t *testing.T) {
	gin.SetMode(gin.TestMode)
	tests := []struct {
		name  string
		value string
	}{
		{name: "blank", value: "   "},
		{name: "too long", value: strings.Repeat("a", maxRequestIDLength+1)},
		{name: "control character", value: "client\nrequest"},
		{name: "delete character", value: "client" + string(rune(0x7f)) + "request"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			engine := gin.New()
			engine.Use(RequestID())
			engine.GET("/request-id", func(c *gin.Context) {
				requestID, ok := RequestIDFromContext(c.Request.Context())
				require.True(t, ok)
				c.String(http.StatusOK, requestID)
			})

			req := httptest.NewRequest(http.MethodGet, "/request-id", nil)
			req.Header.Set(HeaderRequestID, tt.value)
			rec := httptest.NewRecorder()
			engine.ServeHTTP(rec, req)

			requestID := rec.Header().Get(HeaderRequestID)
			require.NotEqual(t, strings.TrimSpace(tt.value), requestID)
			require.Regexp(t, uuidStringPattern, requestID)
			require.Equal(t, requestID, rec.Body.String())
		})
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
	require.Len(t, entries, 1)
	fields := entries[0].ContextMap()
	require.Equal(t, "203.0.113.10", fields["client_ip"])
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
			require.Len(t, entries, 1)
			require.Equal(t, tt.level, entries[0].Level)
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
			require.Len(t, entries, 1)
			span := endedSpan(t, spanRecorder)
			require.Equal(t, tt.wantStatus, span.Status().Code)
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
	require.Len(t, entries, 1)
	fields := entries[0].ContextMap()
	require.Equal(t, "u-123", fields[auth.UserIDKey])
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
	require.Len(t, entries, 1)
	require.Equal(t, "/keep", entries[0].ContextMap()["path"])
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

	require.Equal(t, http.StatusInternalServerError, rec.Code)
	require.Contains(t, rec.Body.String(), `"success":false`)
	entries := logs.FilterMessage("panic recovered").All()
	require.Len(t, entries, 1)
	fields := entries[0].ContextMap()
	require.Equal(t, "00112233445566778899aabbccddeeff", fields[logger.TraceIDField])
	require.Equal(t, "0102030405060708", fields[logger.SpanIDField])
	require.Equal(t, "boom", fields["panic"])
	require.Contains(t, fields, "stack")
}

var uuidStringPattern = regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$`)

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

	require.Equal(t, http.StatusInternalServerError, rec.Code)
	require.Contains(t, rec.Body.String(), `"success":false`)
	require.Len(t, logs.FilterMessage("panic recovered").All(), 1)
	span := endedSpan(t, spanRecorder)
	require.Equal(t, codes.Error, span.Status().Code)
	event := findSpanEvent(t, span, "exception")
	assertSpanEventStringAttribute(t, event, spanAttrErrorType, spanErrorTypePanic)
	for _, attr := range event.Attributes {
		text := attr.Value.String()
		require.NotContains(t, text, "stacktrace")
		require.NotContains(t, text, "token")
		require.NotContains(t, text, "password")
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
	require.NoError(t, err)
	spanID, err := trace.SpanIDFromHex(spanIDHex)
	require.NoError(t, err)
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
	require.NoError(t, provider.Shutdown(context.Background()))
}

func endedSpan(t *testing.T, recorder *tracetest.SpanRecorder) sdktrace.ReadOnlySpan {
	t.Helper()
	spans := recorder.Ended()
	require.Len(t, spans, 1)
	return spans[0]
}

func assertSpanIntAttribute(t *testing.T, span sdktrace.ReadOnlySpan, key string, want int) {
	t.Helper()
	var got *int
	for _, attr := range span.Attributes() {
		if string(attr.Key) == key {
			value := int(attr.Value.AsInt64())
			got = &value
			break
		}
	}
	require.NotNil(t, got, "span attribute %s missing in %#v", key, span.Attributes())
	require.Equal(t, want, *got)
}

func findSpanEvent(t *testing.T, span sdktrace.ReadOnlySpan, name string) sdktrace.Event {
	t.Helper()
	var found sdktrace.Event
	foundEvent := false
	for _, event := range span.Events() {
		if event.Name == name {
			found = event
			foundEvent = true
			break
		}
	}
	require.True(t, foundEvent, "span event %q missing in %#v", name, span.Events())
	return found
}

func assertSpanEventStringAttribute(t *testing.T, event sdktrace.Event, key string, want string) {
	t.Helper()
	var got *string
	for _, attr := range event.Attributes {
		if string(attr.Key) == key {
			value := attr.Value.AsString()
			got = &value
			break
		}
	}
	require.NotNil(t, got, "span event attribute %s missing in %#v", key, event.Attributes)
	require.Equal(t, want, *got)
}
