package authctx

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	commonauth "github.com/aegiscore/common/security/auth"
	authdomain "github.com/aegiscore/user-service/internal/features/auth/domain"
)

func TestAuthenticatedSessionReturnsUserAndSession(t *testing.T) {
	ctx := commonauth.WithSessionID(commonauth.WithUserID(context.Background(), "018f6f3e-7c4d-7b2a-9f8a-4f6b1b2c3d4e"), "s-123")

	userID, sessionID, err := AuthenticatedSession(ctx)
	require.NoError(t, err,
		"AuthenticatedSession: %v", err)
	require.False(t, userID != uuid.MustParse("018f6f3e-7c4d-7b2a-9f8a-4f6b1b2c3d4e") || sessionID != "s-123",
		"userID=%q sessionID=%q", userID, sessionID)

}

func TestAuthenticatedSessionRejectsMissingOrInvalidContext(t *testing.T) {
	tests := []struct {
		name string
		ctx  context.Context
	}{
		{name: "missing user", ctx: context.Background()},
		{name: "invalid user", ctx: commonauth.WithUserID(context.Background(), "not-a-uuid")},
		{name: "missing session", ctx: commonauth.WithUserID(context.Background(), "018f6f3e-7c4d-7b2a-9f8a-4f6b1b2c3d4e")},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, _, err := AuthenticatedSession(tt.ctx)
			require.ErrorIs(t, err, authdomain.ErrMissingSession,
				"err = %v, want ErrMissingSession", err)

		})
	}
}

func TestAuthenticatedUserIDRejectsInvalidUser(t *testing.T) {
	_, err := AuthenticatedUserID(commonauth.WithUserID(context.Background(), "not-a-uuid"))
	require.ErrorIs(t, err, authdomain.ErrMissingSession,
		"err = %v, want ErrMissingSession", err)

}
