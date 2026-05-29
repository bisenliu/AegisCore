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
