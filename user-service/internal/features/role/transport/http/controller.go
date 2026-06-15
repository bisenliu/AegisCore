package rolehttp

import (
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

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
func (ctl *RoleController) ListRoles(c *gin.Context) {
	req := ListRolesRequest{}
	if !binding.BindOrAbort(ctl.validator, c, &req, binding.QueryBinder) {
		return
	}
	NormalizeListRoles(&req)
	cursor, err := ParseListCursor(req)
	if err != nil {
		response.Fail(c, err)
		return
	}
	result, err := ctl.queries.ListRoles(c.Request.Context(), rolequery.ListRolesQuery{Cursor: cursor, PageSize: req.PageSize, Limit: req.Limit, Active: req.Active, IsSystem: req.System})
	if err != nil {
		response.Fail(c, toRoleHTTPError(err))
		return
	}
	response.OK(c, toRoleListResponse(result))
}

// CreateRole 处理角色创建请求。
func (ctl *RoleController) CreateRole(c *gin.Context) {
	req := CreateRoleRequest{}
	if !binding.BindOrAbort(ctl.validator, c, &req, binding.JSONBinder) {
		return
	}
	NormalizeCreateRole(&req)
	result, err := ctl.commands.CreateRole(c.Request.Context(), rolecommand.CreateRoleCommand{Name: req.Name, Description: req.Description, Active: req.Active, IsSystem: req.System})
	if err != nil {
		response.Fail(c, toRoleHTTPError(err))
		return
	}
	response.Created(c, toRoleResponse(result.Role))
}

// GetRole 处理角色详情请求。
func (ctl *RoleController) GetRole(c *gin.Context) {
	roleID, ok := ctl.parseRoleID(c)
	if !ok {
		return
	}
	result, err := ctl.queries.GetRole(c.Request.Context(), rolequery.GetRoleQuery{RoleID: roleID})
	if err != nil {
		response.Fail(c, toRoleHTTPError(err))
		return
	}
	response.OK(c, toRoleResponse(result.Role))
}

// UpdateRole 处理角色更新请求。
func (ctl *RoleController) UpdateRole(c *gin.Context) {
	roleID, ok := ctl.parseRoleID(c)
	if !ok {
		return
	}
	req := UpdateRoleRequest{}
	if !binding.BindOrAbort(ctl.validator, c, &req, binding.JSONBinder) {
		return
	}
	NormalizeUpdateRole(&req)
	result, err := ctl.commands.UpdateRole(c.Request.Context(), rolecommand.UpdateRoleCommand{RoleID: roleID, Name: req.Name, Description: req.Description, Active: req.Active})
	if err != nil {
		response.Fail(c, toRoleHTTPError(err))
		return
	}
	response.OK(c, toRoleResponse(result.Role))
}

// SetRoleStatus 处理角色启停请求。
func (ctl *RoleController) SetRoleStatus(c *gin.Context) {
	roleID, ok := ctl.parseRoleID(c)
	if !ok {
		return
	}
	req := SetRoleStatusRequest{}
	if !binding.BindOrAbort(ctl.validator, c, &req, binding.JSONBinder) {
		return
	}
	result, err := ctl.commands.SetRoleActive(c.Request.Context(), rolecommand.SetRoleActiveCommand{RoleID: roleID, Active: req.Active})
	if err != nil {
		response.Fail(c, toRoleHTTPError(err))
		return
	}
	response.OK(c, toRoleResponse(result.Role))
}

// ListUserRoles 处理用户角色查询请求。
func (ctl *RoleController) ListUserRoles(c *gin.Context) {
	userID, ok := ctl.parseUserID(c)
	if !ok {
		return
	}
	result, err := ctl.queries.ListUserRoles(c.Request.Context(), rolequery.UserRolesQuery{UserID: userID})
	if err != nil {
		response.Fail(c, toRoleHTTPError(err))
		return
	}
	response.OK(c, toRoleResponses(result.Items))
}

// ReplaceUserRoles 处理用户角色全量替换请求。
func (ctl *RoleController) ReplaceUserRoles(c *gin.Context) {
	userID, ok := ctl.parseUserID(c)
	if !ok {
		return
	}
	roleIDs, ok := ctl.parseRoleIDsBody(c)
	if !ok {
		return
	}
	result, err := ctl.commands.ReplaceUserRoles(c.Request.Context(), rolecommand.ReplaceUserRolesCommand{UserID: userID, RoleIDs: roleIDs})
	if err != nil {
		response.Fail(c, toRoleHTTPError(err))
		return
	}
	response.OK(c, toCommandRoleResponses(result))
}

// AddUserRole 处理用户角色增量绑定请求。
func (ctl *RoleController) AddUserRole(c *gin.Context) {
	userID, ok := ctl.parseUserID(c)
	if !ok {
		return
	}
	roleID, ok := ctl.parseRoleIDBody(c)
	if !ok {
		return
	}
	result, err := ctl.commands.AddUserRole(c.Request.Context(), rolecommand.UserRoleCommand{UserID: userID, RoleID: roleID})
	if err != nil {
		response.Fail(c, toRoleHTTPError(err))
		return
	}
	response.OK(c, toCommandRoleResponses(result))
}

// RemoveUserRole 处理用户角色解绑请求。
func (ctl *RoleController) RemoveUserRole(c *gin.Context) {
	userID, roleID, ok := ctl.parseUserRoleIDs(c)
	if !ok {
		return
	}
	result, err := ctl.commands.RemoveUserRole(c.Request.Context(), rolecommand.UserRoleCommand{UserID: userID, RoleID: roleID})
	if err != nil {
		response.Fail(c, toRoleHTTPError(err))
		return
	}
	response.OK(c, toCommandRoleResponses(result))
}

// ListRolePermissions 处理角色权限查询请求。
func (ctl *RoleController) ListRolePermissions(c *gin.Context) {
	roleID, ok := ctl.parseRoleID(c)
	if !ok {
		return
	}
	result, err := ctl.queries.ListRolePermissions(c.Request.Context(), rolequery.RolePermissionsQuery{RoleID: roleID})
	if err != nil {
		response.Fail(c, toRoleHTTPError(err))
		return
	}
	response.OK(c, toPermissionResponses(result.Items))
}

// ReplaceRolePermissions 处理角色权限全量替换请求。
func (ctl *RoleController) ReplaceRolePermissions(c *gin.Context) {
	roleID, ok := ctl.parseRoleID(c)
	if !ok {
		return
	}
	permissionIDs, ok := ctl.parsePermissionIDsBody(c)
	if !ok {
		return
	}
	result, err := ctl.commands.ReplaceRolePermissions(c.Request.Context(), rolecommand.ReplaceRolePermissionsCommand{RoleID: roleID, PermissionIDs: permissionIDs})
	if err != nil {
		response.Fail(c, toRoleHTTPError(err))
		return
	}
	response.OK(c, toPermissionResponses(result.Items))
}

// AddRolePermission 处理角色权限增量绑定请求。
func (ctl *RoleController) AddRolePermission(c *gin.Context) {
	roleID, ok := ctl.parseRoleID(c)
	if !ok {
		return
	}
	permissionID, ok := ctl.parsePermissionIDBody(c)
	if !ok {
		return
	}
	result, err := ctl.commands.AddRolePermission(c.Request.Context(), rolecommand.RolePermissionCommand{RoleID: roleID, PermissionID: permissionID})
	if err != nil {
		response.Fail(c, toRoleHTTPError(err))
		return
	}
	response.OK(c, toPermissionResponses(result.Items))
}

// RemoveRolePermission 处理角色权限解绑请求。
func (ctl *RoleController) RemoveRolePermission(c *gin.Context) {
	roleID, permissionID, ok := ctl.parseRolePermissionIDs(c)
	if !ok {
		return
	}
	result, err := ctl.commands.RemoveRolePermission(c.Request.Context(), rolecommand.RolePermissionCommand{RoleID: roleID, PermissionID: permissionID})
	if err != nil {
		response.Fail(c, toRoleHTTPError(err))
		return
	}
	response.OK(c, toPermissionResponses(result.Items))
}

func (ctl *RoleController) parseRoleID(c *gin.Context) (uuid.UUID, bool) {
	req := RoleIDRequest{}
	if !binding.BindOrAbort(ctl.validator, c, &req, binding.URIBinder) {
		return uuid.Nil, false
	}
	roleID, err := ParseRoleID(req)
	if err != nil {
		response.Fail(c, err)
		return uuid.Nil, false
	}
	return roleID, true
}

func (ctl *RoleController) parseUserID(c *gin.Context) (uuid.UUID, bool) {
	req := UserIDRequest{}
	if !binding.BindOrAbort(ctl.validator, c, &req, binding.URIBinder) {
		return uuid.Nil, false
	}
	userID, err := ParseUserID(req)
	if err != nil {
		response.Fail(c, err)
		return uuid.Nil, false
	}
	return userID, true
}

func (ctl *RoleController) parseUserRoleIDs(c *gin.Context) (uuid.UUID, uuid.UUID, bool) {
	req := UserRoleRequest{}
	if !binding.BindOrAbort(ctl.validator, c, &req, binding.URIBinder) {
		return uuid.Nil, uuid.Nil, false
	}
	userID, roleID, err := ParseUserRoleIDs(req)
	if err != nil {
		response.Fail(c, err)
		return uuid.Nil, uuid.Nil, false
	}
	return userID, roleID, true
}

func (ctl *RoleController) parseRolePermissionIDs(c *gin.Context) (uuid.UUID, uuid.UUID, bool) {
	req := RolePermissionRequest{}
	if !binding.BindOrAbort(ctl.validator, c, &req, binding.URIBinder) {
		return uuid.Nil, uuid.Nil, false
	}
	roleID, permissionID, err := ParseRolePermissionIDs(req)
	if err != nil {
		response.Fail(c, err)
		return uuid.Nil, uuid.Nil, false
	}
	return roleID, permissionID, true
}

func (ctl *RoleController) parseRoleIDsBody(c *gin.Context) ([]uuid.UUID, bool) {
	req := RoleIDsRequest{}
	if !binding.BindOrAbort(ctl.validator, c, &req, binding.JSONBinder) {
		return nil, false
	}
	ids, err := ParseRoleIDs(req)
	if err != nil {
		response.Fail(c, err)
		return nil, false
	}
	return ids, true
}

func (ctl *RoleController) parseRoleIDBody(c *gin.Context) (uuid.UUID, bool) {
	req := RoleIDBodyRequest{}
	if !binding.BindOrAbort(ctl.validator, c, &req, binding.JSONBinder) {
		return uuid.Nil, false
	}
	id, err := ParseRoleIDBody(req)
	if err != nil {
		response.Fail(c, err)
		return uuid.Nil, false
	}
	return id, true
}

func (ctl *RoleController) parsePermissionIDsBody(c *gin.Context) ([]uuid.UUID, bool) {
	req := PermissionIDsRequest{}
	if !binding.BindOrAbort(ctl.validator, c, &req, binding.JSONBinder) {
		return nil, false
	}
	ids, err := ParsePermissionIDs(req)
	if err != nil {
		response.Fail(c, err)
		return nil, false
	}
	return ids, true
}

func (ctl *RoleController) parsePermissionIDBody(c *gin.Context) (uuid.UUID, bool) {
	req := PermissionIDBodyRequest{}
	if !binding.BindOrAbort(ctl.validator, c, &req, binding.JSONBinder) {
		return uuid.Nil, false
	}
	id, err := ParsePermissionIDBody(req)
	if err != nil {
		response.Fail(c, err)
		return uuid.Nil, false
	}
	return id, true
}
