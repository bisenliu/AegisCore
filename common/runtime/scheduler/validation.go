package scheduler

import (
	"fmt"
	"strings"
)

// validateJob 校验任务配置，并归一化锁策略中的默认值。
func (s *Scheduler) validateJob(cfg *JobConfig) error {
	cfg.Key = strings.TrimSpace(cfg.Key)
	cfg.Spec = strings.TrimSpace(cfg.Spec)

	if cfg.Key == "" {
		return fmt.Errorf("%w: key is required", ErrInvalidJob)
	}
	if cfg.Spec == "" {
		return fmt.Errorf("%w: spec is required", ErrInvalidJob)
	}
	if cfg.Task == nil {
		return fmt.Errorf("%w: task is required", ErrInvalidJob)
	}
	if !cfg.Lock.Enabled {
		return nil
	}
	if s.locker == nil {
		return fmt.Errorf("%w: locker is required", ErrInvalidLock)
	}
	if cfg.Lock.Mode == "" {
		cfg.Lock.Mode = LockModeSkipIfLocked
	}
	if cfg.Lock.Mode != LockModeSkipIfLocked && cfg.Lock.Mode != LockModeWait {
		return fmt.Errorf("%w: unsupported mode %q", ErrInvalidLock, cfg.Lock.Mode)
	}
	if cfg.Lock.Mode == LockModeWait && cfg.Lock.WaitTimeout <= 0 {
		return fmt.Errorf("%w: wait timeout is required when mode is wait", ErrInvalidLock)
	}
	if cfg.Lock.TTL <= 0 {
		cfg.Lock.TTL = s.defaultLockTTL
	}
	if cfg.Lock.TTL <= 0 {
		return fmt.Errorf("%w: ttl is required", ErrInvalidLock)
	}
	if cfg.Timeout > 0 && !cfg.Lock.AutoRenew && cfg.Lock.TTL <= cfg.Timeout {
		return fmt.Errorf("%w: ttl must be greater than job timeout or auto renew must be enabled", ErrInvalidLock)
	}
	if cfg.Lock.AutoRenew {
		if cfg.Lock.RenewInterval <= 0 {
			cfg.Lock.RenewInterval = cfg.Lock.TTL / 3
		}
		if cfg.Lock.RenewInterval <= 0 || cfg.Lock.RenewInterval >= cfg.Lock.TTL {
			return fmt.Errorf("%w: renew interval must be positive and less than ttl", ErrInvalidLock)
		}
		if cfg.Lock.RenewTimeout <= 0 {
			cfg.Lock.RenewTimeout = defaultLockRenewTimeout
		}
		if cfg.Lock.RenewTimeout >= cfg.Lock.TTL {
			return fmt.Errorf("%w: renew timeout must be less than ttl", ErrInvalidLock)
		}
	}
	return nil
}
