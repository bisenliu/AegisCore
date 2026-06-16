package rolehttp

import (
	"github.com/gin-gonic/gin"

	"github.com/aegiscore/common/http/binding"
	"github.com/aegiscore/common/http/response"
	commonvalidation "github.com/aegiscore/common/validation"
	rolecommand "github.com/aegiscore/user-service/internal/features/role/application/command"
	rolequery "github.com/aegiscore/user-service/internal/features/role/application/query"
)

// RoleController 处理角色管理端点的 HTTP 请求。
type RoleController struct {
	commands  rolecommand.RoleCommandService
	queries   rolequery.RoleQueryService
	validator *commonvalidation.Validator
}

// NewRoleController 使用 command/query services 和请求 validator 依赖构造角色控制器。
func NewRoleController(commands rolecommand.RoleCommandService, queries rolequery.RoleQueryService, validator *commonvalidation.Validator) *RoleController {
	return &RoleController{commands: commands, queries: queries, validator: validator}
}

// ListRoles 处理分页角色列表请求。
// @Summary 分页查询角色
// @Description 查询角色列表。业务接口由 JWT 和 RBAC 保护，Casbin 使用 role_id 作为策略主体，不依赖 roles.code。
// @Tags 角色
// @Produce json
// @Param cursor query string false "分页游标"
// @Param page_size query int false "每页数量"
// @Param active query bool false "是否启用"
// @Param system query bool false "是否系统角色"
// @Success 200 {object} response.Envelope{data=rolehttp.RoleListResponseDoc} "查询成功"
// @Failure 400 {object} response.Envelope "查询参数错误"
// @Failure 401 {object} response.Envelope "未认证或 token 无效"
// @Failure 403 {object} response.Envelope "无访问权限"
// @Failure 500 {object} response.Envelope "服务器内部错误"
// @Security BearerAuth
// @Router /roles [get]
func (ctl *RoleController) ListRoles(c *gin.Context) {
	req := ListRolesRequest{}
	if !binding.BindOrAbort(ctl.validator, c, &req, binding.QueryBinder) {
		return
	}

	query, err := prepareListRolesQuery(req)
	if err != nil {
		response.Fail(c, err)
		return
	}
	result, err := ctl.queries.ListRoles(c.Request.Context(), query)
	if err != nil {
		response.Fail(c, toRoleHTTPError(err))
		return
	}
	response.OK(c, toRoleListResponse(result))
}

// CreateRole 处理角色创建请求。
// @Summary 创建角色
// @Description 创建角色。业务接口由 JWT 和 RBAC 保护。
// @Tags 角色
// @Accept json
// @Produce json
// @Param request body rolehttp.CreateRoleRequest true "创建角色请求"
// @Success 201 {object} response.Envelope{data=rolehttp.RoleResponse} "创建成功"
// @Failure 400 {object} response.Envelope "请求体错误或参数校验失败"
// @Failure 401 {object} response.Envelope "未认证或 token 无效"
// @Failure 403 {object} response.Envelope "无访问权限"
// @Failure 409 {object} response.Envelope "角色已存在"
// @Failure 500 {object} response.Envelope "服务器内部错误"
// @Security BearerAuth
// @Router /roles [post]
func (ctl *RoleController) CreateRole(c *gin.Context) {
	req := CreateRoleRequest{}
	if !binding.BindOrAbort(ctl.validator, c, &req, binding.JSONBinder) {
		return
	}

	cmd, err := prepareCreateRoleCommand(req)
	if err != nil {
		response.Fail(c, err)
		return
	}
	result, err := ctl.commands.CreateRole(c.Request.Context(), cmd)
	if err != nil {
		response.Fail(c, toRoleHTTPError(err))
		return
	}
	response.Created(c, toRoleResponse(result.Role))
}

