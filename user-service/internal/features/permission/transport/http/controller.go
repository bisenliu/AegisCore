package permissionhttp

import (
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

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
func (ctl *PermissionController) ListPermissions(c *gin.Context) {
	req := ListPermissionsRequest{}
	if !binding.BindOrAbort(ctl.validator, c, &req, binding.QueryBinder) {
		return
	}
	NormalizeListPermissions(&req)
	cursor, err := ParseListCursor(req)
	if err != nil {
		response.Fail(c, err)
		return
	}
	result, err := ctl.queries.ListPermissions(c.Request.Context(), permissionquery.ListPermissionsQuery{Cursor: cursor, PageSize: req.PageSize, Limit: req.Limit, Module: req.Module, HTTPMethod: req.HTTPMethod, Active: req.Active, IsSystem: req.System})
	if err != nil {
		response.Fail(c, toPermissionHTTPError(err))
		return
	}
	response.OK(c, toPermissionListResponse(result))
}

// CreatePermission 处理权限创建请求。
func (ctl *PermissionController) CreatePermission(c *gin.Context) {
	req := CreatePermissionRequest{}
	if !binding.BindOrAbort(ctl.validator, c, &req, binding.JSONBinder) {
		return
	}
	NormalizeCreatePermission(&req)
	result, err := ctl.commands.CreatePermission(c.Request.Context(), permissioncommand.CreatePermissionCommand{Name: req.Name, Description: req.Description, Module: req.Module, HTTPMethod: req.HTTPMethod, PathTemplate: req.PathTemplate, Active: req.Active, IsSystem: req.System})
	if err != nil {
		response.Fail(c, toPermissionHTTPError(err))
		return
	}
	response.Created(c, toPermissionResponse(result.Permission))
}

// GetPermission 处理权限详情请求。
func (ctl *PermissionController) GetPermission(c *gin.Context) {
	permissionID, ok := ctl.parsePermissionID(c)
	if !ok {
		return
	}
	result, err := ctl.queries.GetPermission(c.Request.Context(), permissionquery.GetPermissionQuery{PermissionID: permissionID})
	if err != nil {
		response.Fail(c, toPermissionHTTPError(err))
		return
	}
	response.OK(c, toPermissionResponse(result.Permission))
}

// UpdatePermission 处理权限更新请求。
func (ctl *PermissionController) UpdatePermission(c *gin.Context) {
	permissionID, ok := ctl.parsePermissionID(c)
	if !ok {
		return
	}
	req := UpdatePermissionRequest{}
	if !binding.BindOrAbort(ctl.validator, c, &req, binding.JSONBinder) {
		return
	}
	NormalizeUpdatePermission(&req)
	result, err := ctl.commands.UpdatePermission(c.Request.Context(), permissioncommand.UpdatePermissionCommand{PermissionID: permissionID, Name: req.Name, Description: req.Description, Module: req.Module, HTTPMethod: req.HTTPMethod, PathTemplate: req.PathTemplate, Active: req.Active})
	if err != nil {
		response.Fail(c, toPermissionHTTPError(err))
		return
	}
	response.OK(c, toPermissionResponse(result.Permission))
}

// EnablePermission 处理权限启用请求。
func (ctl *PermissionController) EnablePermission(c *gin.Context) {
	ctl.setPermissionActive(c, true)
}

// DisablePermission 处理权限停用请求。
func (ctl *PermissionController) DisablePermission(c *gin.Context) {
	ctl.setPermissionActive(c, false)
}

// ListUserEffectivePermissions 处理用户有效权限查询请求。
func (ctl *PermissionController) ListUserEffectivePermissions(c *gin.Context) {
	req := UserIDRequest{}
	if !binding.BindOrAbort(ctl.validator, c, &req, binding.URIBinder) {
		return
	}
	userID, err := ParseUserID(req)
	if err != nil {
		response.Fail(c, err)
		return
	}
	result, err := ctl.queries.ListUserEffectivePermissions(c.Request.Context(), permissionquery.UserEffectivePermissionsQuery{UserID: userID})
	if err != nil {
		response.Fail(c, toPermissionHTTPError(err))
		return
	}
	response.OK(c, toPermissionResponses(result.Items))
}

// GetRouteDiff 处理权限目录与已注册路由的差异查询请求。
func (ctl *PermissionController) GetRouteDiff(c *gin.Context) {
	result, err := ctl.queries.GetRouteDiff(c.Request.Context())
	if err != nil {
		response.Fail(c, toPermissionHTTPError(err))
		return
	}
	response.OK(c, toRouteDiffResponse(result))
}

func (ctl *PermissionController) parsePermissionID(c *gin.Context) (uuid.UUID, bool) {
	req := PermissionIDRequest{}
	if !binding.BindOrAbort(ctl.validator, c, &req, binding.URIBinder) {
		return uuid.Nil, false
	}
	permissionID, err := ParsePermissionID(req)
	if err != nil {
		response.Fail(c, err)
		return uuid.Nil, false
	}
	return permissionID, true
}

func (ctl *PermissionController) setPermissionActive(c *gin.Context, active bool) {
	permissionID, ok := ctl.parsePermissionID(c)
	if !ok {
		return
	}
	cmd := permissioncommand.SetPermissionActiveCommand{PermissionID: permissionID}
	var result *permissioncommand.PermissionResult
	var err error
	if active {
		result, err = ctl.commands.EnablePermission(c.Request.Context(), cmd)
	} else {
		result, err = ctl.commands.DisablePermission(c.Request.Context(), cmd)
	}
	if err != nil {
		response.Fail(c, toPermissionHTTPError(err))
		return
	}
	response.OK(c, toPermissionResponse(result.Permission))
}
