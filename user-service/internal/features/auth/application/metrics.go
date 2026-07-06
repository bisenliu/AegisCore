package application

import "context"

//go:generate go run github.com/aegiscore/common/runtime/observability/metrics/nopgen -source metrics.go -type Metrics -output metrics_nop_gen.go -struct nopMetrics -func NopMetrics -comment "NopMetrics 返回 auth 业务指标空实现。"

const (
	// MetricsOperationLogin 表示用户名密码登录流程。
	MetricsOperationLogin = "login"
	// MetricsOperationRefresh 表示 refresh token 续签流程。
	MetricsOperationRefresh = "refresh"
	// MetricsOperationLogoutCurrent 表示退出当前会话流程。
	MetricsOperationLogoutCurrent = "logout_current"
	// MetricsOperationLogoutAll 表示退出全部会话流程。
	MetricsOperationLogoutAll = "logout_all"

	// MetricsReasonNone 表示操作成功或无需补充原因。
	MetricsReasonNone = "none"
	// MetricsReasonValidationFailed 表示输入校验失败。
	MetricsReasonValidationFailed = "validation_failed"
	// MetricsReasonCredentialInvalid 表示登录凭据无效。
	MetricsReasonCredentialInvalid = "credential_invalid" // #nosec G101 -- 指标标签值，不包含真实凭据。
	// MetricsReasonPasswordKDFBusy 表示密码 KDF 资源池繁忙。
	MetricsReasonPasswordKDFBusy = "password_kdf_busy"
	// MetricsReasonUserStatusRejected 表示用户状态拒绝登录。
	MetricsReasonUserStatusRejected = "user_status_rejected"
	// MetricsReasonPasswordChangeRequiredIssueFailed 表示强制改密 token 签发失败。
	MetricsReasonPasswordChangeRequiredIssueFailed = "password_change_required_issue_failed"
	// MetricsReasonTokenIssueFailed 表示 token 签发失败。
	MetricsReasonTokenIssueFailed = "token_issue_failed"
	// MetricsReasonSessionCreateFailed 表示 refresh 会话创建失败。
	MetricsReasonSessionCreateFailed = "session_create_failed"
	// MetricsReasonRefreshTokenInvalid 表示 refresh token 无效。
	MetricsReasonRefreshTokenInvalid = "refresh_token_invalid"
	// MetricsReasonRefreshTokenExpired 表示 refresh token 已过期。
	MetricsReasonRefreshTokenExpired = "refresh_token_expired"
	// MetricsReasonRefreshSessionInvalid 表示 refresh 会话无效。
	MetricsReasonRefreshSessionInvalid = "refresh_session_invalid"
	// MetricsReasonRefreshSessionMismatch 表示 refresh 会话与 token claims 不一致。
	MetricsReasonRefreshSessionMismatch = "refresh_session_mismatch"
	// MetricsReasonTokenVersionMismatch 表示 token version 已失效。
	MetricsReasonTokenVersionMismatch = "token_version_mismatch"
	// MetricsReasonSessionRotateFailed 表示 refresh 会话轮换失败。
	MetricsReasonSessionRotateFailed = "session_rotate_failed"
	// MetricsReasonAuthContextMissing 表示请求上下文缺少认证身份。
	MetricsReasonAuthContextMissing = "auth_context_missing"
	// MetricsReasonSessionDeleteFailed 表示删除当前会话失败。
	MetricsReasonSessionDeleteFailed = "session_delete_failed"
	// MetricsReasonSessionRevokeFailed 表示撤销全部会话失败。
	MetricsReasonSessionRevokeFailed = "session_revoke_failed"
	// MetricsReasonSystemError 表示未能稳定归类的系统异常。
	MetricsReasonSystemError = "system_error"

	// MetricsSourceAccessToken 表示 access token 校验来源。
	MetricsSourceAccessToken = "access_token"
	// MetricsSourceRefreshToken 表示 refresh token 校验来源。
	MetricsSourceRefreshToken = "refresh_token"

	// MetricsPasswordChangeReasonNotFound 表示一次性改密会话不存在、过期或已消费。
	MetricsPasswordChangeReasonNotFound = "not_found"
	// MetricsPasswordChangeReasonMismatch 表示一次性改密会话与 token claims 不一致。
	MetricsPasswordChangeReasonMismatch = "mismatch"
	// MetricsPasswordChangeReasonSystemError 表示一次性改密流程系统异常。
	MetricsPasswordChangeReasonSystemError = "system_error"

	// MetricsPasswordChangeRevocationProjection 表示改密后撤销投影链路失败。
	MetricsPasswordChangeRevocationProjection = "projection"
)

// Metrics 记录 auth feature 的低基数业务指标。
type Metrics interface {
	LoginSucceeded(context.Context)
	LoginFailed(context.Context, string)
	RefreshSucceeded(context.Context)
	RefreshFailed(context.Context, string)
	LogoutSucceeded(context.Context, string)
	LogoutFailed(context.Context, string, string)
	TokenVersionMismatch(context.Context, string)
	SessionPurgeSubmitFailed(context.Context)
	PasswordChangeSessionConsumeFailed(context.Context, string)
	PasswordChangeSessionReuseRejected(context.Context)
	PasswordChangeRevocationProjectionFailed(context.Context, string)
	PasswordChangeRevocationCompensationFailed(context.Context, string)
}
