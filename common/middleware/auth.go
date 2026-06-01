package middleware

import (
	"strings"

	"github.com/aegiscore/common/config"
	"github.com/aegiscore/common/contextutil"
	commonjwt "github.com/aegiscore/common/jwt"
	"github.com/aegiscore/common/logger"
	"github.com/aegiscore/common/response"
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

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
			response.Unauthenticated(c, "missing authorization header")
			c.Abort()
			return
		}
		if !strings.HasPrefix(authHeader, contextutil.TokenPrefix) {
			logger.Error(ctx, "invalid authorization header format")
			response.Unauthenticated(c, "invalid authorization header format")
			c.Abort()
			return
		}

		tokenString := strings.TrimSpace(strings.TrimPrefix(authHeader, contextutil.TokenPrefix))
		if tokenString == "" {
			logger.Error(ctx, "empty bearer token")
			response.Unauthenticated(c, "empty token")
			c.Abort()
			return
		}

		claims, err := jwtService.ParseToken(tokenString)
		if err != nil {
			logger.Error(ctx, "token validation failed", zap.Error(err))
			response.Unauthenticated(c, "invalid token")
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
