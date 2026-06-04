package controller

import (
	"github.com/aegiscore/common/contract/response"
	"github.com/aegiscore/common/http/ginvalidation"
	"github.com/aegiscore/common/security/auth"
	commonvalidation "github.com/aegiscore/common/validation"
	"github.com/aegiscore/user-services/internal/dto"
	"github.com/aegiscore/user-services/internal/service"
	uservalidation "github.com/aegiscore/user-services/internal/validation"
	"github.com/gin-gonic/gin"
)

type AuthController struct {
	authService service.AuthService
	validator   *commonvalidation.Validator
}

// ChangePassword godoc
// @Summary 修改密码
// @Description 使用登录后返回的受限改密凭据修改密码，并将用户状态恢复为正常。
// @Tags 认证
// @Accept json
// @Produce json
// @Param Authorization header string true "Bearer password-change-token"
// @Param request body dto.ChangePasswordRequest true "修改密码请求"
// @Success 200 {object} response.Envelope{data=dto.ChangePasswordResponse} "修改成功"
// @Failure 400 {object} response.Envelope "请求体错误或参数校验失败"
// @Failure 401 {object} response.Envelope "改密凭据无效或已失效"
// @Failure 500 {object} response.Envelope "服务器内部错误"
// @Router /auth/change-password [post]
func (ctl *AuthController) ChangePassword(c *gin.Context) {
	req := dto.ChangePasswordRequest{Token: c.GetHeader(auth.AuthorizationHeader)}
	if !ginvalidation.BindOrAbort(ctl.validator, c, &req, ginvalidation.JSONBinder) {
		return
	}
	if err := uservalidation.NormalizeChangePassword(&req); err != nil {
		response.Fail(c, err)
		return
	}
	result, err := ctl.authService.ChangePassword(c.Request.Context(), req)
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, result)
}

func NewAuthController(authService service.AuthService, validator *commonvalidation.Validator) *AuthController {
	return &AuthController{authService: authService, validator: validator}
}

// LoginUser godoc
// @Summary 用户登录
// @Description 校验用户名和密码，创建可撤销会话并返回 Access Token 与 Refresh Token。
// @Tags 认证
// @Accept json
// @Produce json
// @Param request body dto.LoginRequest true "登录请求"
// @Success 200 {object} response.Envelope{data=dto.TokenResponse} "登录成功"
// @Failure 400 {object} response.Envelope "请求体错误或参数校验失败"
// @Failure 401 {object} response.Envelope "用户名或密码错误"
// @Failure 500 {object} response.Envelope "服务器内部错误"
// @Router /auth/login [post]
func (ctl *AuthController) LoginUser(c *gin.Context) {
	req := dto.LoginRequest{}
	if !ginvalidation.BindOrAbort(ctl.validator, c, &req, ginvalidation.JSONBinder) {
		return
	}
	if err := uservalidation.NormalizeLogin(&req); err != nil {
		response.Fail(c, err)
		return
	}
	tokens, err := ctl.authService.Login(c.Request.Context(), req)
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, tokens)
}

// RefreshToken godoc
// @Summary 刷新 Access Token
// @Description 使用仍有效且未撤销的 Refresh Token 换取新的 Access Token。
// @Tags 认证
// @Accept json
// @Produce json
// @Param request body dto.RefreshTokenRequest true "刷新请求"
// @Success 200 {object} response.Envelope{data=dto.TokenResponse} "刷新成功"
// @Failure 400 {object} response.Envelope "请求体错误或参数校验失败"
// @Failure 401 {object} response.Envelope "Refresh Token 无效或会话已失效"
// @Failure 500 {object} response.Envelope "服务器内部错误"
// @Router /auth/refresh [post]
func (ctl *AuthController) RefreshToken(c *gin.Context) {
	req := dto.RefreshTokenRequest{}
	if !ginvalidation.BindOrAbort(ctl.validator, c, &req, ginvalidation.JSONBinder) {
		return
	}
	if err := uservalidation.NormalizeRefresh(&req); err != nil {
		response.Fail(c, err)
		return
	}
	tokens, err := ctl.authService.Refresh(c.Request.Context(), req)
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, tokens)
}

// LogoutCurrentSession godoc
// @Summary 退出当前设备
// @Description 删除当前会话的 Refresh Token 会话记录，不修改用户 token_version。
// @Tags 认证
// @Produce json
// @Success 200 {object} response.Envelope{data=dto.LogoutResponse} "退出成功"
// @Failure 401 {object} response.Envelope "未认证或 token 无效"
// @Failure 500 {object} response.Envelope "服务器内部错误"
// @Security BearerAuth
// @Router /auth/logout [post]
func (ctl *AuthController) LogoutCurrentSession(c *gin.Context) {
	result, err := ctl.authService.Logout(c.Request.Context())
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, result)
}

// LogoutAllSessions godoc
// @Summary 退出全部设备
// @Description 递增用户 token_version 并清理该用户所有 Refresh Token 会话。
// @Tags 认证
// @Produce json
// @Success 200 {object} response.Envelope{data=dto.LogoutResponse} "退出成功"
// @Failure 401 {object} response.Envelope "未认证或 token 无效"
// @Failure 500 {object} response.Envelope "服务器内部错误"
// @Security BearerAuth
// @Router /auth/logout-all [post]
func (ctl *AuthController) LogoutAllSessions(c *gin.Context) {
	result, err := ctl.authService.LogoutAll(c.Request.Context())
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, result)
}
