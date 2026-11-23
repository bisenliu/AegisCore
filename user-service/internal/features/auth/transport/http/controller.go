package authhttp

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/aegiscore/platform/http/binding"
	"github.com/aegiscore/platform/http/response"
	platformvalidation "github.com/aegiscore/platform/validation"
	"github.com/aegiscore/user-service/internal/features/auth/application/authctx"
	authcommand "github.com/aegiscore/user-service/internal/features/auth/application/command"
)

// AuthController 负责接收和处理用户登录、刷新令牌、强制改密以及退出登录等 HTTP 接口请求。
type AuthController struct {
	login         authcommand.LoginUseCase
	refresh       authcommand.RefreshTokenUseCase
	forceChange   authcommand.ForceChangePasswordUseCase
	logoutCurrent authcommand.LogoutCurrentSessionUseCase
	logoutAll     authcommand.LogoutAllSessionsUseCase
	validator     *platformvalidation.Validator
}

// AuthControllerOptions 包含构造认证控制器所需的普通依赖。
type AuthControllerOptions struct {
	Login         authcommand.LoginUseCase
	Refresh       authcommand.RefreshTokenUseCase
	ForceChange   authcommand.ForceChangePasswordUseCase
	LogoutCurrent authcommand.LogoutCurrentSessionUseCase
	LogoutAll     authcommand.LogoutAllSessionsUseCase
	Validator     *platformvalidation.Validator
}

// NewAuthController 创建认证接口控制器，组装认证业务用例与请求校验服务。
func NewAuthController(options AuthControllerOptions) *AuthController {
	return &AuthController{
		login:         options.Login,
		refresh:       options.Refresh,
		forceChange:   options.ForceChange,
		logoutCurrent: options.LogoutCurrent,
		logoutAll:     options.LogoutAll,
		validator:     options.Validator,
	}
}

// LoginUser 用户登录接口（支持账号密码验证及强制改密流程）。
// @Summary 用户登录
// @Description 校验用户名和密码；普通登录返回 success=true、code=0、Access Token 与 Refresh Token，强制改密登录返回 success=false、code=20006 与受限改密 Token。
// @Tags 认证
// @Accept json
// @Produce json
// @Param request body authhttp.LoginRequest true "登录请求"
// @Success 200 {object} response.Envelope{data=authhttp.TokenResponse} "登录成功或需要强制改密"
// @Failure 400 {object} response.Envelope "请求体错误或参数校验失败"
// @Failure 401 {object} response.Envelope "用户名或密码错误"
// @Failure 403 {object} response.Envelope "登录尝试过于频繁，账号已临时锁定"
// @Failure 429 {object} response.Envelope "请求过于频繁"
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

// RefreshToken 刷新访问令牌接口。
// @Summary 刷新 Access Token
// @Description 使用仍有效且未撤销的 Refresh Token 换取新的 Access Token。
// @Tags 认证
// @Accept json
// @Produce json
// @Param request body authhttp.RefreshTokenRequest true "刷新请求"
// @Success 200 {object} response.Envelope{data=authhttp.TokenResponse} "刷新成功"
// @Failure 400 {object} response.Envelope "请求体错误或参数校验失败"
// @Failure 401 {object} response.Envelope "Refresh Token 无效或会话已失效"
// @Failure 429 {object} response.Envelope "请求过于频繁"
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

// ForceChangePassword 强制修改密码接口（需携带临时受限改密凭据）。
// @Summary 强制修改密码
// @Description 使用登录后返回的 password-change 受限凭据完成强制改密，并将用户状态恢复为正常。
// @Tags 认证
// @Accept json
// @Produce json
// @Param Authorization header string true "Bearer password-change-token"
// @Param request body authhttp.ForceChangePasswordRequest true "强制改密请求"
// @Success 200 {object} response.Envelope{data=authhttp.ForceChangePasswordResponse} "强制改密成功"
// @Failure 400 {object} response.Envelope "请求体错误或参数校验失败"
// @Failure 401 {object} response.Envelope "改密凭据无效或已失效"
// @Failure 429 {object} response.Envelope "请求过于频繁"
// @Failure 503 {object} response.Envelope "认证安全撤销未完成，请稍后重试"
// @Failure 500 {object} response.Envelope "服务器内部错误"
// @Router /auth/force-change-password [post]
func (ctl *AuthController) ForceChangePassword(c *gin.Context) {
	req := ForceChangePasswordRequest{}
	if !binding.BindOrAbort(ctl.validator, c, &req, binding.Compose(binding.HeaderBinder, binding.JSONBinder)) {
		return
	}

	cmd, err := prepareForceChangePasswordCommand(req)
	if err != nil {
		response.Fail(c, err)
		return
	}
	result, err := ctl.forceChange.ForceChangePassword(c.Request.Context(), cmd)
	if err != nil {
		response.Fail(c, err)
		return
	}
	response.OK(c, toForceChangePasswordResponse(result))
}

// LogoutCurrentSession 退出当前登录会话接口。
// @Summary 退出当前设备
// @Description 删除当前 Refresh Token 会话，并将当前 Access Token 加入剩余有效期拒绝名单，不修改用户 token_version。
// @Tags 认证
// @Produce json
// @Success 200 {object} response.Envelope{data=authhttp.LogoutResponse} "退出成功"
// @Failure 401 {object} response.Envelope "未认证或 token 无效"
// @Failure 429 {object} response.Envelope "请求过于频繁"
// @Failure 503 {object} response.Envelope "当前 Access Token 尚未完成吊销"
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

// LogoutAllSessions 退出该用户在所有设备上的登录状态接口。
// @Summary 退出全部设备
// @Description 递增用户 token_version 并清理该用户所有 Refresh Token 会话。
// @Tags 认证
// @Produce json
// @Success 200 {object} response.Envelope{data=authhttp.LogoutResponse} "退出成功"
// @Failure 401 {object} response.Envelope "未认证或 token 无效"
// @Failure 429 {object} response.Envelope "请求过于频繁"
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
