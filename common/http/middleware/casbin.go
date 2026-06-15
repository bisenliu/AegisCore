package middleware

import (
	"context"
	"errors"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"

	contracterrors "github.com/aegiscore/common/contract/errors"
	commonresponse "github.com/aegiscore/common/http/response"
	commoncasbin "github.com/aegiscore/common/security/casbin"
)

const casbinAuthorizationWildcardMethod = "*"

// CasbinAuthorizer 定义 HTTP 中间件依赖的通用授权能力。
type CasbinAuthorizer interface {
	Authorize(ctx context.Context, req commoncasbin.Request) error
}

// CasbinRequestResolver 从 Gin 请求上下文解析 Casbin 授权请求。
type CasbinRequestResolver func(c *gin.Context) (commoncasbin.Request, error)

// CasbinErrorHandler 由调用方决定授权失败响应。
type CasbinErrorHandler func(c *gin.Context, err error)

// CasbinAuthorizationWhitelistRule 描述一条跳过 Casbin 授权的显式路由模板规则。
type CasbinAuthorizationWhitelistRule struct {
	Method       string
	PathTemplate string
}

// CasbinAuthorizationOption 配置通用 Casbin HTTP 授权中间件。
type CasbinAuthorizationOption func(*casbinAuthorizationConfig)

type casbinAuthorizationConfig struct {
	whitelist map[CasbinAuthorizationWhitelistRule]struct{}
	onError   CasbinErrorHandler
}

// WithCasbinAuthorizationWhitelist 配置按 HTTP 方法和 Gin 路由模板匹配的授权白名单。
func WithCasbinAuthorizationWhitelist(rules ...CasbinAuthorizationWhitelistRule) CasbinAuthorizationOption {
	return func(cfg *casbinAuthorizationConfig) {
		for _, rule := range rules {
			if rule.PathTemplate == "" {
				continue
			}
			cfg.whitelist[normalizeCasbinAuthorizationRule(rule)] = struct{}{}
		}
	}
}

// WithCasbinAuthorizationErrorHandler 配置授权失败响应处理函数。
func WithCasbinAuthorizationErrorHandler(handler CasbinErrorHandler) CasbinAuthorizationOption {
	return func(cfg *casbinAuthorizationConfig) {
		cfg.onError = handler
	}
}

// CasbinAuthorization 返回基于 resolver 和 authorizer 的 Gin 授权中间件。
func CasbinAuthorization(authorizer CasbinAuthorizer, resolver CasbinRequestResolver, opts ...CasbinAuthorizationOption) gin.HandlerFunc {
	cfg := casbinAuthorizationConfig{whitelist: make(map[CasbinAuthorizationWhitelistRule]struct{})}
	for _, opt := range opts {
		opt(&cfg)
	}
	return func(c *gin.Context) {
		if c.Request.Method == http.MethodOptions {
			c.Next()
			return
		}
		pathTemplate := c.FullPath()
		if pathTemplate != "" && cfg.isWhitelisted(c.Request.Method, pathTemplate) {
			c.Next()
			return
		}
		if authorizer == nil || resolver == nil {
			abortCasbinAuthorization(c, cfg.onError, commoncasbin.ErrNotConfigured)
			return
		}
		req, err := resolver(c)
		if err != nil {
			abortCasbinAuthorization(c, cfg.onError, err)
			return
		}
		if err := authorizer.Authorize(c.Request.Context(), req); err != nil {
			abortCasbinAuthorization(c, cfg.onError, err)
			return
		}
		c.Next()
	}
}

func (cfg casbinAuthorizationConfig) isWhitelisted(method string, pathTemplate string) bool {
	method = strings.ToUpper(method)
	_, exact := cfg.whitelist[CasbinAuthorizationWhitelistRule{Method: method, PathTemplate: pathTemplate}]
	if exact {
		return true
	}
	_, wildcard := cfg.whitelist[CasbinAuthorizationWhitelistRule{Method: casbinAuthorizationWildcardMethod, PathTemplate: pathTemplate}]
	return wildcard
}

func abortCasbinAuthorization(c *gin.Context, onError CasbinErrorHandler, err error) {
	if onError != nil {
		onError(c, err)
		c.Abort()
		return
	}
	if errors.Is(err, commoncasbin.ErrDenied) {
		commonresponse.Forbidden(c, "permission denied")
	} else {
		commonresponse.Fail(c, contracterrors.InternalError(err))
	}
	c.Abort()
}

func normalizeCasbinAuthorizationRule(rule CasbinAuthorizationWhitelistRule) CasbinAuthorizationWhitelistRule {
	method := strings.ToUpper(rule.Method)
	if method == "" {
		method = casbinAuthorizationWildcardMethod
	}
	return CasbinAuthorizationWhitelistRule{Method: method, PathTemplate: rule.PathTemplate}
}
