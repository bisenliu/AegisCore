package authhttp

import (
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"

	contractresponse "github.com/aegiscore/platform/contract/response"
	"github.com/aegiscore/platform/http/response"
	"github.com/aegiscore/platform/runtime/logger"
	platformauth "github.com/aegiscore/platform/security/auth"
	authapplication "github.com/aegiscore/user-service/internal/features/auth/application"
	authdomain "github.com/aegiscore/user-service/internal/features/auth/domain"
)

// AccessTokenDenylist 在 token version 校验之后、RBAC 之前拒绝已退出的当前 access token。
// 拒绝名单属于 user-service 认证策略，不放入共享认证中间件。
func AccessTokenDenylist(log *zap.Logger, denylist authapplication.AccessTokenDenylist) gin.HandlerFunc {
	return func(c *gin.Context) {
		ctx := c.Request.Context()
		token, ok := platformauth.AccessTokenFromContext(ctx)
		if !ok {
			response.TokenInvalid(c, contractresponse.MessageAuthInvalid)
			c.Abort()
			return
		}
		denied, err := denylist.AccessTokenDenied(ctx, token.ID)
		if err != nil {
			// 无法确认是否已吊销时拒绝访问，避免 Redis 故障期间放行刚退出的 access token。
			logger.Error(ctx, "access token denylist unavailable", logger.StackTrace(zap.String("user_id", token.UserID), zap.Error(err))...)
			response.Fail(c, authdomain.ErrAccessDenylistUnavailable)
			c.Abort()
			return
		}
		if denied {
			logger.Warn(ctx, "access token denied", zap.String("user_id", token.UserID))
			response.TokenInvalid(c, contractresponse.MessageAuthInvalid)
			c.Abort()
			return
		}
		c.Next()
	}
}
