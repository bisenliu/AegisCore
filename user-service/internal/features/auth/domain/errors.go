package domain

import (
	stderrors "errors"

	contracterrors "github.com/aegiscore/platform/contract/errors"
	"github.com/aegiscore/user-service/internal/messages"
)

const (
	// reasonInvalidCredentials 是账号或密码错误等凭据不匹配的错误原因标识。
	reasonInvalidCredentials contracterrors.Reason = "invalid_credentials" // #nosec G101 -- 错误 reason，不包含真实凭据。
	// reasonUserStatusRejected 是用户账号状态不可用（如已被禁用或待激活）的错误原因标识。
	reasonUserStatusRejected contracterrors.Reason = "user_status_rejected"
	// reasonMissingSession 是上下文缺少必要用户会话凭证的错误原因标识。
	reasonMissingSession contracterrors.Reason = "missing_session"
	// reasonAuthTokenInvalid 是 Token 解析校验失败（签名错误、格式不合法或过期）的错误原因标识。
	reasonAuthTokenInvalid contracterrors.Reason = "auth_token_invalid" // #nosec G101 -- 错误 reason，不包含真实凭据。
	// reasonAuthSessionNotFound 是根据会话 ID 在存储中未找到活跃会话的错误原因标识。
	reasonAuthSessionNotFound contracterrors.Reason = "auth_session_not_found"
	// reasonAuthSessionMismatch 是请求提供的凭据与服务端存储会话上下文不一致的错误原因标识。
	reasonAuthSessionMismatch contracterrors.Reason = "auth_session_mismatch"
	// reasonPasswordChangeSessionNotFound 是强制改密一次性会话不存在或已过期的错误原因标识。
	reasonPasswordChangeSessionNotFound contracterrors.Reason = "password_change_session_not_found"
	// reasonPasswordChangeSessionMismatch 是强制改密会话与当前用户身份不匹配的错误原因标识。
	reasonPasswordChangeSessionMismatch contracterrors.Reason = "password_change_session_mismatch"
	// reasonSessionRevocationIncomplete 是改密或退出时撤销会话未能完全成功的错误原因标识。
	reasonSessionRevocationIncomplete contracterrors.Reason = "session_revocation_incomplete"
	// reasonLoginLockoutUnavailable 是登录锁定状态暂时无法确认的错误原因标识。
	reasonLoginLockoutUnavailable contracterrors.Reason = "login_lockout_unavailable"
	// reasonAccessDenylistUnavailable 是 access token 拒绝名单暂时无法确认的错误原因标识。
	reasonAccessDenylistUnavailable contracterrors.Reason = "access_denylist_unavailable"
)

// ErrInvalidCredentials 表示登录凭据不能认证用户。
var ErrInvalidCredentials = contracterrors.New(contracterrors.KindUnauthenticated, reasonInvalidCredentials, contracterrors.CodeUnauthenticated, messages.InvalidCredentials)

// ErrUserStatusRejected 表示用户状态拒绝登录；对外仍映射为无效凭据。
var ErrUserStatusRejected = contracterrors.New(contracterrors.KindUnauthenticated, reasonUserStatusRejected, contracterrors.CodeUnauthenticated, messages.InvalidCredentials)

// ErrMissingSession 表示当前上下文缺少有效认证会话。
var ErrMissingSession = contracterrors.New(contracterrors.KindUnauthenticated, reasonMissingSession, contracterrors.CodeUnauthenticated, messages.MissingSession)

// ErrTokenInvalid 表示认证 token、改密 token 或 refresh 会话无效。
var ErrTokenInvalid = contracterrors.New(contracterrors.KindUnauthenticated, reasonAuthTokenInvalid, contracterrors.CodeTokenInvalid, messages.MissingSession)

// ErrAuthSessionNotFound 表示 refresh token 会话不存在或已过期。
var ErrAuthSessionNotFound = contracterrors.New(contracterrors.KindUnauthenticated, reasonAuthSessionNotFound, contracterrors.CodeTokenInvalid, messages.MissingSession)

// ErrAuthSessionMismatch 表示 refresh token 会话与预期用户或版本不一致。
var ErrAuthSessionMismatch = contracterrors.New(contracterrors.KindUnauthenticated, reasonAuthSessionMismatch, contracterrors.CodeTokenInvalid, messages.MissingSession)

// ErrTokenVersionCacheMiss 表示 token version 缓存未命中，需要从持久化存储回填。
var ErrTokenVersionCacheMiss = stderrors.New("token version cache miss")

// ErrPasswordChangeSessionNotFound 表示强制改密一次性会话不存在、过期或已消费。
var ErrPasswordChangeSessionNotFound = contracterrors.New(contracterrors.KindUnauthenticated, reasonPasswordChangeSessionNotFound, contracterrors.CodeTokenInvalid, messages.MissingSession)

// ErrPasswordChangeSessionMismatch 表示强制改密一次性会话与 token claims 不一致。
var ErrPasswordChangeSessionMismatch = contracterrors.New(contracterrors.KindUnauthenticated, reasonPasswordChangeSessionMismatch, contracterrors.CodeTokenInvalid, messages.MissingSession)

// ErrSessionRevocationIncomplete 表示安全敏感会话撤销投影未完整完成。
var ErrSessionRevocationIncomplete = contracterrors.New(contracterrors.KindServiceUnavailable, reasonSessionRevocationIncomplete, contracterrors.CodeServiceUnavailable, messages.AuthRevocationIncomplete)

// ErrLoginLocked 表示该用户名因连续密码失败被临时锁定。
// 公开文案不包含失败次数、剩余时间和用户是否存在，避免把锁定接口变成账号探测工具。
var ErrLoginLocked = contracterrors.New(contracterrors.KindForbidden, contracterrors.ReasonUserAccountLocked, contracterrors.CodeUserAccountLocked, messages.LoginTemporarilyLocked)

// ErrLoginLockoutUnavailable 表示登录锁定状态无法确认，调用方必须拒绝登录。
var ErrLoginLockoutUnavailable = contracterrors.New(contracterrors.KindServiceUnavailable, reasonLoginLockoutUnavailable, contracterrors.CodeServiceUnavailable, messages.LoginTemporarilyUnavailable)

// ErrAccessDenylistUnavailable 表示 access token 拒绝名单无法确认，调用方必须拒绝访问。
var ErrAccessDenylistUnavailable = contracterrors.New(contracterrors.KindServiceUnavailable, reasonAccessDenylistUnavailable, contracterrors.CodeServiceUnavailable, messages.AuthRevocationIncomplete)
