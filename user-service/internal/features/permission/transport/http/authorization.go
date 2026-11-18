package permissionhttp

import (
	"context"
	"errors"

	"github.com/gin-gonic/gin"

	contracterrors "github.com/aegiscore/common/contract/errors"
	contractresponse "github.com/aegiscore/common/contract/response"
	commonmiddleware "github.com/aegiscore/common/http/middleware"
	commonresponse "github.com/aegiscore/common/http/response"
	commonroute "github.com/aegiscore/common/http/route"
	commonauth "github.com/aegiscore/common/security/auth"
	commoncasbin "github.com/aegiscore/common/security/casbin"
	"github.com/aegiscore/user-service/internal/features/permission/application/authorization"
)

var errAuthorizationUnauthenticated = errors.New("authorization user is not authenticated")

type authorizationAdapter struct {
	authz authorization.Authorizer
}

// Authorize 创建接口权限校验拦截器（必须在用户登录认证通过之后执行），用于检查当前登录用户是否有权访问所请求的接口。
func Authorize(authz authorization.Authorizer, opts ...commonmiddleware.CasbinAuthorizationOption) gin.HandlerFunc {
	options := append([]commonmiddleware.CasbinAuthorizationOption{
		commonmiddleware.WithCasbinAuthorizationErrorHandler(handleAuthorizationError),
	}, opts...)
	return commonmiddleware.CasbinAuthorization(authorizationAdapter{authz: authz}, resolveAuthorizationRequest, options...)
}

// authenticatedUserID 获取当前请求中已经通过登录验证的用户编号；未登录时返回未登录状态。
func authenticatedUserID(c *gin.Context) (string, bool) {
	value, ok := c.Get(commonauth.UserIDKey)
	if ok {
		userID, ok := value.(string)
		return userID, ok && userID != ""
	}
	return commonauth.UserIDFromContext(c.Request.Context())
}

// Authorize 核心权限检查入口：核对当前已登录的用户是否有权访问所请求的接口，如果无权访问则直接拦截并返回无权限提示。
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

// resolveAuthorizationRequest 从当前请求中提取出“谁（用户）想对什么功能（接口地址）做什么操作（GET/POST等）”，提供给权限判定器使用。
func resolveAuthorizationRequest(c *gin.Context) (commoncasbin.Request, error) {
	pathTemplate := commonroute.TemplateOrUnmatched(c)
	if pathTemplate == commonroute.Unmatched {
		return commoncasbin.Request{}, commoncasbin.ErrDenied
	}
	userID, ok := authenticatedUserID(c)
	if !ok {
		return commoncasbin.Request{}, errAuthorizationUnauthenticated
	}
	return commoncasbin.Request{Subject: userID, Object: pathTemplate, Action: c.Request.Method}, nil
}

// handleAuthorizationError 当用户未登录或权限不足时，向客户端返回得体的错误提示，不暴露系统内部的判定规则细节。
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
