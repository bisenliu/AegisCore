package domain

import "errors"

// ErrInvalidCredentials 表示登录凭据不能认证用户。
var ErrInvalidCredentials = errors.New("invalid credentials")

// ErrUserStatusRejected 表示用户状态拒绝登录；对外仍映射为无效凭据。
var ErrUserStatusRejected = errors.New("user status rejected")

// ErrMissingSession 表示当前上下文缺少有效认证会话。
var ErrMissingSession = errors.New("missing authenticated session")

// ErrTokenInvalid 表示认证 token、改密 token 或 refresh 会话无效。
var ErrTokenInvalid = errors.New("token invalid")

// ErrAuthSessionNotFound 表示 refresh token 会话不存在或已过期。
var ErrAuthSessionNotFound = errors.New("auth session not found")

// ErrAuthSessionMismatch 表示 refresh token 会话与预期用户或版本不一致。
var ErrAuthSessionMismatch = errors.New("auth session mismatch")

// ErrTokenVersionCacheMiss 表示 token version 缓存未命中，需要从持久化存储回填。
var ErrTokenVersionCacheMiss = errors.New("token version cache miss")

// ErrPasswordChangeSessionNotFound 表示强制改密一次性会话不存在、过期或已消费。
var ErrPasswordChangeSessionNotFound = errors.New("password change session not found")

// ErrPasswordChangeSessionMismatch 表示强制改密一次性会话与 token claims 不一致。
var ErrPasswordChangeSessionMismatch = errors.New("password change session mismatch")

// ErrSessionRevocationIncomplete 表示安全敏感会话撤销投影未完整完成。
var ErrSessionRevocationIncomplete = errors.New("session revocation incomplete")
