package validators

import (
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	authdomain "github.com/aegiscore/user-service/internal/features/auth/domain"
)

func TestValidateTokenVersionMatch(t *testing.T) {
	tests := []struct {
		name           string
		currentVersion int64
		tokenVersion   int64
		wantErr        error
	}{
		{name: "matching", currentVersion: 2, tokenVersion: 2},
		{name: "stale", currentVersion: 3, tokenVersion: 2, wantErr: authdomain.ErrTokenInvalid},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateTokenVersionMatch(tt.currentVersion, tt.tokenVersion)
			require.ErrorIs(t, err, tt.wantErr,
				"err = %v, want %v", err, tt.wantErr)

		})
	}
}

func TestValidateRefreshSessionClaims(t *testing.T) {
	userID := uuid.MustParse("018f6f3e-7c4d-7b2a-9f8a-4f6b1b2c3d4e")
	otherUserID := uuid.MustParse("018f6f3e-7c4d-7b2a-9f8a-4f6b1b2c3d4f")
	session := authdomain.AuthSession{UserID: userID, SessionID: "session-1", TokenVersion: 2}

	tests := []struct {
		name         string
		userID       uuid.UUID
		tokenVersion int64
		wantErr      error
		wantSpecific error
	}{
		{name: "matching", userID: userID, tokenVersion: 2},
		{name: "user mismatch", userID: otherUserID, tokenVersion: 2, wantErr: authdomain.ErrTokenInvalid, wantSpecific: authdomain.ErrAuthSessionMismatch},
		{name: "version mismatch", userID: userID, tokenVersion: 3, wantErr: authdomain.ErrTokenInvalid, wantSpecific: authdomain.ErrAuthSessionMismatch},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateRefreshSessionClaims(session, tt.userID, tt.tokenVersion)
			require.ErrorIs(t, err, tt.wantErr,
				"err = %v, want %v", err, tt.wantErr)
			if tt.wantSpecific != nil {
				require.ErrorIs(t, err, tt.wantSpecific,
					"err = %v, want %v", err, tt.wantSpecific)
			}

		})
	}
}
