package permissionhttp

import (
	"github.com/gin-gonic/gin"

	"github.com/aegiscore/common/http/binding"
	"github.com/aegiscore/common/http/response"
	commonvalidation "github.com/aegiscore/common/validation"
	permissioncommand "github.com/aegiscore/user-service/internal/features/permission/application/command"
	permissionquery "github.com/aegiscore/user-service/internal/features/permission/application/query"
)

// PermissionController 处理权限目录端点的 HTTP 请求。
type PermissionController struct {
	commands  permissioncommand.PermissionCommandService
	queries   permissionquery.PermissionQueryService
	validator *commonvalidation.Validator
}

// NewPermissionController 使用 command/query services 和请求 validator 依赖构造权限控制器。
func NewPermissionController(commands permissioncommand.PermissionCommandService, queries permissionquery.PermissionQueryService, validator *commonvalidation.Validator) *PermissionController {
	return &PermissionController{commands: commands, queries: queries, validator: validator}
}

// ListPermissions 处理分页权限列表请求。
// @Summary 分页查询权限
// @Description 查询权限目录。业务接口由 JWT 和 RBAC 保护。
// @Tags 权限
// @Produce json
// @Param cursor query string false "分页游标"
// @Param page_size query int false "每页数量"
// @Param module query string false "模块"
// @Param http_method query string false "HTTP 方法"
// @Param active query bool false "是否启用"
// @Param system query bool false "是否系统权限"
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
		response.Fail(c, toPermissionHTTPError(err))
		return
	}
	response.OK(c, toPermissionListResponse(result))
}

// CreatePermission 处理权限创建请求。
// @Summary 创建权限
// @Description 创建正式权限目录记录。业务接口由 JWT 和 RBAC 保护。
// @Tags 权限
// @Accept json
// @Produce json
// @Param request body permissionhttp.CreatePermissionRequest true "创建权限请求"
// @Success 201 {object} response.Envelope{data=permissionhttp.PermissionResponse} "创建成功"
// @Failure 400 {object} response.Envelope "请求体错误或参数校验失败"
// @Failure 401 {object} response.Envelope "未认证或 token 无效"
// @Failure 403 {object} response.Envelope "无访问权限"
// @Failure 409 {object} response.Envelope "权限已存在"
// @Failure 500 {object} response.Envelope "服务器内部错误"
// @Security BearerAuth
// @Router /permissions [post]
func (ctl *PermissionController) CreatePermission(c *gin.Context) {
	req := CreatePermissionRequest{}
	if !binding.BindOrAbort(ctl.validator, c, &req, binding.JSONBinder) {
		return
	}

	cmd, err := prepareCreatePermissionCommand(req)
	if err != nil {
		response.Fail(c, err)
		return
	}
	result, err := ctl.commands.CreatePermission(c.Request.Context(), cmd)
	if err != nil {
		response.Fail(c, toPermissionHTTPError(err))
		return
	}
	response.Created(c, toPermissionResponse(result.Permission))
}

// GetPermission 处理权限详情请求。
// @Summary 查询权限详情
// @Description 通过权限 ID 查询权限目录记录。
// @Tags 权限
// @Produce json
// @Param permission_id path string true "权限ID"
// @Success 200 {object} response.Envelope{data=permissionhttp.PermissionResponse} "查询成功"
// @Failure 400 {object} response.Envelope "权限 ID 参数错误"
// @Failure 401 {object} response.Envelope "未认证或 token 无效"
// @Failure 403 {object} response.Envelope "无访问权限"
// @Failure 404 {object} response.Envelope "权限不存在"
// @Failure 500 {object} response.Envelope "服务器内部错误"
// @Security BearerAuth
// @Router /permissions/{permission_id} [get]
func (ctl *PermissionController) GetPermission(c *gin.Context) {
	req := PermissionIDRequest{}
	if !binding.BindOrAbort(ctl.validator, c, &req, binding.URIBinder) {
		return
	}

	query, err := prepareGetPermissionQuery(req)
	if err != nil {
		response.Fail(c, err)
		return
	}
	result, err := ctl.queries.GetPermission(c.Request.Context(), query)
	if err != nil {
		response.Fail(c, toPermissionHTTPError(err))
		return
	}
	response.OK(c, toPermissionResponse(result.Permission))
}

