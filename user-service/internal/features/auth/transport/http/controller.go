package authhttp

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"go.uber.org/fx"

	"github.com/aegiscore/common/http/binding"
	"github.com/aegiscore/common/http/response"
	commonvalidation "github.com/aegiscore/common/validation"
	"github.com/aegiscore/user-service/internal/features/auth/application/authctx"
	authcommand "github.com/aegiscore/user-service/internal/features/auth/application/command"
)

// AuthController 处理认证和会话端点的 HTTP 请求。
type AuthController struct {
	login          authcommand.LoginUseCase
	refresh        authcommand.RefreshTokenUseCase
	changePassword authcommand.ChangePasswordUseCase
	logoutCurrent  authcommand.LogoutCurrentSessionUseCase
	logoutAll      authcommand.LogoutAllSessionsUseCase
	validator      *commonvalidation.Validator
}

// AuthControllerParams 包含构造认证控制器所需的依赖。
type AuthControllerParams struct {
	fx.In

	Login          authcommand.LoginUseCase
	Refresh        authcommand.RefreshTokenUseCase
	ChangePassword authcommand.ChangePasswordUseCase
	LogoutCurrent  authcommand.LogoutCurrentSessionUseCase
	LogoutAll      authcommand.LogoutAllSessionsUseCase
	Validator      *commonvalidation.Validator
}

// NewAuthController 使用 command use case 和 validator 依赖构造认证控制器。
func NewAuthController(params AuthControllerParams) *AuthController {
	return &AuthController{
		login:          params.Login,
		refresh:        params.Refresh,
		changePassword: params.ChangePassword,
		logoutCurrent:  params.LogoutCurrent,
		logoutAll:      params.LogoutAll,
		validator:      params.Validator,
	}
}

// LoginUser 处理用户名和密码登录请求。
// @Summary 用户登录
// @Description 校验用户名和密码；普通登录返回 success=true、code=0、Access Token 与 Refresh Token，强制改密登录返回 success=false、code=20006 与受限改密 Token。
// @Tags 认证
// @Accept json
// @Produce json
// @Param request body authhttp.LoginRequest true "登录请求"
// @Success 200 {object} response.Envelope{data=authhttp.TokenResponse} "登录成功或需要强制改密"
// @Failure 400 {object} response.Envelope "请求体错误或参数校验失败"
// @Failure 401 {object} response.Envelope "用户名或密码错误"
// @Failure 503 {object} response.Envelope "认证服务繁忙"
// @Failure 500 {object} response.Envelope "服务器内部错误"
// @Router /auth/login [post]
func (ctl *AuthController) LoginUser(c *gin.Context) {
	req := LoginRequest{}
	if !binding.BindOrAbort(ctl.validator, c, &req, binding.JSONBinder) {
		return
	}

	cmd, err := prepareLoginCommand(req)
	if err != nil {
		response.Fail(c, err)
		return
	}
	ctx := authctx.WithClientContext(c.Request.Context(), authctx.ClientContext{
		ClientIP:  c.ClientIP(),
		UserAgent: c.GetHeader("User-Agent"),
	})
	result, err := ctl.login.Login(ctx, cmd)
	if err != nil {
		response.Fail(c, err)
		return
	}
	if result.PasswordChangeRequired {
		response.JSON(c, http.StatusOK, toPasswordChangeRequiredEnvelope(result.Tokens))
		return
	}
	response.OK(c, toTokenResponse(result.Tokens))
}

// RefreshToken 处理 refresh token 换取请求。
// @Summary 刷新 Access Token
// @Description 使用仍有效且未撤销的 Refresh Token 换取新的 Access Token。
// @Tags 认证
// @Accept json
// @Produce json
// @Param request body authhttp.RefreshTokenRequest true "刷新请求"
// @Success 200 {object} response.Envelope{data=authhttp.TokenResponse} "刷新成功"
// @Failure 400 {object} response.Envelope "请求体错误或参数校验失败"
// @Failure 401 {object} response.Envelope "Refresh Token 无效或会话已失效"
// @Failure 500 {object} response.Envelope "服务器内部错误"
// @Router /auth/refresh [post]
func (ctl *AuthController) RefreshToken(c *gin.Context) {
	req := RefreshTokenRequest{}
	if !binding.BindOrAbort(ctl.validator, c, &req, binding.JSONBinder) {
		return
	}

	cmd, err := prepareRefreshTokenCommand(req)
	if err != nil {
		response.Fail(c, err)
		return
	}
	tokens, err := ctl.refresh.Refresh(c.Request.Context(), cmd)
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, toTokenResponse(tokens))
}

// ChangePassword 使用受限 token 处理强制改密请求。
// @Summary 修改密码
// @Description 使用登录后返回的受限改密凭据修改密码，并将用户状态恢复为正常。
// @Tags 认证
// @Accept json
// @Produce json
// @Param Authorization header string true "Bearer password-change-token"
// @Param request body authhttp.ChangePasswordRequest true "修改密码请求"
// @Success 200 {object} response.Envelope{data=authhttp.ChangePasswordResponse} "修改成功"
// @Failure 400 {object} response.Envelope "请求体错误或参数校验失败"
// @Failure 401 {object} response.Envelope "改密凭据无效或已失效"
// @Failure 503 {object} response.Envelope "认证安全撤销未完成，请稍后重试"
// @Failure 500 {object} response.Envelope "服务器内部错误"
// @Router /auth/change-password [post]
func (ctl *AuthController) ChangePassword(c *gin.Context) {
	req := ChangePasswordRequest{}
	if !binding.BindOrAbort(ctl.validator, c, &req, binding.Compose(binding.HeaderBinder, binding.JSONBinder)) {
		return
	}

	cmd, err := prepareChangePasswordCommand(req)
	if err != nil {
		response.Fail(c, err)
		return
	}
	result, err := ctl.changePassword.ChangePassword(c.Request.Context(), cmd)
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, toChangePasswordResponse(result))
}

// LogoutCurrentSession 处理当前认证会话的登出请求。
// @Summary 退出当前设备
// @Description 删除当前会话的 Refresh Token 会话记录，不修改用户 token_version。
// @Tags 认证
// @Produce json
// @Success 200 {object} response.Envelope{data=authhttp.LogoutResponse} "退出成功"
// @Failure 401 {object} response.Envelope "未认证或 token 无效"
// @Failure 500 {object} response.Envelope "服务器内部错误"
// @Security BearerAuth
// @Router /auth/logout [post]
func (ctl *AuthController) LogoutCurrentSession(c *gin.Context) {
	result, err := ctl.logoutCurrent.LogoutCurrentSession(c.Request.Context())
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, toLogoutResponse(result))
}

// LogoutAllSessions 处理认证用户全部会话的撤销请求。
// @Summary 退出全部设备
// @Description 递增用户 token_version 并清理该用户所有 Refresh Token 会话。
// @Tags 认证
// @Produce json
// @Success 200 {object} response.Envelope{data=authhttp.LogoutResponse} "退出成功"
// @Failure 401 {object} response.Envelope "未认证或 token 无效"
// @Failure 500 {object} response.Envelope "服务器内部错误"
// @Security BearerAuth
// @Router /auth/logout-all [post]
func (ctl *AuthController) LogoutAllSessions(c *gin.Context) {
	result, err := ctl.logoutAll.LogoutAllSessions(c.Request.Context())
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, toLogoutResponse(result))
}
