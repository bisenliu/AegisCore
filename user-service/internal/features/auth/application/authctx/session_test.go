package authctx

import (
	"context"
	"errors"
	"testing"

	commonauth "github.com/aegiscore/common/security/auth"
	authdomain "github.com/aegiscore/user-service/internal/features/auth/domain"
)

func TestAuthenticatedSessionReturnsUserAndSession(t *testing.T) {
	ctx := commonauth.WithSessionID(commonauth.WithUserID(context.Background(), "018f6f3e-7c4d-7b2a-9f8a-4f6b1b2c3d4e"), "s-123")

	userID, sessionID, err := AuthenticatedSession(ctx)

	if err != nil {
		t.Fatalf("AuthenticatedSession: %v", err)
	}
	if userID != "018f6f3e-7c4d-7b2a-9f8a-4f6b1b2c3d4e" || sessionID != "s-123" {
		t.Fatalf("userID=%q sessionID=%q", userID, sessionID)
	}
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

			if !errors.Is(err, authdomain.ErrMissingSession) {
				t.Fatalf("err = %v, want ErrMissingSession", err)
			}
		})
	}
}

func TestAuthenticatedUserIDRejectsInvalidUser(t *testing.T) {
	_, err := AuthenticatedUserID(commonauth.WithUserID(context.Background(), "not-a-uuid"))

	if !errors.Is(err, authdomain.ErrMissingSession) {
		t.Fatalf("err = %v, want ErrMissingSession", err)
	}
}
