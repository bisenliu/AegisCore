package validators

import (
	"errors"

	commonauth "github.com/aegiscore/common/security/auth"
	authdomain "github.com/aegiscore/user-service/internal/features/auth/domain"
)

// ValidateTokenVersionMatch 确认 token 或 session version 仍为当前版本。
func ValidateTokenVersionMatch(currentVersion int64, tokenVersion int64) error {
	if currentVersion != tokenVersion {
		return errors.Join(authdomain.ErrTokenInvalid, commonauth.ErrTokenVersionMismatch)
	}
	return nil
}

// ValidateRefreshSessionClaims 确认持久化 refresh session 元数据与 token claims 匹配。
func ValidateRefreshSessionClaims(session authdomain.AuthSession, userID string, tokenVersion int64) error {
	if session.UserID != userID || session.TokenVersion != tokenVersion {
		return errors.Join(authdomain.ErrTokenInvalid, authdomain.ErrAuthSessionMismatch)
	}
	return nil
}
