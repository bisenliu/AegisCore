package permissionhttp

import (
	"github.com/gin-gonic/gin"

	"github.com/aegiscore/common/http/binding"
	"github.com/aegiscore/common/http/response"
	commonvalidation "github.com/aegiscore/common/validation"
	permissionquery "github.com/aegiscore/user-service/internal/features/permission/application/query"
)

// PermissionController 处理权限目录端点的 HTTP 请求。
type PermissionController struct {
	queries   permissionquery.PermissionQueryService
	validator *commonvalidation.Validator
}

// NewPermissionController 使用 query service 和请求 validator 依赖构造权限控制器。
func NewPermissionController(queries permissionquery.PermissionQueryService, validator *commonvalidation.Validator) *PermissionController {
	return &PermissionController{queries: queries, validator: validator}
}

// ListPermissions 处理权限目录列表请求。
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

// ListUserEffectivePermissions 处理用户有效权限查询请求。
// @Summary 查询用户有效权限
// @Description 查询用户经角色绑定后当前生效的权限集合。
// @Tags 权限
// @Produce json
// @Param user_id path string true "用户ID"
// @Success 200 {object} response.Envelope{data=[]permissionhttp.PermissionResponse} "查询成功"
// @Failure 400 {object} response.Envelope "用户 ID 参数错误"
// @Failure 401 {object} response.Envelope "未认证或 token 无效"
// @Failure 403 {object} response.Envelope "无访问权限"
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
