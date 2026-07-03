package pprof

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestRegisterExposesDefaultPprofRoutes(t *testing.T) {
	gin.SetMode(gin.TestMode)
	engine := gin.New()
	Register(engine, Options{})

	t.Run("index redirects to trailing slash", func(t *testing.T) {
		recorder := executeRequest(engine, "/debug/pprof")
		require.Equal(t, http.StatusMovedPermanently, recorder.Code)
		require.Equal(t, "/debug/pprof/", recorder.Header().Get("Location"))
	})

	t.Run("index lists profiles", func(t *testing.T) {
		recorder := executeRequest(engine, "/debug/pprof/")
		require.Equal(t, http.StatusOK, recorder.Code)
		require.Contains(t, recorder.Body.String(), "Types of profiles available")
	})

	t.Run("runtime profile is served", func(t *testing.T) {
		recorder := executeRequest(engine, "/debug/pprof/goroutine?debug=1")
		require.Equal(t, http.StatusOK, recorder.Code, recorder.Body.String())
		require.Equal(t, "nosniff", recorder.Header().Get("X-Content-Type-Options"))
	})
}

func TestRegisterSupportsCustomBasePath(t *testing.T) {
	gin.SetMode(gin.TestMode)
	engine := gin.New()
	Register(engine, Options{BasePath: "internal/debug/pprof/"})

	recorder := executeRequest(engine, "/internal/debug/pprof/goroutine?debug=1")
	require.Equal(t, http.StatusOK, recorder.Code, recorder.Body.String())
}

func TestHandlerReturnsHTTPHandler(t *testing.T) {
	handler := Handler(Options{BasePath: "/debug/pprof"})
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/debug/pprof/cmdline", nil)
	handler.ServeHTTP(recorder, request)
	require.Equal(t, http.StatusOK, recorder.Code)
}

func executeRequest(engine *gin.Engine, path string) *httptest.ResponseRecorder {
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, path, nil)
	engine.ServeHTTP(recorder, request)
	return recorder
}
