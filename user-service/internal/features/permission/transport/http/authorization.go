package permissionhttp

import (
	"context"
	"errors"

	"github.com/gin-gonic/gin"

	contracterrors "github.com/aegiscore/common/contract/errors"
	contractresponse "github.com/aegiscore/common/contract/response"
	commonmiddleware "github.com/aegiscore/common/http/middleware"
	commonresponse "github.com/aegiscore/common/http/response"
	commonauth "github.com/aegiscore/common/security/auth"
	commoncasbin "github.com/aegiscore/common/security/casbin"
	"github.com/aegiscore/user-service/internal/features/permission/application/authorization"
)

var errAuthorizationUnauthenticated = errors.New("authorization user is not authenticated")

// AuthorizationWhitelistRule 描述一条跳过 RBAC 的显式路由模板规则。
type AuthorizationWhitelistRule = commonmiddleware.CasbinAuthorizationWhitelistRule

// AuthorizationOption 配置 RBAC 授权中间件。
type AuthorizationOption = commonmiddleware.CasbinAuthorizationOption

type authorizationAdapter struct {
	authz authorization.Authorizer
}

// WithAuthorizationWhitelist 配置按 HTTP 方法和 Gin 路由模板匹配的授权白名单。
func WithAuthorizationWhitelist(rules ...AuthorizationWhitelistRule) AuthorizationOption {
	return commonmiddleware.WithCasbinAuthorizationWhitelist(rules...)
}

// Authorize 返回在 JWT 认证之后执行的 Gin RBAC 授权中间件。
func Authorize(authz authorization.Authorizer, opts ...AuthorizationOption) gin.HandlerFunc {
	options := append([]AuthorizationOption{
		commonmiddleware.WithCasbinAuthorizationErrorHandler(handleAuthorizationError),
	}, opts...)
	return commonmiddleware.CasbinAuthorization(authorizationAdapter{authz: authz}, resolveAuthorizationRequest, options...)
}

func authenticatedUserID(c *gin.Context) (string, bool) {
	value, ok := c.Get(commonauth.UserIDKey)
	if ok {
		userID, ok := value.(string)
		return userID, ok && userID != ""
	}
	return commonauth.UserIDFromContext(c.Request.Context())
}

func (a authorizationAdapter) Authorize(ctx context.Context, req commoncasbin.Request) error {
	if a.authz == nil {
		return commoncasbin.ErrNotConfigured
	}
	allowed, err := a.authz.Enforce(ctx, req.Subject, req.Object, req.Action)
	if err != nil {
		if errors.Is(err, authorization.ErrInvalidSubjectUserID) {
			return errAuthorizationUnauthenticated
		}
		return err
	}
	if !allowed {
		return commoncasbin.ErrDenied
	}
	return nil
}

func resolveAuthorizationRequest(c *gin.Context) (commoncasbin.Request, error) {
	pathTemplate := c.FullPath()
	if pathTemplate == "" {
		return commoncasbin.Request{}, commoncasbin.ErrDenied
	}
	userID, ok := authenticatedUserID(c)
	if !ok {
		return commoncasbin.Request{}, errAuthorizationUnauthenticated
	}
	return commoncasbin.Request{Subject: userID, Object: pathTemplate, Action: c.Request.Method}, nil
}

func handleAuthorizationError(c *gin.Context, err error) {
	if errors.Is(err, errAuthorizationUnauthenticated) {
		commonresponse.Unauthenticated(c, contractresponse.MessageAuthInvalid)
		return
	}
	if errors.Is(err, commoncasbin.ErrDenied) {
		commonresponse.Forbidden(c, "permission denied")
		return
	}
	commonresponse.Fail(c, contracterrors.InternalError(err))
}
