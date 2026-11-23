package command

import (
	"context"
	"errors"
	"time"

	"go.uber.org/zap"

	"github.com/aegiscore/platform/runtime/logger"
	authapplication "github.com/aegiscore/user-service/internal/features/auth/application"
	authdomain "github.com/aegiscore/user-service/internal/features/auth/domain"
)

// LoginLockoutPolicy 是登录 use case 消费的账号锁定阈值。
type LoginLockoutPolicy struct {
	Enabled       bool
	MaxFailures   int
	FailureWindow time.Duration
	LockoutTTL    time.Duration
}

type loginLockoutGuard struct {
	store    authapplication.LoginLockoutStore
	policy   LoginLockoutPolicy
	disabled bool
}

// newLoginLockoutGuard 在锁定启用时要求存储可用；禁用时不访问存储。
func newLoginLockoutGuard(store authapplication.LoginLockoutStore, policy LoginLockoutPolicy) (*loginLockoutGuard, error) {
	if !policy.Enabled {
		return &loginLockoutGuard{disabled: true}, nil
	}
	if store == nil {
		return nil, errors.New("login lockout store is required when enabled")
	}
	if policy.MaxFailures <= 0 || policy.FailureWindow <= 0 || policy.LockoutTTL <= 0 {
		return nil, errors.New("login lockout policy must be positive when enabled")
	}
	return &loginLockoutGuard{store: store, policy: policy}, nil
}

// rejectIfLocked 在密码已经核对通过后拒绝仍处于锁定窗口的用户名。
// Redis 无法确认状态时拒绝登录，避免锁定存储故障变成可绕过的爆破窗口。
func (g *loginLockoutGuard) rejectIfLocked(ctx context.Context, username string) error {
	if g == nil || g.disabled {
		return nil
	}
	status, err := g.store.LoginLockoutStatus(ctx, username)
	if err != nil {
		logger.Error(ctx, "login lockout status unavailable", logger.StackTrace(zap.Error(err))...)
		return authdomain.ErrLoginLockoutUnavailable
	}
	if status.Locked {
		logger.Warn(ctx, "login rejected by account lockout")
		return authdomain.ErrLoginLocked
	}
	return nil
}

// recordCredentialFailure 只记录用户名不存在或密码不匹配。
// 达到阈值时对外返回锁定错误；未达阈值时保留调用方原来的统一无效凭据。
func (g *loginLockoutGuard) recordCredentialFailure(ctx context.Context, username string, cause error) error {
	if g == nil || g.disabled || !errors.Is(cause, authdomain.ErrInvalidCredentials) || errors.Is(cause, authdomain.ErrUserStatusRejected) {
		return cause
	}
	status, err := g.store.RegisterLoginFailure(ctx, username, g.policy.MaxFailures, g.policy.FailureWindow, g.policy.LockoutTTL)
	if err != nil {
		logger.Error(ctx, "register login failure unavailable", logger.StackTrace(zap.Error(err))...)
		return authdomain.ErrLoginLockoutUnavailable
	}
	if status.Locked {
		logger.Warn(ctx, "login locked after repeated credential failures")
		return authdomain.ErrLoginLocked
	}
	return cause
}

// clearOnSuccess 在签发 token 前清零失败计数。
// 清零失败不能继续签发 token，否则下一次失败可能立刻沿用旧计数锁定刚登录成功的账号。
func (g *loginLockoutGuard) clearOnSuccess(ctx context.Context, username string) error {
	if g == nil || g.disabled {
		return nil
	}
	if err := g.store.ClearLoginFailures(ctx, username); err != nil {
		logger.Error(ctx, "clear login failures unavailable", logger.StackTrace(zap.Error(err))...)
		return authdomain.ErrLoginLockoutUnavailable
	}
	return nil
}
