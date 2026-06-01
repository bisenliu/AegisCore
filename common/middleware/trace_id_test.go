package middleware

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/aegiscore/common/logger"
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
	"go.uber.org/zap/zaptest/observer"
)

func TestTraceIDPropagatesHeaderToGinAndGoContext(t *testing.T) {
	gin.SetMode(gin.TestMode)
	engine := gin.New()
	engine.Use(TraceID())
	engine.GET("/trace", func(c *gin.Context) {
		v, ok := c.Get(TraceIDKey)
		if !ok || v != "trace-123" {
			t.Fatalf("gin context trace id = %#v, %v; want trace-123, true", v, ok)
		}
		if got := traceID(c); got != "trace-123" {
			t.Fatalf("traceID(c) = %q, want trace-123", got)
		}
		if got := logger.TraceIDFromContext(c.Request.Context()); got != "trace-123" {
			t.Fatalf("TraceIDFromContext = %q, want trace-123", got)
		}
		c.Status(http.StatusNoContent)
	})

	req := httptest.NewRequest(http.MethodGet, "/trace", nil)
	req.Header.Set("X-Trace-ID", "trace-123")
	rec := httptest.NewRecorder()
	engine.ServeHTTP(rec, req)

	if rec.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want 204", rec.Code)
	}
	if got := rec.Header().Get("X-Trace-ID"); got != "trace-123" {
		t.Fatalf("X-Trace-ID = %q, want trace-123", got)
	}
}

func TestTraceIDGeneratesWhenMissing(t *testing.T) {
	gin.SetMode(gin.TestMode)
	engine := gin.New()
	engine.Use(TraceID())
	engine.GET("/trace", func(c *gin.Context) {
		got := traceID(c)
		if got == "" {
			t.Fatal("traceID(c) = empty, want generated value")
		}
		if ctxTraceID := logger.TraceIDFromContext(c.Request.Context()); ctxTraceID != got {
			t.Fatalf("TraceIDFromContext = %q, want %q", ctxTraceID, got)
		}
		c.Status(http.StatusNoContent)
	})

	rec := httptest.NewRecorder()
	engine.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/trace", nil))

	if got := rec.Header().Get("X-Trace-ID"); got == "" {
		t.Fatal("X-Trace-ID = empty, want generated value")
	}
}

func TestTraceIDReplacesUnsafeInboundValue(t *testing.T) {
	gin.SetMode(gin.TestMode)
	engine := gin.New()
	engine.Use(TraceIDWithOptions(TraceIDOptions{MaxLength: 8, Validate: func(value string) bool { return !strings.Contains(value, " ") }}))
	engine.GET("/trace", func(c *gin.Context) {
		got := traceID(c)
		if got == "unsafe trace value" || got == "" {
			t.Fatalf("traceID(c) = %q, want generated replacement", got)
		}
		if ctxTraceID := logger.TraceIDFromContext(c.Request.Context()); ctxTraceID != got {
			t.Fatalf("TraceIDFromContext = %q, want %q", ctxTraceID, got)
		}
		c.Status(http.StatusNoContent)
	})

	req := httptest.NewRequest(http.MethodGet, "/trace", nil)
	req.Header.Set(HeaderTraceID, "unsafe trace value")
	rec := httptest.NewRecorder()
	engine.ServeHTTP(rec, req)

	if got := rec.Header().Get(HeaderTraceID); got == "unsafe trace value" || got == "" {
		t.Fatalf("X-Trace-ID = %q, want generated replacement", got)
	}
}

func TestTraceIDStoresBaseLoggerInRequestContext(t *testing.T) {
	gin.SetMode(gin.TestMode)
	core, logs := observer.New(zap.InfoLevel)
	log := zap.New(core)
	engine := gin.New()
	engine.Use(func(c *gin.Context) {
		c.Set(ContextKeyLogger, log)
		c.Next()
	}, TraceID())
	engine.GET("/trace", func(c *gin.Context) {
		logger.Info(c.Request.Context(), "context logger used")
		c.Status(http.StatusNoContent)
	})

	req := httptest.NewRequest(http.MethodGet, "/trace", nil)
	req.Header.Set(HeaderTraceID, "trace-context-log")
	engine.ServeHTTP(httptest.NewRecorder(), req)

	entries := logs.FilterMessage("context logger used").All()
	if len(entries) != 1 {
		t.Fatalf("context log count = %d, want 1", len(entries))
	}
	if got := entries[0].ContextMap()[logger.TraceIDField]; got != "trace-context-log" {
		t.Fatalf("%s = %q, want trace-context-log", logger.TraceIDField, got)
	}
}

func TestCORSWithOptions(t *testing.T) {
	gin.SetMode(gin.TestMode)
	engine := gin.New()
	engine.Use(CORSWithOptions(CORSOptions{
		AllowedMethods:   []string{http.MethodGet},
		AllowedHeaders:   []string{"Content-Type"},
		ExposedHeaders:   []string{HeaderTraceID},
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
	if got := rec.Header().Get(HeaderAccessControlExposeHeaders); got != HeaderTraceID {
		t.Fatalf("expose headers = %q", got)
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
	engine.Use(TraceID(), RequestLogger(log))
	engine.GET("/ok", func(c *gin.Context) { c.Status(http.StatusAccepted) })

	req := httptest.NewRequest(http.MethodGet, "/ok", nil)
	req.Header.Set("X-Trace-ID", "trace-log")
	engine.ServeHTTP(httptest.NewRecorder(), req)

	entries := logs.FilterMessage("http request completed").All()
	if len(entries) != 1 {
		t.Fatalf("request log count = %d, want 1", len(entries))
	}
	fields := entries[0].ContextMap()
	if fields[logger.TraceIDField] != "trace-log" || fields["method"] != http.MethodGet || fields["path"] != "/ok" || fields["status"] != int64(http.StatusAccepted) {
		t.Fatalf("request log fields = %#v", fields)
	}
	if _, ok := fields["latency"]; !ok {
		t.Fatalf("request log missing latency: %#v", fields)
	}
	if _, ok := fields["client_ip"]; !ok {
		t.Fatalf("request log missing client_ip: %#v", fields)
	}
}

func TestRecoveryIncludesTraceIDAndEnvelope(t *testing.T) {
	gin.SetMode(gin.TestMode)
	core, logs := observer.New(zap.ErrorLevel)
	log := zap.New(core)
	engine := gin.New()
	engine.Use(TraceID(), Recovery(log))
	engine.GET("/panic", func(c *gin.Context) { panic("boom") })

	req := httptest.NewRequest(http.MethodGet, "/panic", nil)
	req.Header.Set("X-Trace-ID", "trace-panic")
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
	if fields[logger.TraceIDField] != "trace-panic" || fields["panic"] != "boom" {
		t.Fatalf("recovery log fields = %#v", fields)
	}
	if _, ok := fields["stack"]; !ok {
		t.Fatalf("recovery log missing stack: %#v", fields)
	}
}
