package router

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestSwaggerEnabled(t *testing.T) {
	t.Setenv(swaggerEnabledEnv, "")

	t.Run("production disabled by default", func(t *testing.T) {
		t.Setenv(swaggerEnabledEnv, "")
		if swaggerEnabled("production") {
			t.Fatal("swaggerEnabled(production) = true, want false")
		}
	})

	t.Run("local enabled by default", func(t *testing.T) {
		t.Setenv(swaggerEnabledEnv, "")
		if !swaggerEnabled("local") {
			t.Fatal("swaggerEnabled(local) = false, want true")
		}
	})

	t.Run("env override", func(t *testing.T) {
		t.Setenv(swaggerEnabledEnv, "true")
		if !swaggerEnabled("production") {
			t.Fatal("swaggerEnabled(production) with override = false, want true")
		}
		t.Setenv(swaggerEnabledEnv, "false")
		if swaggerEnabled("local") {
			t.Fatal("swaggerEnabled(local) with override = true, want false")
		}
	})
}

func TestRegisterSwaggerRedirects(t *testing.T) {
	gin.SetMode(gin.TestMode)
	t.Setenv(swaggerEnabledEnv, "true")
	engine := gin.New()
	RegisterSwagger(engine, "production")

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/swagger/index.html", nil)
	engine.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusOK {
		t.Fatalf("/swagger/index.html status = %d, want %d", recorder.Code, http.StatusOK)
	}

	for _, path := range []string{"/docs", "/api-docs"} {
		recorder := httptest.NewRecorder()
		request := httptest.NewRequest(http.MethodGet, path, nil)
		engine.ServeHTTP(recorder, request)
		if recorder.Code != http.StatusMovedPermanently {
			t.Fatalf("%s status = %d, want %d", path, recorder.Code, http.StatusMovedPermanently)
		}
		if location := recorder.Header().Get("Location"); location != "/swagger/index.html" {
			t.Fatalf("%s Location = %q, want /swagger/index.html", path, location)
		}
	}
}

func TestRegisterSwaggerDisabled(t *testing.T) {
	gin.SetMode(gin.TestMode)
	t.Setenv(swaggerEnabledEnv, "false")
	engine := gin.New()
	RegisterSwagger(engine, "local")

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/docs", nil)
	engine.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusNotFound)
	}
}
