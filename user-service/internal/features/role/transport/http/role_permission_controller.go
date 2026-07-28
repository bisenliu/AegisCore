package rolehttp

import (
	"github.com/gin-gonic/gin"

	"github.com/aegiscore/common/http/binding"
	"github.com/aegiscore/common/http/response"
)

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
// @Failure 429 {object} response.Envelope "请求过于频繁"
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
		response.Fail(c, err)
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
// @Failure 429 {object} response.Envelope "请求过于频繁"
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
		response.Fail(c, err)
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
// @Failure 429 {object} response.Envelope "请求过于频繁"
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
		response.Fail(c, err)
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
// @Failure 429 {object} response.Envelope "请求过于频繁"
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
		response.Fail(c, err)
		return
	}
	response.OK(c, toPermissionResponses(result.Items))
}
