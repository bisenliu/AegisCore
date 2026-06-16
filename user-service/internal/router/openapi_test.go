package router

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"

	"github.com/aegiscore/user-service/docs"
)

func TestOpenAPIEnabled(t *testing.T) {
	t.Setenv(openAPIEnabledEnv, "")

	t.Run("production disabled by default", func(t *testing.T) {
		t.Setenv(openAPIEnabledEnv, "")
		if openAPIEnabled("production") {
			t.Fatal("openAPIEnabled(production) = true, want false")
		}
	})

	t.Run("local enabled by default", func(t *testing.T) {
		t.Setenv(openAPIEnabledEnv, "")
		if !openAPIEnabled("local") {
			t.Fatal("openAPIEnabled(local) = false, want true")
		}
	})

	t.Run("env override", func(t *testing.T) {
		t.Setenv(openAPIEnabledEnv, "true")
		if !openAPIEnabled("production") {
			t.Fatal("openAPIEnabled(production) with override = false, want true")
		}
		t.Setenv(openAPIEnabledEnv, "false")
		if openAPIEnabled("local") {
			t.Fatal("openAPIEnabled(local) with override = true, want false")
		}
	})
}

func TestRegisterOpenAPIRedirects(t *testing.T) {
	gin.SetMode(gin.TestMode)
	t.Setenv(openAPIEnabledEnv, "true")
	engine := gin.New()
	RegisterOpenAPI(engine, "production")

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, openAPIUIPath, nil)
	engine.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusOK {
		t.Fatalf("%s status = %d, want %d", openAPIUIPath, recorder.Code, http.StatusOK)
	}

	recorder = httptest.NewRecorder()
	request = httptest.NewRequest(http.MethodGet, openAPIJSONPath, nil)
	engine.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusOK {
		t.Fatalf("%s status = %d, want %d", openAPIJSONPath, recorder.Code, http.StatusOK)
	}
	if contentType := recorder.Header().Get("Content-Type"); !strings.HasPrefix(contentType, "application/json") {
		t.Fatalf("%s Content-Type = %q, want application/json", openAPIJSONPath, contentType)
	}

	for _, path := range []string{"/docs", "/api-docs"} {
		recorder := httptest.NewRecorder()
		request := httptest.NewRequest(http.MethodGet, path, nil)
		engine.ServeHTTP(recorder, request)
		if recorder.Code != http.StatusMovedPermanently {
			t.Fatalf("%s status = %d, want %d", path, recorder.Code, http.StatusMovedPermanently)
		}
		if location := recorder.Header().Get("Location"); location != openAPIUIPath {
			t.Fatalf("%s Location = %q, want %s", path, location, openAPIUIPath)
		}
	}
}

func TestRegisterOpenAPIDisabled(t *testing.T) {
	gin.SetMode(gin.TestMode)
	t.Setenv(openAPIEnabledEnv, "false")
	engine := gin.New()
	RegisterOpenAPI(engine, "local")

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/docs", nil)
	engine.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusNotFound)
	}
}

func TestOpenAPIDocumentRoutes(t *testing.T) {
	var document struct {
		OpenAPI       string `json:"openapi"`
		LegacySwagger string `json:"swagger"`
		Servers       []struct {
			URL string `json:"url"`
		} `json:"servers"`
		Paths map[string]struct {
			Servers []struct {
				URL string `json:"url"`
			} `json:"servers"`
		} `json:"paths"`
		Components struct {
			SecuritySchemes map[string]struct {
				Type         string `json:"type"`
				Scheme       string `json:"scheme"`
				BearerFormat string `json:"bearerFormat"`
			} `json:"securitySchemes"`
		} `json:"components"`
	}
	if err := json.Unmarshal(docs.ReadOpenAPI(), &document); err != nil {
		t.Fatalf("decode OpenAPI document: %v", err)
	}

	if !strings.HasPrefix(document.OpenAPI, "3.") {
		t.Fatalf("openapi = %q, want 3.x", document.OpenAPI)
	}
	if document.LegacySwagger != "" {
		t.Fatalf("swagger = %q, want empty", document.LegacySwagger)
	}
	if len(document.Servers) != 1 || document.Servers[0].URL != "/api/v1" {
		t.Fatalf("servers = %+v, want /api/v1", document.Servers)
	}

	for _, path := range []string{"/livez", "/readyz", "/startupz"} {
		item, ok := document.Paths[path]
		if !ok {
			t.Fatalf("missing OpenAPI path %s", path)
		}
		if len(item.Servers) != 1 || item.Servers[0].URL != "/" {
			t.Fatalf("%s servers = %+v, want /", path, item.Servers)
		}
		if _, ok := document.Paths["/api/v1"+path]; ok {
			t.Fatalf("OpenAPI document should not include /api/v1%s", path)
		}
	}

	for _, path := range []string{"/permissions", "/users", "/auth/login"} {
		if _, ok := document.Paths[path]; !ok {
			t.Fatalf("missing OpenAPI path %s", path)
		}
		if _, ok := document.Paths["/api/v1"+path]; ok {
			t.Fatalf("OpenAPI document should not include /api/v1%s", path)
		}
	}

	bearer := document.Components.SecuritySchemes["BearerAuth"]
	if bearer.Type != "http" || bearer.Scheme != "bearer" || bearer.BearerFormat != "JWT" {
		t.Fatalf("BearerAuth = %+v, want http bearer JWT", bearer)
	}
}
