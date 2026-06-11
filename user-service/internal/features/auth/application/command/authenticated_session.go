package command

import (
	"context"

	commonauth "github.com/aegiscore/common/security/auth"
	authdomain "github.com/aegiscore/user-service/internal/features/auth/domain"
	"github.com/google/uuid"
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
