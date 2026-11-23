package redis

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"time"

	rediscache "github.com/redis/go-redis/v9"
	"golang.org/x/text/cases"

	authapplication "github.com/aegiscore/user-service/internal/features/auth/application"
)

const loginLockoutValueLocked = "locked"

// LoginLockoutStatus 读取用户名当前是否已锁定。
func (s *SessionStore) LoginLockoutStatus(ctx context.Context, username string) (authapplication.LoginLockoutStatus, error) {
	if s == nil || s.redis == nil {
		return authapplication.LoginLockoutStatus{}, fmt.Errorf("login lockout store is not configured")
	}
	value, err := s.redis.Get(ctx, s.loginLockoutKey(username)).Result()
	if err != nil {
		if errors.Is(err, rediscache.Nil) {
			return authapplication.LoginLockoutStatus{}, nil
		}
		return authapplication.LoginLockoutStatus{}, fmt.Errorf("get login lockout: %w", err)
	}
	return authapplication.LoginLockoutStatus{Locked: value == loginLockoutValueLocked}, nil
}

// RegisterLoginFailure 原子增加失败次数，达到阈值后把同一记录改为锁定状态。
func (s *SessionStore) RegisterLoginFailure(ctx context.Context, username string, maxFailures int, failureWindow time.Duration, lockoutTTL time.Duration) (authapplication.LoginLockoutStatus, error) {
	if s == nil || s.redis == nil {
		return authapplication.LoginLockoutStatus{}, fmt.Errorf("login lockout store is not configured")
	}
	if maxFailures <= 0 || failureWindow <= 0 || lockoutTTL <= 0 {
		return authapplication.LoginLockoutStatus{}, fmt.Errorf("login lockout policy must be positive")
	}
	result, err := registerLoginFailureScript.Run(
		ctx,
		s.redis,
		[]string{s.loginLockoutKey(username)},
		maxFailures,
		failureWindow.Milliseconds(),
		lockoutTTL.Milliseconds(),
	).Slice()
	if err != nil {
		return authapplication.LoginLockoutStatus{}, fmt.Errorf("register login failure: %w", err)
	}
	locked, err := scriptFlag(result, 1)
	if err != nil {
		return authapplication.LoginLockoutStatus{}, fmt.Errorf("parse login lockout result: %w", err)
	}
	return authapplication.LoginLockoutStatus{Locked: locked}, nil
}

// ClearLoginFailures 删除用户名的失败计数；删除已不存在的计数也视为成功。
func (s *SessionStore) ClearLoginFailures(ctx context.Context, username string) error {
	if s == nil || s.redis == nil {
		return fmt.Errorf("login lockout store is not configured")
	}
	if err := s.redis.Del(ctx, s.loginLockoutKey(username)).Err(); err != nil {
		return fmt.Errorf("clear login failures: %w", err)
	}
	return nil
}

// loginLockoutKey 用用户名摘要构造 Redis key，避免把原始用户名写入 key、日志或错误文本。
func (s *SessionStore) loginLockoutKey(username string) string {
	sum := sha256.Sum256([]byte(normalizeLoginUsername(username)))
	return s.keys.AuthLoginLockout(hex.EncodeToString(sum[:]))
}

// normalizeLoginUsername 统一大小写和首尾空白，避免同一账号用不同写法绕过锁定。
func normalizeLoginUsername(username string) string {
	return cases.Fold().String(strings.TrimSpace(username))
}

// scriptFlag 读取 Lua 数组返回值中的 0/1 标记。
func scriptFlag(values []any, index int) (bool, error) {
	if index < 0 || index >= len(values) {
		return false, fmt.Errorf("missing script flag %d", index)
	}
	switch value := values[index].(type) {
	case int64:
		return value == 1, nil
	case string:
		return value == "1", nil
	default:
		return false, fmt.Errorf("unexpected script flag type %T", values[index])
	}
}
