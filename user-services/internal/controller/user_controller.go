package controller

import (
	"github.com/aegiscore/common/response"
	"github.com/aegiscore/common/validation"
	"github.com/aegiscore/user-services/internal/dto"
	"github.com/aegiscore/user-services/internal/service"
	"github.com/gin-gonic/gin"
)

type UserController struct {
	userService service.UserService
	validator   *validation.Validator
}

func NewUserController(userService service.UserService, validator *validation.Validator) *UserController {
	return &UserController{userService: userService, validator: validator}
}

// List godoc
// @Summary 分页查询用户列表
// @Description 分页查询用户资料列表，支持按用户昵称、用户名和启用状态过滤。分页参数未传或小于 1 时使用默认值 page=1、page_size=10。
// @Tags 用户
// @Produce json
// @Param page query int false "页码，未传或小于 1 时默认为 1" minimum(1)
// @Param page_size query int false "每页数量，未传或小于 1 时默认为 10" minimum(1)
// @Param name query string false "用户昵称模糊匹配"
// @Param username query string false "用户名精确匹配"
// @Param active query bool false "是否启用"
// @Success 200 {object} response.Envelope{data=dto.UserListResponseDoc} "查询成功"
// @Failure 400 {object} response.Envelope "查询参数错误"
// @Failure 401 {object} response.Envelope "未认证或 token 无效"
// @Failure 500 {object} response.Envelope "服务器内部错误"
// @Security BearerAuth
// @Router /users [get]
func (ctl *UserController) List(c *gin.Context) {
	req := dto.ListUsersRequest{}
	if !ctl.validator.BindOrAbort(c, &req, validation.QueryBinder) {
		return
	}

	users, err := ctl.userService.ListUsers(c.Request.Context(), req)
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, users)
}

// Create godoc
// @Summary 创建用户
// @Description 创建一个新的用户资料。请求体使用共享校验器校验，用户名必须唯一，active 缺省为 true。
// @Tags 用户
// @Accept json
// @Produce json
// @Param request body dto.CreateUserRequest true "创建用户请求"
// @Success 201 {object} response.Envelope{data=dto.UserResponse} "创建成功"
// @Failure 400 {object} response.Envelope "请求体错误或参数校验失败"
// @Failure 401 {object} response.Envelope "未认证或 token 无效"
// @Failure 409 {object} response.Envelope "用户已存在"
// @Failure 500 {object} response.Envelope "服务器内部错误"
// @Security BearerAuth
// @Router /users [post]
func (ctl *UserController) Create(c *gin.Context) {
	req := dto.CreateUserRequest{}
	if !ctl.validator.BindOrAbort(c, &req, validation.JSONBinder) {
		return
	}

	user, err := ctl.userService.CreateUser(c.Request.Context(), req)
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.Created(c, user)
}

// GetByID godoc
// @Summary 查询用户资料
// @Description 通过外部 UUID 用户 ID 查询用户基础资料。
// @Tags 用户
// @Produce json
// @Param user_id path string true "用户ID"
// @Success 200 {object} response.Envelope{data=dto.UserResponse} "查询成功"
// @Failure 400 {object} response.Envelope "用户 ID 参数错误"
// @Failure 401 {object} response.Envelope "未认证或 token 无效"
// @Failure 404 {object} response.Envelope "用户不存在"
// @Failure 500 {object} response.Envelope "服务器内部错误"
// @Security BearerAuth
// @Router /users/{user_id} [get]
func (ctl *UserController) GetByID(c *gin.Context) {
	req := dto.GetUserRequest{}
	if !ctl.validator.BindOrAbort(c, &req, validation.URIBinder) {
		return
	}

	user, err := ctl.userService.GetUserByID(c.Request.Context(), req.UserID)
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, user)
}
