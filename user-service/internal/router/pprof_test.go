package router

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"

	"github.com/aegiscore/common/runtime/config"
)

func TestRegisterPprofRoutes(t *testing.T) {
	gin.SetMode(gin.TestMode)

	t.Run("disabled by default", func(t *testing.T) {
		engine := gin.New()
		registerPprofRoutes(engine, config.PprofConfig{})

		recorder := executePprofRequest(engine, "/debug/pprof/")
		if recorder.Code != http.StatusNotFound {
			t.Fatalf("status = %d, want %d", recorder.Code, http.StatusNotFound)
		}
	})

	t.Run("enabled exposes configured base path", func(t *testing.T) {
		engine := gin.New()
		registerPprofRoutes(engine, config.PprofConfig{Enabled: true, BasePath: "/debug/pprof"})

		recorder := executePprofRequest(engine, "/debug/pprof/goroutine?debug=1")
		if recorder.Code != http.StatusOK {
			t.Fatalf("status = %d, want %d; body=%s", recorder.Code, http.StatusOK, recorder.Body.String())
		}
	})
}

func executePprofRequest(engine *gin.Engine, path string) *httptest.ResponseRecorder {
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, path, nil)
	engine.ServeHTTP(recorder, request)
	return recorder
}
