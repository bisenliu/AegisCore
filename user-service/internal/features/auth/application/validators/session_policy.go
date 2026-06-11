package validators

import authdomain "github.com/aegiscore/user-service/internal/features/auth/domain"

// ValidateTokenVersionMatch confirms a token/session version is still current.
func ValidateTokenVersionMatch(currentVersion int64, tokenVersion int64) error {
	if currentVersion != tokenVersion {
		return authdomain.ErrTokenInvalid
	}
	return nil
}

// ValidateRefreshSessionClaims confirms persisted refresh session metadata matches token claims.
func ValidateRefreshSessionClaims(session authdomain.AuthSession, userID string, tokenVersion int64) error {
	if session.UserID != userID || session.TokenVersion != tokenVersion {
		return authdomain.ErrTokenInvalid
	}
	return nil
}
