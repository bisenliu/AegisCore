package credentials

import "context"

const (
	AuthorizationHeader = "Authorization"
	TokenTypeBearer     = "Bearer"
	TokenPrefix         = TokenTypeBearer + " "
	UserIDKey           = "user_id"
	SessionIDKey        = "session_id"
)

type userIDContextKey struct{}
type sessionIDContextKey struct{}

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

func WithSessionID(ctx context.Context, sessionID string) context.Context {
	return context.WithValue(ctx, sessionIDContextKey{}, sessionID)
}

func SessionIDFromContext(ctx context.Context) (string, bool) {
	if ctx == nil {
		return "", false
	}
	sessionID, ok := ctx.Value(sessionIDContextKey{}).(string)
	return sessionID, ok && sessionID != ""
}
