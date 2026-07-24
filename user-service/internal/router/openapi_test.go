package router

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"

	"github.com/aegiscore/user-service/docs"
)

func TestOpenAPIEnabled(t *testing.T) {
	t.Run("production disabled by default", func(t *testing.T) {
		require.False(t, openAPIEnabled("production"))
	})

	t.Run("local enabled by default", func(t *testing.T) {
		require.True(t, openAPIEnabled("local"))
	})
}

func TestRegisterOpenAPIRedirects(t *testing.T) {
	gin.SetMode(gin.TestMode)
	engine := gin.New()
	RegisterOpenAPI(engine, "local")

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, openAPIUIPath, nil)
	engine.ServeHTTP(recorder, request)
	require.Equal(t, http.StatusOK, recorder.Code)
	require.Contains(t, recorder.Body.String(), "swagger-initializer.js")

	recorder = httptest.NewRecorder()
	request = httptest.NewRequest(http.MethodGet, "/openapi/swagger-initializer.js", nil)
	engine.ServeHTTP(recorder, request)
	require.Equal(t, http.StatusOK, recorder.Code)
	require.Contains(t, recorder.Body.String(), openAPIJSONPath)
	require.NotContains(t, recorder.Body.String(), "https://petstore.swagger.io/v2/swagger.json")

	for _, path := range []string{
		"/openapi/swagger-ui.css",
		"/openapi/swagger-ui-bundle.js",
		"/openapi/swagger-ui-standalone-preset.js",
		"/openapi/oauth2-redirect.html",
		"/openapi/favicon-32x32.png",
	} {
		recorder = httptest.NewRecorder()
		request = httptest.NewRequest(http.MethodGet, path, nil)
		engine.ServeHTTP(recorder, request)
		require.Equal(t, http.StatusOK, recorder.Code)
		require.NotEmpty(t, recorder.Body.Bytes())
	}

	recorder = httptest.NewRecorder()
	request = httptest.NewRequest(http.MethodGet, "/openapi/missing.asset", nil)
	engine.ServeHTTP(recorder, request)
	require.Equal(t, http.StatusNotFound, recorder.Code)

	recorder = httptest.NewRecorder()
	request = httptest.NewRequest(http.MethodGet, openAPIJSONPath, nil)
	engine.ServeHTTP(recorder, request)
	require.Equal(t, http.StatusOK, recorder.Code)
	require.Regexp(t, `^application/json`, recorder.Header().Get("Content-Type"))

	for _, path := range []string{"/docs", "/api-docs"} {
		recorder := httptest.NewRecorder()
		request := httptest.NewRequest(http.MethodGet, path, nil)
		engine.ServeHTTP(recorder, request)
		require.Equal(t, http.StatusMovedPermanently, recorder.Code)
		require.Equal(t, openAPIUIPath, recorder.Header().Get("Location"))
	}
}

func TestRegisterOpenAPIDisabled(t *testing.T) {
	gin.SetMode(gin.TestMode)
	engine := gin.New()
	RegisterOpenAPI(engine, "production")

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/docs", nil)
	engine.ServeHTTP(recorder, request)
	require.Equal(t, http.StatusNotFound, recorder.Code)
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
	require.NoError(t, json.Unmarshal(docs.ReadOpenAPI(), &document))

	require.Regexp(t, `^3\.`, document.OpenAPI)
	require.Empty(t, document.LegacySwagger)
	require.Len(t, document.Servers, 1)
	require.Equal(t, "/api/v1", document.Servers[0].URL)

	for _, path := range []string{"/livez", "/readyz", "/startupz"} {
		require.Contains(t, document.Paths, path)
		item := document.Paths[path]
		require.Len(t, item.Servers, 1)
		require.Equal(t, "/", item.Servers[0].URL)
		require.NotContains(t, document.Paths, "/api/v1"+path)
	}

	for _, path := range []string{"/permissions", "/users", "/auth/login"} {
		require.Contains(t, document.Paths, path)
		require.NotContains(t, document.Paths, "/api/v1"+path)
	}

	bearer := document.Components.SecuritySchemes["BearerAuth"]
	require.Equal(t, "http", bearer.Type)
	require.Equal(t, "bearer", bearer.Scheme)
	require.Equal(t, "JWT", bearer.BearerFormat)
}
