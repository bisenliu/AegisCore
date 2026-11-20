package authctx

import (
	"context"
	"uuid"

	platformauth "github.com/aegiscore/platform/security/auth"
	authdomain "github.com/aegiscore/user-service/internal/features/auth/domain"
)

// AuthenticatedSession 从 ctx 读取已认证用户和会话标识。
func AuthenticatedSession(ctx context.Context) (uuid.UUID, string, error) {
	userIDString, ok := platformauth.UserIDFromContext(ctx)
	if !ok {
		return uuid.Nil(), "", authdomain.ErrMissingSession
	}
	parsedUserID, err := uuid.Parse(userIDString)
	if err != nil {
		return uuid.Nil(), "", authdomain.ErrMissingSession
	}
	sessionID, ok := platformauth.SessionIDFromContext(ctx)
	if !ok {
		return uuid.Nil(), "", authdomain.ErrMissingSession
	}
	return parsedUserID, sessionID, nil
}

// AuthenticatedUserID 从 ctx 读取已认证用户标识。
func AuthenticatedUserID(ctx context.Context) (uuid.UUID, error) {
	userIDString, ok := platformauth.UserIDFromContext(ctx)
	if !ok {
		return uuid.Nil(), authdomain.ErrMissingSession
	}
	parsedUserID, err := uuid.Parse(userIDString)
	if err != nil {
		return uuid.Nil(), authdomain.ErrMissingSession
	}
	return parsedUserID, nil
}
