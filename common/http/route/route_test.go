package route

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestTemplateOrUnmatched(t *testing.T) {
	gin.SetMode(gin.TestMode)
	engine := gin.New()
	engine.GET("/api/v1/users/:user_id", func(c *gin.Context) {
		require.Equal(t, "/api/v1/users/:user_id", TemplateOrUnmatched(c))
		c.Status(http.StatusNoContent)
	})

	engine.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/api/v1/users/123", nil))
}

func TestTemplateOrUnmatchedReturnsFallback(t *testing.T) {
	gin.SetMode(gin.TestMode)
	ctx, _ := gin.CreateTestContext(httptest.NewRecorder())
	ctx.Request = httptest.NewRequest(http.MethodGet, "/api/v1/users/123", nil)

	require.Equal(t, Unmatched, TemplateOrUnmatched(ctx))
	require.Equal(t, Unmatched, TemplateOrUnmatched(nil))
}
