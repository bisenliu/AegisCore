package scheduler

import (
	"fmt"
	"strings"

	"github.com/robfig/cron/v3"
	"go.uber.org/zap"
)

// Add 注册一个定时任务。
func (s *Scheduler) Add(job Job) error {
	cfg, err := s.normalizeJob(job)
	if err != nil {
		return err
	}

	localGate := make(chan struct{}, 1)
	localGate <- struct{}{}
	pipeline := s.buildPipeline(localGate)

	s.mu.Lock()
	defer s.mu.Unlock()

	if s.state == schedulerStopping || s.state == schedulerStopped || s.cron == nil {
		return ErrSchedulerStopped
	}
	if _, exists := s.jobs[cfg.Key]; exists {
		return fmt.Errorf("%w: %s", ErrDuplicateJobKey, cfg.Key)
	}

	id, err := s.cron.AddJob(cfg.Spec, cron.FuncJob(func() {
		_ = pipeline(&invocation{ctx: s.root, job: cfg})
	}))
	if err != nil {
		return err
	}
	s.jobs[cfg.Key] = id
	s.logger.Info("scheduler job registered", zap.String("job", cfg.Key), zap.String("spec", cfg.Spec))
	return nil
}

// Remove 按固定 key 移除已注册任务；已开始的 invocation 不会被中断。
func (s *Scheduler) Remove(key string) bool {
	key = strings.TrimSpace(key)

	s.mu.Lock()
	defer s.mu.Unlock()

	if s.cron == nil {
		return false
	}
	id, exists := s.jobs[key]
	if !exists {
		return false
	}

	s.cron.Remove(id)
	delete(s.jobs, key)
	s.logger.Info("scheduler job removed", zap.String("job", key))
	return true
}