// GetRole 处理角色详情请求。
// @Summary 查询角色详情
// @Description 通过角色 ID 查询角色详情。业务接口由 JWT 和 RBAC 保护。
// @Tags 角色
// @Produce json
// @Param role_id path string true "角色ID"
// @Success 200 {object} response.Envelope{data=rolehttp.RoleResponse} "查询成功"
// @Failure 400 {object} response.Envelope "角色 ID 参数错误"
// @Failure 401 {object} response.Envelope "未认证或 token 无效"
// @Failure 403 {object} response.Envelope "无访问权限"
// @Failure 404 {object} response.Envelope "角色不存在"
// @Failure 500 {object} response.Envelope "服务器内部错误"
// @Security BearerAuth
// @Router /roles/{role_id} [get]
func (ctl *RoleController) GetRole(c *gin.Context) {
	req := RoleIDRequest{}
	if !binding.BindOrAbort(ctl.validator, c, &req, binding.URIBinder) {
		return
	}

	query, err := prepareGetRoleQuery(req)
	if err != nil {
		response.Fail(c, err)
		return
	}
	result, err := ctl.queries.GetRole(c.Request.Context(), query)
	if err != nil {
		response.Fail(c, toRoleHTTPError(err))
		return
	}
	response.OK(c, toRoleResponse(result.Role))
}

// UpdateRole 处理角色更新请求。
// @Summary 更新角色
// @Description 更新角色元数据。系统角色受保护字段不可破坏性修改。
// @Tags 角色
// @Accept json
// @Produce json
// @Param role_id path string true "角色ID"
// @Param request body rolehttp.UpdateRoleRequest true "更新角色请求"
// @Success 200 {object} response.Envelope{data=rolehttp.RoleResponse} "更新成功"
// @Failure 400 {object} response.Envelope "请求错误或系统角色受保护"
// @Failure 401 {object} response.Envelope "未认证或 token 无效"
// @Failure 403 {object} response.Envelope "无访问权限"
// @Failure 404 {object} response.Envelope "角色不存在"
// @Failure 409 {object} response.Envelope "角色已存在"
// @Failure 500 {object} response.Envelope "服务器内部错误"
// @Security BearerAuth
// @Router /roles/{role_id} [patch]
func (ctl *RoleController) UpdateRole(c *gin.Context) {
	req := UpdateRoleHTTPRequest{}
	if !binding.BindOrAbort(ctl.validator, c, &req, binding.Compose(binding.URIBinder, binding.JSONBinder)) {
		return
	}

	cmd, err := prepareUpdateRoleCommand(req)
	if err != nil {
		response.Fail(c, err)
		return
	}
	result, err := ctl.commands.UpdateRole(c.Request.Context(), cmd)
	if err != nil {
		response.Fail(c, toRoleHTTPError(err))
		return
	}
	response.OK(c, toRoleResponse(result.Role))
}

// SetRoleStatus 处理角色启停请求。
// @Summary 启停角色
// @Description 启用或停用角色。停用角色后相关 RBAC 授权应被拒绝。
// @Tags 角色
// @Accept json
// @Produce json
// @Param role_id path string true "角色ID"
// @Param request body rolehttp.SetRoleStatusRequest true "角色状态请求"
// @Success 200 {object} response.Envelope{data=rolehttp.RoleResponse} "操作成功"
// @Failure 400 {object} response.Envelope "请求错误或系统角色受保护"
// @Failure 401 {object} response.Envelope "未认证或 token 无效"
// @Failure 403 {object} response.Envelope "无访问权限"
// @Failure 404 {object} response.Envelope "角色不存在"
// @Failure 500 {object} response.Envelope "服务器内部错误"
// @Security BearerAuth
// @Router /roles/{role_id}/status [patch]
func (ctl *RoleController) SetRoleStatus(c *gin.Context) {
	req := SetRoleStatusHTTPRequest{}
	if !binding.BindOrAbort(ctl.validator, c, &req, binding.Compose(binding.URIBinder, binding.JSONBinder)) {
		return
	}

	cmd, err := prepareSetRoleActiveCommand(req)
	if err != nil {
		response.Fail(c, err)
		return
	}
	result, err := ctl.commands.SetRoleActive(c.Request.Context(), cmd)
	if err != nil {
		response.Fail(c, toRoleHTTPError(err))
		return
	}
	response.OK(c, toRoleResponse(result.Role))
}

