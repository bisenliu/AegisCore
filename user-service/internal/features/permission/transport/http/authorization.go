package permissionhttp

import (
	"context"
	"errors"

	"github.com/gin-gonic/gin"

	contracterrors "github.com/aegiscore/platform/contract/errors"
	contractresponse "github.com/aegiscore/platform/contract/response"
	platformmiddleware "github.com/aegiscore/platform/http/middleware"
	platformresponse "github.com/aegiscore/platform/http/response"
	platformroute "github.com/aegiscore/platform/http/route"
	platformauth "github.com/aegiscore/platform/security/auth"
	platformcasbin "github.com/aegiscore/platform/security/casbin"
	"github.com/aegiscore/user-service/internal/features/permission/application/authorization"
)

var errAuthorizationUnauthenticated = errors.New("authorization user is not authenticated")

type authorizationAdapter struct {
	authz authorization.Authorizer
}

// Authorize 创建接口权限校验拦截器（必须在用户登录认证通过之后执行），用于检查当前登录用户是否有权访问所请求的接口。
func Authorize(authz authorization.Authorizer, opts ...platformmiddleware.CasbinAuthorizationOption) gin.HandlerFunc {
	options := append([]platformmiddleware.CasbinAuthorizationOption{
		platformmiddleware.WithCasbinAuthorizationErrorHandler(handleAuthorizationError),
	}, opts...)
	return platformmiddleware.CasbinAuthorization(authorizationAdapter{authz: authz}, resolveAuthorizationRequest, options...)
}

// authenticatedUserID 获取当前请求中已经通过登录验证的用户编号；未登录时返回未登录状态。
func authenticatedUserID(c *gin.Context) (string, bool) {
	value, ok := c.Get(platformauth.UserIDKey)
	if ok {
		userID, ok := value.(string)
		return userID, ok && userID != ""
	}
	return platformauth.UserIDFromContext(c.Request.Context())
}

// Authorize 核心权限检查入口：核对当前已登录的用户是否有权访问所请求的接口，如果无权访问则直接拦截并返回无权限提示。
func (a authorizationAdapter) Authorize(ctx context.Context, req platformcasbin.Request) error {
	if a.authz == nil {
		return platformcasbin.ErrNotConfigured
	}
	allowed, err := a.authz.Enforce(ctx, req.Subject, req.Object, req.Action)
	if err != nil {
		if errors.Is(err, authorization.ErrInvalidSubjectUserID) {
			return errAuthorizationUnauthenticated
		}
		return err
	}
	if !allowed {
		return platformcasbin.ErrDenied
	}
	return nil
}

// resolveAuthorizationRequest 从当前请求中提取出“谁（用户）想对什么功能（接口地址）做什么操作（GET/POST等）”，提供给权限判定器使用。
func resolveAuthorizationRequest(c *gin.Context) (platformcasbin.Request, error) {
	pathTemplate := platformroute.TemplateOrUnmatched(c)
	if pathTemplate == platformroute.Unmatched {
		return platformcasbin.Request{}, platformcasbin.ErrDenied
	}
	userID, ok := authenticatedUserID(c)
	if !ok {
		return platformcasbin.Request{}, errAuthorizationUnauthenticated
	}
	return platformcasbin.Request{Subject: userID, Object: pathTemplate, Action: c.Request.Method}, nil
}

// handleAuthorizationError 当用户未登录或权限不足时，向客户端返回得体的错误提示，不暴露系统内部的判定规则细节。
func handleAuthorizationError(c *gin.Context, err error) {
	if errors.Is(err, errAuthorizationUnauthenticated) {
		platformresponse.Unauthenticated(c, contractresponse.MessageAuthInvalid)
		return
	}
	if errors.Is(err, platformcasbin.ErrDenied) {
		platformresponse.Forbidden(c, "permission denied")
		return
	}
	platformresponse.Fail(c, contracterrors.InternalError(err))
}
