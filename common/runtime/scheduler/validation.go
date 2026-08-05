package scheduler

import (
	"fmt"
	"strings"
)

// validateJob 校验任务配置，并在注册前把默认 TTL、续租间隔和操作超时固化到 Job 副本。
func (s *Scheduler) validateJob(cfg *Job) error {
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
	if cfg.Timeout < 0 {
		return fmt.Errorf("%w: timeout must not be negative", ErrInvalidJob)
	}
	if cfg.Lock == nil {
		return nil
	}
	if s.locker == nil {
		return fmt.Errorf("%w: locker is required", ErrInvalidLock)
	}
	if cfg.Lock.WaitTimeout < 0 {
		return fmt.Errorf("%w: wait timeout must not be negative", ErrInvalidLock)
	}
	if cfg.Lock.TTL <= 0 {
		cfg.Lock.TTL = s.defaultLockTTL
	}
	if cfg.Lock.TTL <= 0 {
		return fmt.Errorf("%w: ttl is required", ErrInvalidLock)
	}
	if cfg.Timeout > 0 && cfg.Lock.Renew == nil && cfg.Lock.TTL <= cfg.Timeout {
		return fmt.Errorf("%w: ttl must be greater than job timeout or renew policy must be configured", ErrInvalidLock)
	}
	if cfg.Lock.Renew != nil {
		if cfg.Lock.Renew.Interval <= 0 {
			cfg.Lock.Renew.Interval = cfg.Lock.TTL / 3
		}
		if cfg.Lock.Renew.Interval <= 0 || cfg.Lock.Renew.Interval >= cfg.Lock.TTL {
			return fmt.Errorf("%w: renew interval must be positive and less than ttl", ErrInvalidLock)
		}
		if cfg.Lock.Renew.Timeout <= 0 {
			cfg.Lock.Renew.Timeout = defaultLockRenewTimeout
		}
		if cfg.Lock.Renew.Timeout >= cfg.Lock.TTL {
			return fmt.Errorf("%w: renew timeout must be less than ttl", ErrInvalidLock)
		}
	}
	return nil
}
