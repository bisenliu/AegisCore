package scheduler

import (
	"fmt"
	"strings"
)

// normalizeJob 复制嵌套策略，并在注册前把默认值固化到 scheduler 自有的 Job 快照。
func (s *Scheduler) normalizeJob(job Job) (Job, error) {
	cfg := job
	if job.Lock != nil {
		lockPolicy := *job.Lock
		cfg.Lock = &lockPolicy
		if job.Lock.Renew != nil {
			renewPolicy := *job.Lock.Renew
			cfg.Lock.Renew = &renewPolicy
		}
	}

	cfg.Key = strings.TrimSpace(cfg.Key)
	cfg.Spec = strings.TrimSpace(cfg.Spec)

	if cfg.Key == "" {
		return Job{}, fmt.Errorf("%w: key is required", ErrInvalidJob)
	}
	if cfg.Spec == "" {
		return Job{}, fmt.Errorf("%w: spec is required", ErrInvalidJob)
	}
	if cfg.Task == nil {
		return Job{}, fmt.Errorf("%w: task is required", ErrInvalidJob)
	}
	if cfg.Timeout < 0 {
		return Job{}, fmt.Errorf("%w: timeout must not be negative", ErrInvalidJob)
	}
	if cfg.Lock == nil {
		return cfg, nil
	}
	if s.locker == nil {
		return Job{}, fmt.Errorf("%w: locker is required", ErrInvalidLock)
	}
	if cfg.Lock.WaitTimeout < 0 {
		return Job{}, fmt.Errorf("%w: wait timeout must not be negative", ErrInvalidLock)
	}
	if cfg.Lock.TTL < 0 {
		return Job{}, fmt.Errorf("%w: ttl must not be negative", ErrInvalidLock)
	}
	if cfg.Lock.TTL == 0 {
		cfg.Lock.TTL = s.defaultLockTTL
	}
	if cfg.Lock.TTL <= 0 {
		return Job{}, fmt.Errorf("%w: ttl is required", ErrInvalidLock)
	}
	if cfg.Timeout > 0 && cfg.Lock.Renew == nil && cfg.Lock.TTL <= cfg.Timeout {
		return Job{}, fmt.Errorf("%w: ttl must be greater than job timeout or renew policy must be configured", ErrInvalidLock)
	}
	if cfg.Lock.Renew != nil {
		if cfg.Lock.Renew.Interval < 0 {
			return Job{}, fmt.Errorf("%w: renew interval must not be negative", ErrInvalidLock)
		}
		if cfg.Lock.Renew.Interval == 0 {
			cfg.Lock.Renew.Interval = cfg.Lock.TTL / 3
		}
		if cfg.Lock.Renew.Interval <= 0 || cfg.Lock.Renew.Interval >= cfg.Lock.TTL {
			return Job{}, fmt.Errorf("%w: renew interval must be positive and less than ttl", ErrInvalidLock)
		}
		if cfg.Lock.Renew.Timeout < 0 {
			return Job{}, fmt.Errorf("%w: renew timeout must not be negative", ErrInvalidLock)
		}
		if cfg.Lock.Renew.Timeout == 0 {
			cfg.Lock.Renew.Timeout = defaultLockRenewTimeout
		}
		if cfg.Lock.Renew.Timeout >= cfg.Lock.TTL {
			return Job{}, fmt.Errorf("%w: renew timeout must be less than ttl", ErrInvalidLock)
		}
	}
	return cfg, nil
}
