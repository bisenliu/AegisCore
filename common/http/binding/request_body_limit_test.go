package binding

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"

	contracterrors "github.com/aegiscore/common/contract/errors"
	contractresponse "github.com/aegiscore/common/contract/response"
	commonmw "github.com/aegiscore/common/http/middleware"
)

func TestRequestBodyLimitWithJSONBinding(t *testing.T) {
	const maxBytes int64 = 32

	tests := []struct {
		name        string
		body        string
		chunked     bool
		wantStatus  int
		wantCode    contracterrors.Code
		wantHandled bool
	}{
		{name: "valid small json", body: `{"name":"ok"}`, wantStatus: http.StatusNoContent, wantHandled: true},
		{name: "empty body", wantStatus: http.StatusBadRequest, wantCode: contracterrors.CodeBadRequest},
		{name: "unknown field remains compatible", body: `{"name":"ok","extra":true}`, wantStatus: http.StatusNoContent, wantHandled: true},
		{name: "small trailing json remains bad request", body: `{"name":"ok"} {}`, wantStatus: http.StatusBadRequest, wantCode: contracterrors.CodeBadRequest},
		{name: "fixed length oversized", body: `{"name":"` + strings.Repeat("x", 40) + `"}`, wantStatus: http.StatusRequestEntityTooLarge, wantCode: contracterrors.CodeRequestBodyTooLarge},
		{name: "chunked oversized", body: `{"name":"` + strings.Repeat("x", 40) + `"}`, chunked: true, wantStatus: http.StatusRequestEntityTooLarge, wantCode: contracterrors.CodeRequestBodyTooLarge},
		{name: "oversized trailing json", body: `{"name":"ok"} {"padding":"` + strings.Repeat("x", 40) + `"}`, chunked: true, wantStatus: http.StatusRequestEntityTooLarge, wantCode: contracterrors.CodeRequestBodyTooLarge},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			engine, handled := newLimitedJSONBindingEngine(t, maxBytes)
			recorder := httptest.NewRecorder()
			request := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(tt.body))
			request.Header.Set("Content-Type", "application/json")
			if tt.chunked {
				request.ContentLength = -1
			}

			engine.ServeHTTP(recorder, request)

			require.Equal(t, tt.wantStatus, recorder.Code)
			require.Equal(t, tt.wantHandled, *handled)
			if tt.wantStatus != http.StatusNoContent {
				var envelope contractresponse.Envelope
				require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &envelope))
				require.False(t, envelope.Success)
				require.Equal(t, tt.wantCode, envelope.Code)
			}
		})
	}
}

func newLimitedJSONBindingEngine(t *testing.T, maxBytes int64) (*gin.Engine, *bool) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	validator := newTestValidator(t)
	limit, err := commonmw.RequestBodyLimit(maxBytes)
	require.NoError(t, err)
	handled := false
	engine := gin.New()
	engine.Use(limit)
	engine.POST("/", func(c *gin.Context) {
		var request struct {
			Name string `json:"name" validate:"required"`
		}
		if !BindOrAbort(validator, c, &request, JSONBinder) {
			return
		}
		handled = true
		c.Status(http.StatusNoContent)
	})
	return engine, &handled
}