// ListUserRoles 处理用户角色查询请求。
// @Summary 查询用户角色
// @Description 查询用户当前绑定的角色列表。
// @Tags 角色
// @Produce json
// @Param user_id path string true "用户ID"
// @Success 200 {object} response.Envelope{data=[]rolehttp.RoleResponse} "查询成功"
// @Failure 400 {object} response.Envelope "用户 ID 参数错误"
// @Failure 401 {object} response.Envelope "未认证或 token 无效"
// @Failure 403 {object} response.Envelope "无访问权限"
// @Failure 500 {object} response.Envelope "服务器内部错误"
// @Security BearerAuth
// @Router /users/{user_id}/roles [get]
func (ctl *RoleController) ListUserRoles(c *gin.Context) {
	req := UserIDRequest{}
	if !binding.BindOrAbort(ctl.validator, c, &req, binding.URIBinder) {
		return
	}

	query, err := prepareUserRolesQuery(req)
	if err != nil {
		response.Fail(c, err)
		return
	}
	result, err := ctl.queries.ListUserRoles(c.Request.Context(), query)
	if err != nil {
		response.Fail(c, toRoleHTTPError(err))
		return
	}
	response.OK(c, toRoleResponses(result.Items))
}

// ReplaceUserRoles 处理用户角色全量替换请求。
// @Summary 替换用户角色
// @Description 幂等替换用户完整角色集合，用户角色解绑后相关 RBAC 授权应被拒绝。
// @Tags 角色
// @Accept json
// @Produce json
// @Param user_id path string true "用户ID"
// @Param request body rolehttp.RoleIDsRequest true "角色ID列表"
// @Success 200 {object} response.Envelope{data=[]rolehttp.RoleResponse} "替换成功"
// @Failure 400 {object} response.Envelope "请求错误"
// @Failure 401 {object} response.Envelope "未认证或 token 无效"
// @Failure 403 {object} response.Envelope "无访问权限"
// @Failure 404 {object} response.Envelope "角色不存在"
// @Failure 500 {object} response.Envelope "服务器内部错误"
// @Security BearerAuth
// @Router /users/{user_id}/roles [put]
func (ctl *RoleController) ReplaceUserRoles(c *gin.Context) {
	req := ReplaceUserRolesHTTPRequest{}
	if !binding.BindOrAbort(ctl.validator, c, &req, binding.Compose(binding.URIBinder, binding.JSONBinder)) {
		return
	}

	cmd, err := prepareReplaceUserRolesCommand(req)
	if err != nil {
		response.Fail(c, err)
		return
	}
	result, err := ctl.commands.ReplaceUserRoles(c.Request.Context(), cmd)
	if err != nil {
		response.Fail(c, toRoleHTTPError(err))
		return
	}
	response.OK(c, toCommandRoleResponses(result))
}

// AddUserRole 处理用户角色增量绑定请求。
// @Summary 绑定用户角色
// @Description 为用户新增一个角色绑定。
// @Tags 角色
// @Accept json
// @Produce json
// @Param user_id path string true "用户ID"
// @Param request body rolehttp.RoleIDBodyRequest true "角色ID"
// @Success 200 {object} response.Envelope{data=[]rolehttp.RoleResponse} "绑定成功"
// @Failure 400 {object} response.Envelope "请求错误"
// @Failure 401 {object} response.Envelope "未认证或 token 无效"
// @Failure 403 {object} response.Envelope "无访问权限"
// @Failure 404 {object} response.Envelope "角色不存在"
// @Failure 409 {object} response.Envelope "绑定已存在"
// @Failure 500 {object} response.Envelope "服务器内部错误"
// @Security BearerAuth
// @Router /users/{user_id}/roles [post]
func (ctl *RoleController) AddUserRole(c *gin.Context) {
	req := UserRoleHTTPRequest{}
	if !binding.BindOrAbort(ctl.validator, c, &req, binding.Compose(binding.URIBinder, binding.JSONBinder)) {
		return
	}

	cmd, err := prepareUserRoleCommand(req)
	if err != nil {
		response.Fail(c, err)
		return
	}
	result, err := ctl.commands.AddUserRole(c.Request.Context(), cmd)
	if err != nil {
		response.Fail(c, toRoleHTTPError(err))
		return
	}
	response.OK(c, toCommandRoleResponses(result))
}

