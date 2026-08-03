package userhttp

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	contracterrors "github.com/aegiscore/common/contract/errors"
	contractresponse "github.com/aegiscore/common/contract/response"
	commonmw "github.com/aegiscore/common/http/middleware"
	"github.com/aegiscore/common/validation"
	userquery "github.com/aegiscore/user-service/internal/features/user/application/query"
)

func TestCreateUserRejectsOversizedRequestBodies(t *testing.T) {
	const maxBytes int64 = 64
	tests := []struct {
		name    string
		body    string
		chunked bool
	}{
		{name: "fixed length", body: `{"nickname":"` + strings.Repeat("x", 80) + `","username":"alice","password":"secret"}`},
		{name: "oversized trailing json", body: `{"nickname":"A","username":"alice","password":"secret"} {"padding":"` + strings.Repeat("x", 80) + `"}`, chunked: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gin.SetMode(gin.TestMode)
			validator, err := validation.NewDefault()
			require.NoError(t, err)
			ctl := NewUserController(
				NewMockCreateUserService(gomock.NewController(t)),
				NewMockUserQueryService(gomock.NewController(t)),
				validator,
			)
			limit, err := commonmw.RequestBodyLimit(maxBytes)
			require.NoError(t, err)
			engine := gin.New()
			engine.Use(limit)
			engine.POST("/api/v1/users", ctl.CreateUser)
			request := httptest.NewRequest(http.MethodPost, "/api/v1/users", strings.NewReader(tt.body))
			request.Header.Set("Content-Type", "application/json")
			if tt.chunked {
				request.ContentLength = -1
			}
			recorder := httptest.NewRecorder()

			engine.ServeHTTP(recorder, request)

			envelope := decodeUserBodyLimitEnvelope(t, recorder)
			require.Equal(t, http.StatusRequestEntityTooLarge, recorder.Code)
			require.False(t, envelope.Success)
			require.Equal(t, contracterrors.CodeRequestBodyTooLarge, envelope.Code)
			require.Equal(t, contracterrors.MessageRequestBodyTooLarge, envelope.Message)
		})
	}
}

func TestUserListQueryIsUnaffectedByRequestBodyLimit(t *testing.T) {
	gin.SetMode(gin.TestMode)
	validator, err := validation.NewDefault()
	require.NoError(t, err)
	queries := NewMockUserQueryService(gomock.NewController(t))
	queries.EXPECT().ListUsers(gomock.Any(), gomock.Any()).Return(&userquery.ListUsersResult{Items: nil, PageSize: 20}, nil)
	ctl := NewUserController(NewMockCreateUserService(gomock.NewController(t)), queries, validator)
	limit, err := commonmw.RequestBodyLimit(1)
	require.NoError(t, err)
	engine := gin.New()
	engine.Use(limit)
	engine.GET("/api/v1/users", ctl.ListUsers)
	recorder := httptest.NewRecorder()

	engine.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/api/v1/users?page_size=20&nickname=Alice", nil))

	envelope := decodeUserBodyLimitEnvelope(t, recorder)
	require.Equal(t, http.StatusOK, recorder.Code)
	require.True(t, envelope.Success)
	require.Equal(t, contracterrors.CodeOK, envelope.Code)
}

func decodeUserBodyLimitEnvelope(t *testing.T, recorder *httptest.ResponseRecorder) contractresponse.Envelope {
	t.Helper()
	var envelope contractresponse.Envelope
	require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &envelope))
	return envelope
}
