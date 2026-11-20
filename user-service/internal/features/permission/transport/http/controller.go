package permissionhttp

import (
	"github.com/gin-gonic/gin"

	"github.com/aegiscore/platform/http/binding"
	"github.com/aegiscore/platform/http/response"
	platformvalidation "github.com/aegiscore/platform/validation"
	permissionquery "github.com/aegiscore/user-service/internal/features/permission/application/query"
)

// PermissionController 负责接收和处理权限目录查询相关的 HTTP 接口请求。
type PermissionController struct {
	queries   permissionquery.PermissionQueryService
	validator *platformvalidation.Validator
}

// NewPermissionController 创建权限接口控制器，组装权限查询与参数校验服务。
func NewPermissionController(queries permissionquery.PermissionQueryService, validator *platformvalidation.Validator) *PermissionController {
	return &PermissionController{queries: queries, validator: validator}
}

// ListPermissions 查询系统完整权限目录接口。
// @Summary 查询完整权限目录
// @Description 返回由代码基线定义并同步到数据库的完整权限目录，不使用分页；支持按业务模块和 HTTP 方法过滤。该接口由 JWT 和 RBAC 保护。
// @Tags 权限
// @Produce json
// @Param module query string false "业务模块"
// @Param http_method query string false "HTTP 方法"
// @Success 200 {object} response.Envelope{data=permissionhttp.PermissionListResponseDoc} "查询成功"
// @Failure 400 {object} response.Envelope "查询参数错误"
// @Failure 401 {object} response.Envelope "未认证或 token 无效"
// @Failure 403 {object} response.Envelope "无访问权限"
// @Failure 429 {object} response.Envelope "请求过于频繁"
// @Failure 500 {object} response.Envelope "服务器内部错误"
// @Security BearerAuth
// @Router /permissions [get]
func (ctl *PermissionController) ListPermissions(c *gin.Context) {
	req := ListPermissionsRequest{}
	if !binding.BindOrAbort(ctl.validator, c, &req, binding.QueryBinder) {
		return
	}
	query, err := prepareListPermissionsQuery(req)
	if err != nil {
		response.Fail(c, err)
		return
	}
	result, err := ctl.queries.ListPermissions(c.Request.Context(), query)
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, toPermissionListResponse(result))
}

// ListUserEffectivePermissions 查询指定用户当前拥有的全部生效权限接口。
// @Summary 查询用户有效权限
// @Description 查询用户经角色绑定后当前生效的权限集合。
// @Tags 权限
// @Produce json
// @Param user_id path string true "用户ID"
// @Success 200 {object} response.Envelope{data=[]permissionhttp.PermissionResponse} "查询成功"
// @Failure 400 {object} response.Envelope "用户 ID 参数错误"
// @Failure 401 {object} response.Envelope "未认证或 token 无效"
// @Failure 403 {object} response.Envelope "无访问权限"
// @Failure 429 {object} response.Envelope "请求过于频繁"
// @Failure 500 {object} response.Envelope "服务器内部错误"
// @Security BearerAuth
// @Router /permissions/users/{user_id}/effective [get]
func (ctl *PermissionController) ListUserEffectivePermissions(c *gin.Context) {
	req := UserIDRequest{}
	if !binding.BindOrAbort(ctl.validator, c, &req, binding.URIBinder) {
		return
	}
	query, err := prepareUserEffectivePermissionsQuery(req)
	if err != nil {
		response.Fail(c, err)
		return
	}
	result, err := ctl.queries.ListUserEffectivePermissions(c.Request.Context(), query)
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, toPermissionResponses(result.Items))
}