// RemoveUserRole 处理用户角色解绑请求。
// @Summary 解绑用户角色
// @Description 删除用户角色绑定，解绑后相关 RBAC 授权应被拒绝。
// @Tags 角色
// @Produce json
// @Param user_id path string true "用户ID"
// @Param role_id path string true "角色ID"
// @Success 200 {object} response.Envelope{data=[]rolehttp.RoleResponse} "解绑成功"
// @Failure 400 {object} response.Envelope "请求错误"
// @Failure 401 {object} response.Envelope "未认证或 token 无效"
// @Failure 403 {object} response.Envelope "无访问权限"
// @Failure 404 {object} response.Envelope "绑定不存在"
// @Failure 500 {object} response.Envelope "服务器内部错误"
// @Security BearerAuth
// @Router /users/{user_id}/roles/{role_id} [delete]
func (ctl *RoleController) RemoveUserRole(c *gin.Context) {
	req := UserRoleHTTPRequest{}
	if !binding.BindOrAbort(ctl.validator, c, &req, binding.URIBinder) {
		return
	}

	cmd, err := prepareUserRoleCommand(req)
	if err != nil {
		response.Fail(c, err)
		return
	}
	result, err := ctl.commands.RemoveUserRole(c.Request.Context(), cmd)
	if err != nil {
		response.Fail(c, toRoleHTTPError(err))
		return
	}
	response.OK(c, toCommandRoleResponses(result))
}

// ListRolePermissions 处理角色权限查询请求。
// @Summary 查询角色权限
// @Description 查询角色绑定的权限列表。
// @Tags 角色
// @Produce json
// @Param role_id path string true "角色ID"
// @Success 200 {object} response.Envelope{data=[]rolehttp.PermissionResponse} "查询成功"
// @Failure 400 {object} response.Envelope "角色 ID 参数错误"
// @Failure 401 {object} response.Envelope "未认证或 token 无效"
// @Failure 403 {object} response.Envelope "无访问权限"
// @Failure 500 {object} response.Envelope "服务器内部错误"
// @Security BearerAuth
// @Router /roles/{role_id}/permissions [get]
func (ctl *RoleController) ListRolePermissions(c *gin.Context) {
	req := RoleIDRequest{}
	if !binding.BindOrAbort(ctl.validator, c, &req, binding.URIBinder) {
		return
	}

	query, err := prepareRolePermissionsQuery(req)
	if err != nil {
		response.Fail(c, err)
		return
	}
	result, err := ctl.queries.ListRolePermissions(c.Request.Context(), query)
	if err != nil {
		response.Fail(c, toRoleHTTPError(err))
		return
	}
	response.OK(c, toPermissionResponses(result.Items))
}

