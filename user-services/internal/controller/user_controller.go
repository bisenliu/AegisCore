package controller

import (
	"github.com/aegiscore/common/response"
	"github.com/aegiscore/common/validation"
	"github.com/aegiscore/user-services/internal/dto"
	"github.com/aegiscore/user-services/internal/service"
	"github.com/gin-gonic/gin"
)

type UserController struct {
	service   service.UserService
	validator *validation.Validator
}

func NewUserController(service service.UserService, validator *validation.Validator) *UserController {
	return &UserController{service: service, validator: validator}
}

// Create godoc
// @Summary 创建用户
// @Description 创建一个新的用户资料。请求体使用共享校验器校验，邮箱必须唯一，active 缺省为 true。
// @Tags 用户
// @Accept json
// @Produce json
// @Param request body dto.CreateUserRequest true "创建用户请求"
// @Success 201 {object} response.Envelope{data=dto.UserResponse} "创建成功"
// @Failure 400 {object} response.Envelope "请求体错误或参数校验失败"
// @Failure 409 {object} response.Envelope "用户已存在"
// @Failure 500 {object} response.Envelope "服务器内部错误"
// @Router /users [post]
func (ctl *UserController) Create(c *gin.Context) {
	req := dto.CreateUserRequest{}
	if !ctl.validator.BindOrAbort(c, &req, validation.JSONBinder) {
		return
	}

	user, err := ctl.service.CreateUser(c.Request.Context(), req)
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.Created(c, user)
}

// GetByID godoc
// @Summary 查询用户资料
// @Description 通过正整数用户 ID 查询用户基础资料。
// @Tags 用户
// @Produce json
// @Param id path int true "用户ID" minimum(1)
// @Success 200 {object} response.Envelope{data=dto.UserResponse} "查询成功"
// @Failure 400 {object} response.Envelope "用户 ID 参数错误"
// @Failure 404 {object} response.Envelope "用户不存在"
// @Failure 500 {object} response.Envelope "服务器内部错误"
// @Router /users/{id} [get]
func (ctl *UserController) GetByID(c *gin.Context) {
	req := dto.GetUserRequest{}
	if !ctl.validator.BindOrAbort(c, &req, validation.URIBinder) {
		return
	}

	user, err := ctl.service.GetUserByID(c.Request.Context(), req.ID)
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, user)
}
