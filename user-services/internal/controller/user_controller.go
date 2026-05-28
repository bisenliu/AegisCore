package controller

import (
	"strconv"

	"github.com/aegiscore/common/response"
	"github.com/aegiscore/user-services/internal/dto"
	"github.com/aegiscore/user-services/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/go-playground/validator/v10"
)

type UserController struct {
	service   service.UserService
	validator *validator.Validate
}

func NewUserController(service service.UserService) *UserController {
	return &UserController{service: service, validator: validator.New(validator.WithRequiredStructEnabled())}
}

func (ctl *UserController) GetByID(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "invalid user id")
		return
	}

	req := dto.GetUserRequest{ID: id}
	if err := ctl.validator.Struct(req); err != nil {
		response.BadRequest(c, "invalid user id")
		return
	}

	user, err := ctl.service.GetUserByID(c.Request.Context(), req.ID)
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, user)
}