// UpdatePermission 处理权限更新请求。
// @Summary 更新权限
// @Description 更新权限目录记录。系统权限身份字段受保护。
// @Tags 权限
// @Accept json
// @Produce json
// @Param permission_id path string true "权限ID"
// @Param request body permissionhttp.UpdatePermissionRequest true "更新权限请求"
// @Success 204 "更新成功"
// @Failure 400 {object} response.Envelope "请求错误或系统权限受保护"
// @Failure 401 {object} response.Envelope "未认证或 token 无效"
// @Failure 403 {object} response.Envelope "无访问权限"
// @Failure 404 {object} response.Envelope "权限不存在"
// @Failure 409 {object} response.Envelope "权限已存在"
// @Failure 500 {object} response.Envelope "服务器内部错误"
// @Security BearerAuth
// @Router /permissions/{permission_id} [put]
func (ctl *PermissionController) UpdatePermission(c *gin.Context) {
	req := UpdatePermissionHTTPRequest{}
	if !binding.BindOrAbort(ctl.validator, c, &req, binding.Compose(binding.URIBinder, binding.JSONBinder)) {
		return
	}

	cmd, err := prepareUpdatePermissionCommand(req)
	if err != nil {
		response.Fail(c, err)
		return
	}
	err = ctl.commands.UpdatePermission(c.Request.Context(), cmd)
	if err != nil {
		response.Fail(c, toPermissionHTTPError(err))
		return
	}
	response.NoContent(c)
}

// EnablePermission 处理权限启用请求。
// @Summary 启用权限
// @Description 启用权限目录记录。
// @Tags 权限
// @Produce json
// @Param permission_id path string true "权限ID"
// @Success 204 "启用成功"
// @Failure 400 {object} response.Envelope "权限 ID 参数错误"
// @Failure 401 {object} response.Envelope "未认证或 token 无效"
// @Failure 403 {object} response.Envelope "无访问权限"
// @Failure 404 {object} response.Envelope "权限不存在"
// @Failure 500 {object} response.Envelope "服务器内部错误"
// @Security BearerAuth
// @Router /permissions/{permission_id}/enable [post]
func (ctl *PermissionController) EnablePermission(c *gin.Context) {
	ctl.setPermissionActive(c, true)
}

// DisablePermission 处理权限停用请求。
// @Summary 停用权限
// @Description 停用权限目录记录。停用后相关 RBAC 授权应被拒绝。
// @Tags 权限
// @Produce json
// @Param permission_id path string true "权限ID"
// @Success 204 "停用成功"
// @Failure 400 {object} response.Envelope "权限 ID 参数错误"
// @Failure 401 {object} response.Envelope "未认证或 token 无效"
// @Failure 403 {object} response.Envelope "无访问权限"
// @Failure 404 {object} response.Envelope "权限不存在"
// @Failure 500 {object} response.Envelope "服务器内部错误"
// @Security BearerAuth
// @Router /permissions/{permission_id}/disable [post]
func (ctl *PermissionController) DisablePermission(c *gin.Context) {
	ctl.setPermissionActive(c, false)
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
		response.Fail(c, toPermissionHTTPError(err))
		return
	}
	response.OK(c, toPermissionResponses(result.Items))
}

// GetRouteDiff 处理权限目录与已注册路由的差异查询请求。
// @Summary 查询权限路由差异
// @Description 只读比较 Gin 已注册业务路由和正式权限目录差异；不会创建权限，也不会绑定角色。
// @Tags 权限
// @Produce json
// @Success 200 {object} response.Envelope{data=permissionhttp.RouteDiffResponse} "查询成功"
// @Failure 401 {object} response.Envelope "未认证或 token 无效"
// @Failure 403 {object} response.Envelope "无访问权限"
// @Failure 500 {object} response.Envelope "服务器内部错误"
// @Security BearerAuth
// @Router /permissions/route-diff [get]
func (ctl *PermissionController) GetRouteDiff(c *gin.Context) {
	result, err := ctl.queries.GetRouteDiff(c.Request.Context())
	if err != nil {
		response.Fail(c, toPermissionHTTPError(err))
		return
	}
	response.OK(c, toRouteDiffResponse(result))
}

func (ctl *PermissionController) setPermissionActive(c *gin.Context, active bool) {
	req := PermissionIDRequest{}
	if !binding.BindOrAbort(ctl.validator, c, &req, binding.URIBinder) {
		return
	}

	cmd, err := prepareSetPermissionActiveCommand(req)
	if err != nil {
		response.Fail(c, err)
		return
	}
	if active {
		err = ctl.commands.EnablePermission(c.Request.Context(), cmd)
	} else {
		err = ctl.commands.DisablePermission(c.Request.Context(), cmd)
	}
	if err != nil {
		response.Fail(c, toPermissionHTTPError(err))
		return
	}
	response.NoContent(c)
}
