package pprof

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestRegisterExposesDefaultPprofRoutes(t *testing.T) {
	gin.SetMode(gin.TestMode)
	engine := gin.New()
	Register(engine, Options{})

	t.Run("index redirects to trailing slash", func(t *testing.T) {
		recorder := executeRequest(engine, "/debug/pprof")
		if recorder.Code != http.StatusMovedPermanently {
			t.Fatalf("status = %d, want %d", recorder.Code, http.StatusMovedPermanently)
		}
		if recorder.Header().Get("Location") != "/debug/pprof/" {
			t.Fatalf("Location = %q, want /debug/pprof/", recorder.Header().Get("Location"))
		}
	})

	t.Run("index lists profiles", func(t *testing.T) {
		recorder := executeRequest(engine, "/debug/pprof/")
		if recorder.Code != http.StatusOK {
			t.Fatalf("status = %d, want %d", recorder.Code, http.StatusOK)
		}
		if !strings.Contains(recorder.Body.String(), "Types of profiles available") {
			t.Fatalf("body missing profile index: %s", recorder.Body.String())
		}
	})

	t.Run("runtime profile is served", func(t *testing.T) {
		recorder := executeRequest(engine, "/debug/pprof/goroutine?debug=1")
		if recorder.Code != http.StatusOK {
			t.Fatalf("status = %d, want %d; body=%s", recorder.Code, http.StatusOK, recorder.Body.String())
		}
		if recorder.Header().Get("X-Content-Type-Options") != "nosniff" {
			t.Fatalf("X-Content-Type-Options = %q, want nosniff", recorder.Header().Get("X-Content-Type-Options"))
		}
	})
}

func TestRegisterSupportsCustomBasePath(t *testing.T) {
	gin.SetMode(gin.TestMode)
	engine := gin.New()
	Register(engine, Options{BasePath: "internal/debug/pprof/"})

	recorder := executeRequest(engine, "/internal/debug/pprof/goroutine?debug=1")
	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body=%s", recorder.Code, http.StatusOK, recorder.Body.String())
	}
}

func TestHandlerReturnsHTTPHandler(t *testing.T) {
	handler := Handler(Options{BasePath: "/debug/pprof"})
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/debug/pprof/cmdline", nil)
	handler.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusOK)
	}
}

func executeRequest(engine *gin.Engine, path string) *httptest.ResponseRecorder {
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, path, nil)
	engine.ServeHTTP(recorder, request)
	return recorder
}
