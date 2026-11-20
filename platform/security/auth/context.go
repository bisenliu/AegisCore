package auth

import "context"

const (
	// AuthorizationHeader 是承载 Bearer 令牌的 HTTP 请求头。
	AuthorizationHeader = "Authorization"
	// TokenTypeBearer 是 Authorization 请求头使用的 OAuth2 Bearer 令牌类型。
	TokenTypeBearer = "Bearer"
	// TokenPrefix 是 Authorization 请求头中原始令牌前必须携带的前缀。
	TokenPrefix = TokenTypeBearer + " "
	// UserIDKey 是认证用户标识对外暴露为字段或请求值时使用的名称。
	UserIDKey = "user_id"
	// SessionIDKey 是认证会话标识对外暴露为字段或请求值时使用的名称。
	SessionIDKey = "session_id"
)

type userIDContextKey struct{}
type sessionIDContextKey struct{}

// WithUserID 返回携带认证用户 ID 的 context。
func WithUserID(ctx context.Context, userID string) context.Context {
	return context.WithValue(ctx, userIDContextKey{}, userID)
}

// UserIDFromContext 从 ctx 返回非空认证用户 ID，并标记其是否存在。
func UserIDFromContext(ctx context.Context) (string, bool) {
	if ctx == nil {
		return "", false
	}
	userID, ok := ctx.Value(userIDContextKey{}).(string)
	return userID, ok && userID != ""
}

// WithSessionID 返回携带认证会话 ID 的 context。
func WithSessionID(ctx context.Context, sessionID string) context.Context {
	return context.WithValue(ctx, sessionIDContextKey{}, sessionID)
}

// SessionIDFromContext 从 ctx 返回非空认证会话 ID，并标记其是否存在。
func SessionIDFromContext(ctx context.Context) (string, bool) {
	if ctx == nil {
		return "", false
	}
	sessionID, ok := ctx.Value(sessionIDContextKey{}).(string)
	return sessionID, ok && sessionID != ""
}
