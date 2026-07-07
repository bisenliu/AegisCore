package validators

import (
	"testing"

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
	session := authdomain.AuthSession{UserID: "user-1", SessionID: "session-1", TokenVersion: 2}

	tests := []struct {
		name         string
		userID       string
		tokenVersion int64
		wantErr      error
		wantSpecific error
	}{
		{name: "matching", userID: "user-1", tokenVersion: 2},
		{name: "user mismatch", userID: "user-2", tokenVersion: 2, wantErr: authdomain.ErrTokenInvalid, wantSpecific: authdomain.ErrAuthSessionMismatch},
		{name: "version mismatch", userID: "user-1", tokenVersion: 3, wantErr: authdomain.ErrTokenInvalid, wantSpecific: authdomain.ErrAuthSessionMismatch},
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
