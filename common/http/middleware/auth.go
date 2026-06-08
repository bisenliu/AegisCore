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

type TokenVersionValidator interface {
	ValidateTokenVersion(ctx context.Context, userID string, tokenVersion int64) error
}

type TokenVersionValidatorFunc func(ctx context.Context, userID string, tokenVersion int64) error

func (f TokenVersionValidatorFunc) ValidateTokenVersion(ctx context.Context, userID string, tokenVersion int64) error {
	return f(ctx, userID, tokenVersion)
}

func Auth(log *zap.Logger, jwtService *auth.JWTService, cfg config.AuthConfig) gin.HandlerFunc {
	return AuthWithTokenVersionValidator(log, jwtService, cfg, nil)
}

func AuthWithTokenVersionValidator(log *zap.Logger, jwtService *auth.JWTService, cfg config.AuthConfig, validator TokenVersionValidator) gin.HandlerFunc {
	return func(c *gin.Context) {
		ctx := c.Request.Context()
		reqLog := logger.WithContext(log, ctx)
		authHeader := c.GetHeader(auth.AuthorizationHeader)
		if authHeader == "" {
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
		c.Set(auth.UserIDKey, claims.UserID)
		c.Set(auth.SessionIDKey, claims.SessionID)
		c.Next()
	}
}
