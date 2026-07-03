package authorization

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

func TestAuthorizerEnforceDelegatesValidUserID(t *testing.T) {
	engine := NewMockEngine(gomock.NewController(t))
	var gotUserID uuid.UUID
	var gotPathTemplate string
	var gotMethod string
	engine.EXPECT().Enforce(gomock.Any(), gomock.Any(), "/api/v1/users/:user_id", "GET").DoAndReturn(func(_ context.Context, userID uuid.UUID, pathTemplate string, method string) (bool, error) {
		gotUserID = userID
		gotPathTemplate = pathTemplate
		gotMethod = method
		return true, nil
	})
	authz := NewAuthorizer(engine)
	userID := "018f0000-0000-7000-8000-000000000701"
	allowed, err := authz.Enforce(context.Background(), userID, "/api/v1/users/:user_id", "GET")
	require.NoError(t, err)
	require.True(t, allowed)
	require.Equal(t, userID, gotUserID.String())
	require.Equal(t, "/api/v1/users/:user_id", gotPathTemplate)
	require.Equal(t, "GET", gotMethod)
}

func TestAuthorizerEnforceInvalidUserIDFailsClosed(t *testing.T) {
	engine := NewMockEngine(gomock.NewController(t))
	authz := NewAuthorizer(engine)
	allowed, err := authz.Enforce(context.Background(), "not-a-uuid", "/api/v1/users", "GET")
	require.ErrorIs(t, err, ErrInvalidSubjectUserID)
	require.False(t, allowed)
}
