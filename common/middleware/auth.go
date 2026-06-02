package middleware

import (
	"context"
	"errors"
	"strings"

	"github.com/aegiscore/common/config"
	"github.com/aegiscore/common/contextutil"
	commonjwt "github.com/aegiscore/common/jwt"
	"github.com/aegiscore/common/logger"
	"github.com/aegiscore/common/response"
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

func Auth(log *zap.Logger, jwtService *commonjwt.Service, cfg config.AuthConfig) gin.HandlerFunc {
	return AuthWithTokenVersionValidator(log, jwtService, cfg, nil)
}

func AuthWithTokenVersionValidator(log *zap.Logger, jwtService *commonjwt.Service, cfg config.AuthConfig, validator TokenVersionValidator) gin.HandlerFunc {
	return func(c *gin.Context) {
		ctx := c.Request.Context()
		reqLog := logger.WithContext(log, ctx)
		if isWhitelistedPath(c.Request.URL.Path, cfg.Whitelist) {
			reqLog.Debug("skipping authentication for whitelisted path", zap.String("path", c.Request.URL.Path))
			c.Next()
			return
		}

		authHeader := c.GetHeader(contextutil.AuthorizationHeader)
		if authHeader == "" {
			reqLog.Error("missing authorization header")
			response.Unauthenticated(c, response.MessageAuthInvalid)
			c.Abort()
			return
		}
		if !strings.HasPrefix(authHeader, contextutil.TokenPrefix) {
			reqLog.Error("invalid authorization header format")
			response.TokenInvalid(c, response.MessageAuthInvalid)
			c.Abort()
			return
		}

		tokenString := strings.TrimSpace(strings.TrimPrefix(authHeader, contextutil.TokenPrefix))
		if tokenString == "" {
			reqLog.Error("empty bearer token")
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

		ctx = contextutil.WithUserID(ctx, claims.UserID)
		ctx = contextutil.WithSessionID(ctx, claims.SessionID)
		c.Request = c.Request.WithContext(ctx)
		c.Set(contextutil.UserIDKey, claims.UserID)
		c.Set(contextutil.SessionIDKey, claims.SessionID)
		c.Next()
	}
}

func isWhitelistedPath(path string, whitelist []string) bool {
	for _, prefix := range whitelist {
		if prefix != "" && strings.HasPrefix(path, prefix) {
			return true
		}
	}
	return false
}
