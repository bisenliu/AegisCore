package middleware

import (
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

const unauthenticatedMessage = "登录状态无效或已过期，请重新登录"

func Auth(jwtService *commonjwt.Service, cfg config.AuthConfig) gin.HandlerFunc {
	return func(c *gin.Context) {
		ctx := c.Request.Context()
		if isWhitelistedPath(c.Request.URL.Path, cfg.Whitelist) {
			logger.Debug(ctx, "skipping authentication for whitelisted path", zap.String("path", c.Request.URL.Path))
			c.Next()
			return
		}

		authHeader := c.GetHeader(contextutil.AuthorizationHeader)
		if authHeader == "" {
			logger.Error(ctx, "missing authorization header")
			response.Unauthenticated(c, unauthenticatedMessage)
			c.Abort()
			return
		}
		if !strings.HasPrefix(authHeader, contextutil.TokenPrefix) {
			logger.Error(ctx, "invalid authorization header format")
			response.TokenInvalid(c, unauthenticatedMessage)
			c.Abort()
			return
		}

		tokenString := strings.TrimSpace(strings.TrimPrefix(authHeader, contextutil.TokenPrefix))
		if tokenString == "" {
			logger.Error(ctx, "empty bearer token")
			response.TokenInvalid(c, unauthenticatedMessage)
			c.Abort()
			return
		}

		claims, err := jwtService.ParseToken(tokenString)
		if err != nil {
			logger.Error(ctx, "token validation failed", zap.Error(err))
			if errors.Is(err, jwtv5.ErrTokenExpired) {
				response.TokenExpired(c, unauthenticatedMessage)
			} else {
				response.TokenInvalid(c, unauthenticatedMessage)
			}
			c.Abort()
			return
		}

		ctx = contextutil.WithUserID(ctx, claims.UserID)
		c.Request = c.Request.WithContext(ctx)
		c.Set(contextutil.UserIDKey, claims.UserID)
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
