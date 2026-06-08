package middleware

import (
	"context"
	"errors"

	"github.com/aegiscore/common/contract/response"
	"github.com/aegiscore/common/runtime/config"
	"github.com/aegiscore/common/runtime/logger"
	"github.com/aegiscore/common/security/auth"
	"github.com/gin-gonic/gin"
	jwtv5 "github.com/golang-jwt/jwt/v5"
	"go.uber.org/zap"
)

// TokenVersionValidator 校验 token version，使中间件可以拒绝已撤销或过期状态的 access token。
type TokenVersionValidator interface {
	ValidateTokenVersion(ctx context.Context, userID string, tokenVersion int64) error
}

// TokenVersionValidatorFunc 将函数适配为 TokenVersionValidator 接口。
type TokenVersionValidatorFunc func(ctx context.Context, userID string, tokenVersion int64) error

// ValidateTokenVersion 将 token version 校验委托给 f。
func (f TokenVersionValidatorFunc) ValidateTokenVersion(ctx context.Context, userID string, tokenVersion int64) error {
	return f(ctx, userID, tokenVersion)
}

// Auth 返回不执行 token version 撤销校验的 JWT 认证中间件。
func Auth(log *zap.Logger, jwtService *auth.JWTService, cfg config.AuthConfig) gin.HandlerFunc {
	return AuthWithTokenVersionValidator(log, jwtService, cfg, nil)
}

// AuthWithTokenVersionValidator 返回支持可选 token version 校验的 JWT 认证中间件。
func AuthWithTokenVersionValidator(log *zap.Logger, jwtService *auth.JWTService, cfg config.AuthConfig, validator TokenVersionValidator) gin.HandlerFunc {
	return func(c *gin.Context) {
		ctx := c.Request.Context()
		reqLog := logger.WithContext(log, ctx)
		authHeader := c.GetHeader(auth.AuthorizationHeader)
		if authHeader == "" {
			// 缺少请求头表示调用方未认证；下面的 Bearer 格式错误则属于 token 无效。
			reqLog.Error("missing authorization header")
			response.Unauthenticated(c, response.MessageAuthInvalid)
			c.Abort()
			return
		}
		tokenString, err := auth.ParseBearerAuthorization(authHeader)
		if errors.Is(err, auth.ErrMissingBearerPrefix) {
			reqLog.Error("invalid authorization header format")
			response.TokenInvalid(c, response.MessageAuthInvalid)
			c.Abort()
			return
		}
		if errors.Is(err, auth.ErrEmptyBearerToken) {
			reqLog.Error("empty bearer token")
			response.TokenInvalid(c, response.MessageAuthInvalid)
			c.Abort()
			return
		}
		if err != nil {
			reqLog.Error("invalid authorization header format", zap.Error(err))
			response.TokenInvalid(c, response.MessageAuthInvalid)
			c.Abort()
			return
		}

		claims, err := jwtService.ParseToken(tokenString)
		if err != nil {
			reqLog.Error("token validation failed", zap.Error(err))
			if errors.Is(err, jwtv5.ErrTokenExpired) {
				response.TokenExpired(c, response.MessageAuthInvalid)
			} else {
				response.TokenInvalid(c, response.MessageAuthInvalid)
			}
			c.Abort()
			return
		}

		if validator != nil {
			if err := validator.ValidateTokenVersion(ctx, claims.UserID, claims.TokenVersion); err != nil {
				reqLog.Error("token version validation failed", zap.Error(err))
				response.TokenInvalid(c, response.MessageAuthInvalid)
				c.Abort()
				return
			}
		}

		ctx = auth.WithUserID(ctx, claims.UserID)
		ctx = auth.WithSessionID(ctx, claims.SessionID)
		c.Request = c.Request.WithContext(ctx)
		// 同时写入两种 context，因为下游代码可能读取 net/http context 或 Gin helper。
		c.Set(auth.UserIDKey, claims.UserID)
		c.Set(auth.SessionIDKey, claims.SessionID)
		c.Next()
	}
}
