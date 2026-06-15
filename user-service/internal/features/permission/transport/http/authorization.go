package permissionhttp

import (
	"net/http"
	"strings"

	contracterrors "github.com/aegiscore/common/contract/errors"
	contractresponse "github.com/aegiscore/common/contract/response"
	commonresponse "github.com/aegiscore/common/http/response"
	commonauth "github.com/aegiscore/common/security/auth"
	"github.com/aegiscore/user-service/internal/features/permission/application/authorization"
	"github.com/gin-gonic/gin"
)

const authorizationWildcardMethod = "*"

// AuthorizationWhitelistRule 描述一条跳过 RBAC 的显式路由模板规则。
type AuthorizationWhitelistRule struct {
	Method       string
	PathTemplate string
}

// AuthorizationOption 配置 RBAC 授权中间件。
type AuthorizationOption func(*authorizationConfig)

type authorizationConfig struct {
	whitelist map[AuthorizationWhitelistRule]struct{}
}

// WithAuthorizationWhitelist 配置按 HTTP 方法和 Gin 路由模板匹配的授权白名单。
func WithAuthorizationWhitelist(rules ...AuthorizationWhitelistRule) AuthorizationOption {
	return func(cfg *authorizationConfig) {
		for _, rule := range rules {
			if rule.PathTemplate == "" {
				continue
			}
			cfg.whitelist[normalizeAuthorizationRule(rule)] = struct{}{}
		}
	}
}

// Authorize 返回在 JWT 认证之后执行的 Gin RBAC 授权中间件。
func Authorize(authz authorization.Authorizer, opts ...AuthorizationOption) gin.HandlerFunc {
	cfg := authorizationConfig{whitelist: make(map[AuthorizationWhitelistRule]struct{})}
	for _, opt := range opts {
		opt(&cfg)
	}
	return func(c *gin.Context) {
		if c.Request.Method == http.MethodOptions {
			c.Next()
			return
		}

		pathTemplate := c.FullPath()
		if pathTemplate == "" {
			commonresponse.Forbidden(c, "permission denied")
			c.Abort()
			return
		}
		if cfg.isWhitelisted(c.Request.Method, pathTemplate) {
			c.Next()
			return
		}

		userID, ok := authenticatedUserID(c)
		if !ok {
			commonresponse.Unauthenticated(c, contractresponse.MessageAuthInvalid)
			c.Abort()
			return
		}

		allowed, err := authz.Enforce(c.Request.Context(), userID, pathTemplate, c.Request.Method)
		if err != nil {
			commonresponse.Fail(c, contracterrors.InternalError(err))
			c.Abort()
			return
		}
		if !allowed {
			commonresponse.Forbidden(c, "permission denied")
			c.Abort()
			return
		}
		c.Next()
	}
}

func (cfg authorizationConfig) isWhitelisted(method string, pathTemplate string) bool {
	method = strings.ToUpper(method)
	_, exact := cfg.whitelist[AuthorizationWhitelistRule{Method: method, PathTemplate: pathTemplate}]
	if exact {
		return true
	}
	_, wildcard := cfg.whitelist[AuthorizationWhitelistRule{Method: authorizationWildcardMethod, PathTemplate: pathTemplate}]
	return wildcard
}

func authenticatedUserID(c *gin.Context) (string, bool) {
	value, ok := c.Get(commonauth.UserIDKey)
	if ok {
		userID, ok := value.(string)
		return userID, ok && userID != ""
	}
	return commonauth.UserIDFromContext(c.Request.Context())
}

func normalizeAuthorizationRule(rule AuthorizationWhitelistRule) AuthorizationWhitelistRule {
	method := strings.ToUpper(rule.Method)
	if method == "" {
		method = authorizationWildcardMethod
	}
	return AuthorizationWhitelistRule{Method: method, PathTemplate: rule.PathTemplate}
}
