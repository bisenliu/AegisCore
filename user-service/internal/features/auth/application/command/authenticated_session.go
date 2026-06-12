package command

import (
	"context"

	"github.com/google/uuid"

	commonauth "github.com/aegiscore/common/security/auth"
	authdomain "github.com/aegiscore/user-service/internal/features/auth/domain"
)

func authenticatedSession(ctx context.Context) (string, string, error) {
	userIDString, ok := commonauth.UserIDFromContext(ctx)
	if !ok {
		return "", "", authdomain.ErrMissingSession
	}
	if _, err := uuid.Parse(userIDString); err != nil {
		return "", "", authdomain.ErrMissingSession
	}
	sessionID, ok := commonauth.SessionIDFromContext(ctx)
	if !ok {
		return "", "", authdomain.ErrMissingSession
	}
	return userIDString, sessionID, nil
}
