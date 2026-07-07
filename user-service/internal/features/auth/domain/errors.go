package domain

import (
	stderrors "errors"

	contracterrors "github.com/aegiscore/common/contract/errors"
	"github.com/aegiscore/user-service/internal/messages"
)

const (
	reasonInvalidCredentials            contracterrors.Reason = "invalid_credentials"
	reasonUserStatusRejected            contracterrors.Reason = "user_status_rejected"
	reasonMissingSession                contracterrors.Reason = "missing_session"
	reasonAuthTokenInvalid              contracterrors.Reason = "auth_token_invalid"
	reasonAuthSessionNotFound           contracterrors.Reason = "auth_session_not_found"
	reasonAuthSessionMismatch           contracterrors.Reason = "auth_session_mismatch"
	reasonPasswordChangeSessionNotFound contracterrors.Reason = "password_change_session_not_found"
	reasonPasswordChangeSessionMismatch contracterrors.Reason = "password_change_session_mismatch"
	reasonSessionRevocationIncomplete   contracterrors.Reason = "session_revocation_incomplete"
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
