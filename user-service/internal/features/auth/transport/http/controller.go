package authhttp

import (
	"github.com/aegiscore/common/http/binding"
	"github.com/aegiscore/common/http/response"
	commonauth "github.com/aegiscore/common/security/auth"
	commonvalidation "github.com/aegiscore/common/validation"
	authapi "github.com/aegiscore/user-service/internal/features/auth/api"
	authapp "github.com/aegiscore/user-service/internal/features/auth/app"
	"github.com/gin-gonic/gin"
)

// AuthController 处理认证和会话端点的 HTTP 请求。
type AuthController struct {
	authService authapp.AuthService
	validator   *commonvalidation.Validator
}

// ChangePassword 使用受限 token 处理强制改密请求。
// @Summary 修改密码
// @Description 使用登录后返回的受限改密凭据修改密码，并将用户状态恢复为正常。
// @Tags 认证
// @Accept json
// @Produce json
// @Param Authorization header string true "Bearer password-change-token"
// @Param request body authapi.ChangePasswordRequest true "修改密码请求"
// @Success 200 {object} response.Envelope{data=authapi.ChangePasswordResponse} "修改成功"
// @Failure 400 {object} response.Envelope "请求体错误或参数校验失败"
// @Failure 401 {object} response.Envelope "改密凭据无效或已失效"
// @Failure 500 {object} response.Envelope "服务器内部错误"
// @Router /auth/change-password [post]
func (ctl *AuthController) ChangePassword(c *gin.Context) {
	req := authapi.ChangePasswordRequest{Token: c.GetHeader(commonauth.AuthorizationHeader)}
	if !binding.BindOrAbort(ctl.validator, c, &req, binding.JSONBinder) {
		return
	}
	if err := NormalizeChangePassword(&req); err != nil {
		response.Fail(c, err)
		return
	}
	result, err := ctl.authService.ChangePassword(c.Request.Context(), authapp.ChangePasswordCommand{
		Token:       req.Token,
		NewPassword: req.NewPassword,
	})
	if err != nil {
		response.Fail(c, toAuthHTTPError(err))
		return
	}
	response.OK(c, toChangePasswordResponse(result))
}

// NewAuthController 使用 service 和 validator 依赖构造认证控制器。
func NewAuthController(authService authapp.AuthService, validator *commonvalidation.Validator) *AuthController {
	return &AuthController{authService: authService, validator: validator}
}

// LoginUser 处理用户名和密码登录请求。
// @Summary 用户登录
// @Description 校验用户名和密码，创建可撤销会话并返回 Access Token 与 Refresh Token。
// @Tags 认证
// @Accept json
// @Produce json
// @Param request body authapi.LoginRequest true "登录请求"
// @Success 200 {object} response.Envelope{data=authapi.TokenResponse} "登录成功"
// @Failure 400 {object} response.Envelope "请求体错误或参数校验失败"
// @Failure 401 {object} response.Envelope "用户名或密码错误"
// @Failure 500 {object} response.Envelope "服务器内部错误"
// @Router /auth/login [post]
func (ctl *AuthController) LoginUser(c *gin.Context) {
	req := authapi.LoginRequest{}
	if !binding.BindOrAbort(ctl.validator, c, &req, binding.JSONBinder) {
		return
	}
	if err := NormalizeLogin(&req); err != nil {
		response.Fail(c, err)
		return
	}
	tokens, err := ctl.authService.Login(c.Request.Context(), authapp.LoginCommand{
		Username: req.Username,
		Password: req.Password,
	})
	if err != nil {
		response.Fail(c, toAuthHTTPError(err))
		return
	}
	response.OK(c, toTokenResponse(tokens))
}

// RefreshToken 处理 refresh token 换取请求。
// @Summary 刷新 Access Token
// @Description 使用仍有效且未撤销的 Refresh Token 换取新的 Access Token。
// @Tags 认证
// @Accept json
// @Produce json
// @Param request body authapi.RefreshTokenRequest true "刷新请求"
// @Success 200 {object} response.Envelope{data=authapi.TokenResponse} "刷新成功"
// @Failure 400 {object} response.Envelope "请求体错误或参数校验失败"
// @Failure 401 {object} response.Envelope "Refresh Token 无效或会话已失效"
// @Failure 500 {object} response.Envelope "服务器内部错误"
// @Router /auth/refresh [post]
func (ctl *AuthController) RefreshToken(c *gin.Context) {
	req := authapi.RefreshTokenRequest{}
	if !binding.BindOrAbort(ctl.validator, c, &req, binding.JSONBinder) {
		return
	}
	if err := NormalizeRefresh(&req); err != nil {
		response.Fail(c, err)
		return
	}
	tokens, err := ctl.authService.Refresh(c.Request.Context(), authapp.RefreshTokenCommand{
		RefreshToken: req.RefreshToken,
	})
	if err != nil {
		response.Fail(c, toAuthHTTPError(err))
		return
	}
	response.OK(c, toTokenResponse(tokens))
}

// LogoutCurrentSession 处理当前认证会话的登出请求。
// @Summary 退出当前设备
// @Description 删除当前会话的 Refresh Token 会话记录，不修改用户 token_version。
// @Tags 认证
// @Produce json
// @Success 200 {object} response.Envelope{data=authapi.LogoutResponse} "退出成功"
// @Failure 401 {object} response.Envelope "未认证或 token 无效"
// @Failure 500 {object} response.Envelope "服务器内部错误"
// @Security BearerAuth
// @Router /auth/logout [post]
func (ctl *AuthController) LogoutCurrentSession(c *gin.Context) {
	result, err := ctl.authService.Logout(c.Request.Context())
	if err != nil {
		response.Fail(c, toAuthHTTPError(err))
		return
	}
	response.OK(c, toLogoutResponse(result))
}

// LogoutAllSessions 处理认证用户全部会话的撤销请求。
// @Summary 退出全部设备
// @Description 递增用户 token_version 并清理该用户所有 Refresh Token 会话。
// @Tags 认证
// @Produce json
// @Success 200 {object} response.Envelope{data=authapi.LogoutResponse} "退出成功"
// @Failure 401 {object} response.Envelope "未认证或 token 无效"
// @Failure 500 {object} response.Envelope "服务器内部错误"
// @Security BearerAuth
// @Router /auth/logout-all [post]
func (ctl *AuthController) LogoutAllSessions(c *gin.Context) {
	result, err := ctl.authService.LogoutAll(c.Request.Context())
	if err != nil {
		response.Fail(c, toAuthHTTPError(err))
		return
	}
	response.OK(c, toLogoutResponse(result))
}
