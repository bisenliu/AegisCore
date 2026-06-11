package userhttp

import (
	"github.com/aegiscore/common/http/binding"
	"github.com/aegiscore/common/http/response"
	commonvalidation "github.com/aegiscore/common/validation"
	userapplication "github.com/aegiscore/user-service/internal/features/user/application"
	"github.com/gin-gonic/gin"
)

// UserController 处理用户资料端点的 HTTP 请求。
type UserController struct {
	userService userapplication.UserService
	validator   *commonvalidation.Validator
}

// NewUserController 使用 service 和请求 validator 依赖构造用户控制器。
func NewUserController(userService userapplication.UserService, validator *commonvalidation.Validator) *UserController {
	return &UserController{userService: userService, validator: validator}
}

// ListUsers 处理分页用户列表请求。
// @Summary 分页查询用户列表
// @Description 基于 user_id keyset 分页查询用户资料列表，支持按用户昵称、用户名和用户状态过滤，默认排除软删除用户。page_size 未传或小于 1 时使用默认值 10，超过 100 时按 100 处理。
// @Tags 用户
// @Produce json
// @Param cursor query string false "分页游标，上一页最后一条用户的 user_id"
// @Param page_size query int false "每页数量，未传或小于 1 时默认为 10，超过 100 时按 100 处理" minimum(1) maximum(100)
// @Param nickname query string false "用户昵称模糊匹配"
// @Param username query string false "用户名精确匹配"
// @Param status query int false "用户状态：100 正常，200 冻结/停用，300 必须修改密码" Enums(100,200,300)
// @Success 200 {object} response.Envelope{data=userhttp.UserListResponseDoc} "查询成功"
// @Failure 400 {object} response.Envelope "查询参数错误"
// @Failure 401 {object} response.Envelope "未认证或 token 无效"
// @Failure 500 {object} response.Envelope "服务器内部错误"
// @Security BearerAuth
// @Router /users [get]
func (ctl *UserController) ListUsers(c *gin.Context) {
	req := ListUsersRequest{}
	if !binding.BindOrAbort(ctl.validator, c, &req, binding.QueryBinder) {
		return
	}

	NormalizeListUsers(&req)
	cursor, err := ParseListCursor(req)
	if err != nil {
		response.Fail(c, err)
		return
	}
	users, err := ctl.userService.ListUsers(c.Request.Context(), userapplication.ListUsersQuery{
		Cursor:   cursor,
		PageSize: req.PageSize,
		Limit:    req.Limit,
		Nickname: req.Nickname,
		Username: req.Username,
		Status:   req.Status,
	})
	if err != nil {
		response.Fail(c, toUserHTTPError(err))
		return
	}
	response.OK(c, toUserListResponse(users))
}

// CreateUser 处理用户创建请求。
// @Summary 创建用户
// @Description 创建一个新的用户资料。请求体使用共享校验器校验，用户名必须唯一，status 缺省为 100。
// @Tags 用户
// @Accept json
// @Produce json
// @Param request body userhttp.CreateUserRequest true "创建用户请求"
// @Success 201 {object} response.Envelope{data=userhttp.UserResponseDoc} "创建成功"
// @Failure 400 {object} response.Envelope "请求体错误或参数校验失败"
// @Failure 401 {object} response.Envelope "未认证或 token 无效"
// @Failure 409 {object} response.Envelope "用户已存在"
// @Failure 500 {object} response.Envelope "服务器内部错误"
// @Security BearerAuth
// @Router /users [post]
func (ctl *UserController) CreateUser(c *gin.Context) {
	req := CreateUserRequest{}
	if !binding.BindOrAbort(ctl.validator, c, &req, binding.JSONBinder) {
		return
	}
	if err := NormalizeCreateUser(&req); err != nil {
		response.Fail(c, err)
		return
	}

	user, err := ctl.userService.CreateUser(c.Request.Context(), userapplication.CreateUserCommand{
		Nickname: req.Nickname,
		Username: req.Username,
		Password: req.Password,
		Status:   req.Status,
	})
	if err != nil {
		response.Fail(c, toUserHTTPError(err))
		return
	}
	response.Created(c, toUserResponse(user))
}

// GetByUserID 处理通过外部 UUID 查询用户资料的请求。
// @Summary 查询用户资料
// @Description 通过外部 UUID 用户 ID 查询用户基础资料。
// @Tags 用户
// @Produce json
// @Param user_id path string true "用户ID"
// @Success 200 {object} response.Envelope{data=userhttp.UserResponseDoc} "查询成功"
// @Failure 400 {object} response.Envelope "用户 ID 参数错误"
// @Failure 401 {object} response.Envelope "未认证或 token 无效"
// @Failure 404 {object} response.Envelope "用户不存在"
// @Failure 500 {object} response.Envelope "服务器内部错误"
// @Security BearerAuth
// @Router /users/{user_id} [get]
func (ctl *UserController) GetByUserID(c *gin.Context) {
	req := GetUserRequest{}
	if !binding.BindOrAbort(ctl.validator, c, &req, binding.URIBinder) {
		return
	}
	userID, err := ParseUserID(req)
	if err != nil {
		response.Fail(c, err)
		return
	}

	user, err := ctl.userService.GetUserByID(c.Request.Context(), userID)
	if err != nil {
		response.Fail(c, toUserHTTPError(err))
		return
	}
	response.OK(c, toUserResponse(user))
}
