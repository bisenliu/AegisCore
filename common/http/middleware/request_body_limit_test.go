package middleware

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestRequestBodyLimitRejectsInvalidLimit(t *testing.T) {
	for _, limit := range []int64{0, -1} {
		handler, err := RequestBodyLimit(limit)
		require.Nil(t, handler)
		require.EqualError(t, err, "request body max bytes must be > 0")
	}
}

func TestRequestBodyLimitRejectsKnownOversizedBodyBeforeHandler(t *testing.T) {
	gin.SetMode(gin.TestMode)
	limit, err := RequestBodyLimit(8)
	require.NoError(t, err)
	called := false
	engine := gin.New()
	engine.Use(limit)
	engine.POST("/", func(c *gin.Context) {
		called = true
		c.Status(http.StatusNoContent)
	})

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/", strings.NewReader("123456789"))
	engine.ServeHTTP(recorder, request)

	require.Equal(t, http.StatusRequestEntityTooLarge, recorder.Code)
	require.False(t, called)
}

func TestRequestBodyLimitConstrainsUnknownLengthBody(t *testing.T) {
	gin.SetMode(gin.TestMode)
	limit, err := RequestBodyLimit(8)
	require.NoError(t, err)
	var readErr error
	engine := gin.New()
	engine.Use(limit)
	engine.POST("/", func(c *gin.Context) {
		_, readErr = io.ReadAll(c.Request.Body)
		c.Status(http.StatusNoContent)
	})

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/", strings.NewReader("123456789"))
	request.ContentLength = -1
	engine.ServeHTTP(recorder, request)

	var maxBytesErr *http.MaxBytesError
	require.ErrorAs(t, readErr, &maxBytesErr)
	require.EqualValues(t, 8, maxBytesErr.Limit)
}
