package rolehttp

import (
	"github.com/gin-gonic/gin"

	"github.com/aegiscore/common/http/binding"
	"github.com/aegiscore/common/http/response"
)

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
		response.Fail(c, err)
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
		response.Fail(c, err)
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
		response.Fail(c, err)
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
		response.Fail(c, err)
		return
	}
	response.OK(c, toCommandRoleResponses(result))
}