// ReplaceRolePermissions 处理角色权限全量替换请求。
// @Summary 替换角色权限
// @Description 幂等替换角色完整权限集合，角色权限解绑后相关 RBAC 授权应被拒绝。
// @Tags 角色
// @Accept json
// @Produce json
// @Param role_id path string true "角色ID"
// @Param request body rolehttp.PermissionIDsRequest true "权限ID列表"
// @Success 200 {object} response.Envelope{data=[]rolehttp.PermissionResponse} "替换成功"
// @Failure 400 {object} response.Envelope "请求错误"
// @Failure 401 {object} response.Envelope "未认证或 token 无效"
// @Failure 403 {object} response.Envelope "无访问权限"
// @Failure 404 {object} response.Envelope "角色或权限不存在"
// @Failure 500 {object} response.Envelope "服务器内部错误"
// @Security BearerAuth
// @Router /roles/{role_id}/permissions [put]
func (ctl *RoleController) ReplaceRolePermissions(c *gin.Context) {
	req := ReplaceRolePermissionsHTTPRequest{}
	if !binding.BindOrAbort(ctl.validator, c, &req, binding.Compose(binding.URIBinder, binding.JSONBinder)) {
		return
	}

	cmd, err := prepareReplaceRolePermissionsCommand(req)
	if err != nil {
		response.Fail(c, err)
		return
	}
	result, err := ctl.commands.ReplaceRolePermissions(c.Request.Context(), cmd)
	if err != nil {
		response.Fail(c, toRoleHTTPError(err))
		return
	}
	response.OK(c, toPermissionResponses(result.Items))
}

// AddRolePermission 处理角色权限增量绑定请求。
// @Summary 绑定角色权限
// @Description 为角色新增一个启用权限绑定。
// @Tags 角色
// @Accept json
// @Produce json
// @Param role_id path string true "角色ID"
// @Param request body rolehttp.PermissionIDBodyRequest true "权限ID"
// @Success 200 {object} response.Envelope{data=[]rolehttp.PermissionResponse} "绑定成功"
// @Failure 400 {object} response.Envelope "请求错误"
// @Failure 401 {object} response.Envelope "未认证或 token 无效"
// @Failure 403 {object} response.Envelope "无访问权限"
// @Failure 404 {object} response.Envelope "角色或启用权限不存在"
// @Failure 409 {object} response.Envelope "绑定已存在"
// @Failure 500 {object} response.Envelope "服务器内部错误"
// @Security BearerAuth
// @Router /roles/{role_id}/permissions [post]
func (ctl *RoleController) AddRolePermission(c *gin.Context) {
	req := RolePermissionHTTPRequest{}
	if !binding.BindOrAbort(ctl.validator, c, &req, binding.Compose(binding.URIBinder, binding.JSONBinder)) {
		return
	}

	cmd, err := prepareRolePermissionCommand(req)
	if err != nil {
		response.Fail(c, err)
		return
	}
	result, err := ctl.commands.AddRolePermission(c.Request.Context(), cmd)
	if err != nil {
		response.Fail(c, toRoleHTTPError(err))
		return
	}
	response.OK(c, toPermissionResponses(result.Items))
}

// RemoveRolePermission 处理角色权限解绑请求。
// @Summary 解绑角色权限
// @Description 删除角色权限绑定，解绑后相关 RBAC 授权应被拒绝。
// @Tags 角色
// @Produce json
// @Param role_id path string true "角色ID"
// @Param permission_id path string true "权限ID"
// @Success 200 {object} response.Envelope{data=[]rolehttp.PermissionResponse} "解绑成功"
// @Failure 400 {object} response.Envelope "请求错误"
// @Failure 401 {object} response.Envelope "未认证或 token 无效"
// @Failure 403 {object} response.Envelope "无访问权限"
// @Failure 404 {object} response.Envelope "绑定不存在"
// @Failure 500 {object} response.Envelope "服务器内部错误"
// @Security BearerAuth
// @Router /roles/{role_id}/permissions/{permission_id} [delete]
func (ctl *RoleController) RemoveRolePermission(c *gin.Context) {
	req := RolePermissionHTTPRequest{}
	if !binding.BindOrAbort(ctl.validator, c, &req, binding.URIBinder) {
		return
	}

	cmd, err := prepareRolePermissionCommand(req)
	if err != nil {
		response.Fail(c, err)
		return
	}
	result, err := ctl.commands.RemoveRolePermission(c.Request.Context(), cmd)
	if err != nil {
		response.Fail(c, toRoleHTTPError(err))
		return
	}
	response.OK(c, toPermissionResponses(result.Items))
}
