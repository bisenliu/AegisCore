package contextutil

import "context"

const (
	AuthorizationHeader = "Authorization"
	TokenPrefix         = "Bearer "
	UserIDKey           = "user_id"
)

type userIDContextKey struct{}

func WithUserID(ctx context.Context, userID string) context.Context {
	return context.WithValue(ctx, userIDContextKey{}, userID)
}

func UserIDFromContext(ctx context.Context) (string, bool) {
	if ctx == nil {
		return "", false
	}
	userID, ok := ctx.Value(userIDContextKey{}).(string)
	return userID, ok && userID != ""
}
