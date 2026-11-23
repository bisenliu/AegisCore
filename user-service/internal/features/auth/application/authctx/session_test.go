package authctx

import (
	"context"
	"testing"
	"time"
	"uuid"

	"github.com/stretchr/testify/require"

	platformauth "github.com/aegiscore/platform/security/auth"
	authdomain "github.com/aegiscore/user-service/internal/features/auth/domain"
)

func TestAuthenticatedSessionReturnsUserAndSession(t *testing.T) {
	ctx := platformauth.WithSessionID(platformauth.WithUserID(context.Background(), "018f6f3e-7c4d-7b2a-9f8a-4f6b1b2c3d4e"), "s-123")

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
		{name: "invalid user", ctx: platformauth.WithUserID(context.Background(), "not-a-uuid")},
		{name: "missing session", ctx: platformauth.WithUserID(context.Background(), "018f6f3e-7c4d-7b2a-9f8a-4f6b1b2c3d4e")},
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
	_, err := AuthenticatedUserID(platformauth.WithUserID(context.Background(), "not-a-uuid"))
	require.ErrorIs(t, err, authdomain.ErrMissingSession,
		"err = %v, want ErrMissingSession", err)

}

func TestAuthenticatedAccessToken(t *testing.T) {
	t.Run("returns valid access token", func(t *testing.T) {
		token := platformauth.AccessToken{ID: "jti-123", UserID: "018f6f3e-7c4d-7b2a-9f8a-4f6b1b2c3d4e", ExpiresAt: time.Now().Add(time.Minute)}
		ctx := platformauth.WithAccessToken(context.Background(), token)

		extracted, err := AuthenticatedAccessToken(ctx)
		require.NoError(t, err)
		require.Equal(t, "jti-123", extracted.ID)
	})

	t.Run("rejects missing or incomplete access token", func(t *testing.T) {
		tests := []struct {
			name string
			ctx  context.Context
		}{
			{name: "missing in context", ctx: context.Background()},
			{name: "missing token id", ctx: platformauth.WithAccessToken(context.Background(), platformauth.AccessToken{UserID: "018f6f3e-7c4d-7b2a-9f8a-4f6b1b2c3d4e", ExpiresAt: time.Now().Add(time.Minute)})},
			{name: "missing user id", ctx: platformauth.WithAccessToken(context.Background(), platformauth.AccessToken{ID: "jti-123", ExpiresAt: time.Now().Add(time.Minute)})},
			{name: "zero expires at", ctx: platformauth.WithAccessToken(context.Background(), platformauth.AccessToken{ID: "jti-123", UserID: "018f6f3e-7c4d-7b2a-9f8a-4f6b1b2c3d4e"})},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				_, err := AuthenticatedAccessToken(tt.ctx)
				require.ErrorIs(t, err, authdomain.ErrMissingSession)
			})
		}
	})
}
