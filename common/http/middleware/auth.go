package middleware

import (
	"errors"

	"github.com/gin-gonic/gin"
	jwtv5 "github.com/golang-jwt/jwt/v5"
	"go.uber.org/zap"

	contracterrors "github.com/aegiscore/common/contract/errors"
	contractresponse "github.com/aegiscore/common/contract/response"
	"github.com/aegiscore/common/http/response"
	"github.com/aegiscore/common/runtime/logger"
	"github.com/aegiscore/common/security/auth"
)

// AuthWithTokenVersionValidator 返回支持可选 token version 校验的 JWT 认证中间件。
func AuthWithTokenVersionValidator(log *zap.Logger, jwtService *auth.JWTService, validator auth.TokenVersionValidator) gin.HandlerFunc {
	return func(c *gin.Context) {
		ctx := c.Request.Context()
		reqLog := logger.WithContext(ctx, log)
		authHeader := c.GetHeader(auth.AuthorizationHeader)
		if authHeader == "" {
			// 缺少请求头表示调用方未认证；下面的 Bearer 格式错误则属于 token 无效。
			reqLog.Info("missing authorization header", authFailureLogFields(c)...)
			response.Unauthenticated(c, contractresponse.MessageAuthInvalid)
			c.Abort()
			return
		}
		tokenString, err := auth.ParseBearerAuthorization(authHeader)
		if errors.Is(err, auth.ErrMissingBearerPrefix) {
			reqLog.Warn("invalid authorization header format", authFailureLogFields(c)...)
			response.TokenInvalid(c, contractresponse.MessageAuthInvalid)
			c.Abort()
			return
		}
		if errors.Is(err, auth.ErrEmptyBearerToken) {
			reqLog.Warn("empty bearer token", authFailureLogFields(c)...)
			response.TokenInvalid(c, contractresponse.MessageAuthInvalid)
			c.Abort()
			return
		}
		if err != nil {
			fields := append(authFailureLogFields(c), zap.Error(err))
			reqLog.Warn("invalid authorization header format", fields...)
			response.TokenInvalid(c, contractresponse.MessageAuthInvalid)
			c.Abort()
			return
		}

		claims, err := jwtService.ParseToken(tokenString)
		if err != nil {
			fields := append(authFailureLogFields(c), zap.Error(err))
			if errors.Is(err, auth.ErrMissingSecret) {
				reqLog.Error("token validation failed", fields...)
			} else {
				reqLog.Warn("token validation failed", fields...)
			}
			if errors.Is(err, jwtv5.ErrTokenExpired) {
				response.TokenExpired(c, contractresponse.MessageAuthInvalid)
			} else {
				response.TokenInvalid(c, contractresponse.MessageAuthInvalid)
			}
			c.Abort()
			return
		}

		if validator != nil {
			if err := validator.ValidateTokenVersion(ctx, claims.UserID, claims.TokenVersion); err != nil {
				if errors.Is(err, auth.ErrTokenVersionMismatch) {
					fields := append(authFailureLogFields(c), zap.String("user_id", claims.UserID), zap.Error(err))
					var mismatch *auth.TokenVersionMismatchError
					if errors.As(err, &mismatch) {
						fields = append(fields, zap.Int64("current_token_version", mismatch.Current), zap.Int64("token_version", mismatch.Token))
					}
					reqLog.Warn("token version mismatch", fields...)
					response.TokenInvalid(c, contractresponse.MessageAuthInvalid)
				} else {
					fields := append(authFailureLogFields(c), zap.String("user_id", claims.UserID), zap.Error(err))
					reqLog.Error("token version validation failed", fields...)
					response.Fail(c, contracterrors.InternalError(err))
				}
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
