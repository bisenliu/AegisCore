package scheduler

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"go.uber.org/zap"
)

func TestRunJobRecoversPanicAndReleasesLocalGate(t *testing.T) {
	s := newTestScheduler(t, Config{})
	localGate := newLocalGate()
	var runs atomic.Int64

	cfg := JobConfig{
		Key:  "panic-job",
		Spec: "@every 1s",
		Task: func(context.Context) error {
			if runs.Add(1) == 1 {
				panic("boom")
			}
			return nil
		},
	}

	s.runJob(cfg, localGate)
	s.runJob(cfg, localGate)

	metrics := s.metrics.(*recordingMetrics)
	if got := metrics.failedCount("panic-job"); got != 1 {
		t.Fatalf("failed count = %d, want 1", got)
	}
	if got := metrics.completedCount("panic-job"); got != 1 {
		t.Fatalf("completed count = %d, want 1", got)
	}
}

func TestRunJobSkipsWhenLocalOverlapExists(t *testing.T) {
	s := newTestScheduler(t, Config{})
	localGate := make(chan struct{}, 1)
	var executed atomic.Bool

	s.runJob(JobConfig{
		Key:  "overlap-job",
		Spec: "@every 1s",
		Task: func(context.Context) error {
			executed.Store(true)
			return nil
		},
	}, localGate)

	if executed.Load() {
		t.Fatal("task executed while local gate was not available")
	}
	assertSkipped(t, s.metrics.(*recordingMetrics), "overlap-job", "local_overlap")
}

func TestRunJobSkipsWhenGlobalConcurrencyLimitReached(t *testing.T) {
	s := newTestScheduler(t, Config{MaxConcurrentJobs: 1})
	s.globalGate <- struct{}{}
	localGate := newLocalGate()
	var executed atomic.Bool

	s.runJob(JobConfig{
		Key:  "global-limit-job",
		Spec: "@every 1s",
		Task: func(context.Context) error {
			executed.Store(true)
			return nil
		},
	}, localGate)

	if executed.Load() {
		t.Fatal("task executed while global gate was full")
	}
	assertSkipped(t, s.metrics.(*recordingMetrics), "global-limit-job", "global_concurrency_limit")

	select {
	case localGate <- struct{}{}:
		t.Fatal("local gate was not released after global concurrency skip")
	default:
	}
}

func TestLockModeSkipForcesZeroWaitTimeout(t *testing.T) {
	locker := &recordingLocker{lock: &recordingLock{}}
	s := newTestScheduler(t, Config{Locker: locker, DefaultLockTTL: time.Minute})

	cfg := JobConfig{
		Key:  "skip-lock-job",
		Spec: "@every 1s",
		Lock: LockPolicy{
			Enabled:     true,
			Mode:        LockModeSkipIfLocked,
			WaitTimeout: time.Minute,
		},
		Task: func(context.Context) error {
			return nil
		},
	}
	if err := s.validateJob(&cfg); err != nil {
		t.Fatalf("validateJob: %v", err)
	}

	s.runJob(cfg, newLocalGate())

	if got := locker.lastWaitTimeout.Load(); got != 0 {
		t.Fatalf("wait timeout = %s, want 0", time.Duration(got))
	}
}

func TestLockModeWaitUsesConfiguredWaitTimeout(t *testing.T) {
	locker := &recordingLocker{lock: &recordingLock{}}
	s := newTestScheduler(t, Config{Locker: locker, DefaultLockTTL: time.Minute})
	waitTimeout := 250 * time.Millisecond

	cfg := JobConfig{
		Key:  "wait-lock-job",
		Spec: "@every 1s",
		Lock: LockPolicy{
			Enabled:     true,
			Mode:        LockModeWait,
			WaitTimeout: waitTimeout,
		},
		Task: func(context.Context) error {
			return nil
		},
	}
	if err := s.validateJob(&cfg); err != nil {
		t.Fatalf("validateJob: %v", err)
	}

	s.runJob(cfg, newLocalGate())

	if got := time.Duration(locker.lastWaitTimeout.Load()); got != waitTimeout {
		t.Fatalf("wait timeout = %s, want %s", got, waitTimeout)
	}
}

func TestRenewFailureCancelsTaskAndMarksFailed(t *testing.T) {
	renewErr := errors.New("renew failed")
	locker := &recordingLocker{lock: &recordingLock{renewErr: renewErr}}
	s := newTestScheduler(t, Config{Locker: locker})
	taskCanceled := make(chan struct{})

	cfg := JobConfig{
		Key:     "renew-failure-job",
		Spec:    "@every 1s",
		Timeout: time.Second,
		Lock: LockPolicy{
			Enabled:       true,
			TTL:           90 * time.Millisecond,
			Mode:          LockModeSkipIfLocked,
			AutoRenew:     true,
			RenewInterval: 10 * time.Millisecond,
			RenewTimeout:  20 * time.Millisecond,
		},
		Task: func(ctx context.Context) error {
			<-ctx.Done()
			close(taskCanceled)
			return nil
		},
	}

	s.runJob(cfg, newLocalGate())

	select {
	case <-taskCanceled:
	default:
		t.Fatal("task did not observe cancellation after renew failure")
	}
	metrics := s.metrics.(*recordingMetrics)
	if got := metrics.lockRenewFailedCount("renew-failure-job"); got != 1 {
		t.Fatalf("lock renew failed count = %d, want 1", got)
	}
	if got := metrics.failedCount("renew-failure-job"); got != 1 {
		t.Fatalf("failed count = %d, want 1", got)
	}
	if got := metrics.completedCount("renew-failure-job"); got != 0 {
		t.Fatalf("completed count = %d, want 0", got)
	}
}

func TestAutoRenewUsesFallbackIntervals(t *testing.T) {
	lock := &recordingLock{}
	locker := &recordingLocker{lock: lock}
	s := newTestScheduler(t, Config{Locker: locker})

	cfg := JobConfig{
		Key:  "fallback-renew-job",
		Spec: "@every 1s",
		Lock: LockPolicy{
			Enabled:   true,
			TTL:       90 * time.Millisecond,
			Mode:      LockModeSkipIfLocked,
			AutoRenew: true,
		},
		Task: func(context.Context) error {
			time.Sleep(40 * time.Millisecond)
			return nil
		},
	}

	s.runJob(cfg, newLocalGate())

	if got := lock.renewCount.Load(); got == 0 {
		t.Fatal("renew was not called with fallback interval")
	}
	if got := s.metrics.(*recordingMetrics).completedCount("fallback-renew-job"); got != 1 {
		t.Fatalf("completed count = %d, want 1", got)
	}
}

func TestShutdownRejectsAddJobAfterStop(t *testing.T) {
	s := newTestScheduler(t, Config{})
	if err := s.Shutdown(context.Background()); err != nil {
		t.Fatalf("Shutdown: %v", err)
	}

	_, err := s.AddJob(JobConfig{
		Key:  "after-stop",
		Spec: "@every 1s",
		Task: func(context.Context) error {
			return nil
		},
	})
	if !errors.Is(err, ErrSchedulerStopped) {
		t.Fatalf("AddJob err = %v, want ErrSchedulerStopped", err)
	}
}

func TestAddJobRejectsDuplicateKey(t *testing.T) {
	s := newTestScheduler(t, Config{})
	cfg := JobConfig{
		Key:  "duplicate",
		Spec: "@every 1s",
		Task: func(context.Context) error {
			return nil
		},
	}

	if _, err := s.AddJob(cfg); err != nil {
		t.Fatalf("first AddJob: %v", err)
	}
	_, err := s.AddJob(cfg)
	if !errors.Is(err, ErrDuplicateJobKey) {
		t.Fatalf("second AddJob err = %v, want ErrDuplicateJobKey", err)
	}
}

func newTestScheduler(t *testing.T, cfg Config) *Scheduler {
	t.Helper()
	cfg.Logger = zap.NewNop()
	cfg.Metrics = &recordingMetrics{}
	s, err := New(cfg)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	return s
}

func newLocalGate() chan struct{} {
	gate := make(chan struct{}, 1)
	gate <- struct{}{}
	return gate
}

func assertSkipped(t *testing.T, metrics *recordingMetrics, jobKey string, reason string) {
	t.Helper()
	if got := metrics.skippedCount(jobKey, reason); got != 1 {
		t.Fatalf("skipped count for %s/%s = %d, want 1", jobKey, reason, got)
	}
}

type recordingMetrics struct {
	mu              sync.Mutex
	registered      map[string]int
	triggered       map[string]int
	started         map[string]int
	completed       map[string]int
	failed          map[string]int
	skipped         map[string]int
	lockRenewFailed map[string]int
}

func (m *recordingMetrics) JobRegistered(jobKey string) {
	m.ensure()
	m.add(m.registered, jobKey)
}

func (m *recordingMetrics) JobTriggered(jobKey string) {
	m.ensure()
	m.add(m.triggered, jobKey)
}

func (m *recordingMetrics) JobStarted(jobKey string) {
	m.ensure()
	m.add(m.started, jobKey)
}

func (m *recordingMetrics) JobCompleted(jobKey string, _ time.Duration) {
	m.ensure()
	m.add(m.completed, jobKey)
}

func (m *recordingMetrics) JobFailed(jobKey string, _ time.Duration) {
	m.ensure()
	m.add(m.failed, jobKey)
}

func (m *recordingMetrics) JobSkipped(jobKey string, reason string) {
	m.ensure()
	m.add(m.skipped, jobKey+"/"+reason)
}

func (m *recordingMetrics) JobLockRenewFailed(jobKey string) {
	m.ensure()
	m.add(m.lockRenewFailed, jobKey)
}

func (m *recordingMetrics) completedCount(jobKey string) int {
	m.ensure()
	return m.count(m.completed, jobKey)
}

func (m *recordingMetrics) failedCount(jobKey string) int {
	m.ensure()
	return m.count(m.failed, jobKey)
}

func (m *recordingMetrics) skippedCount(jobKey string, reason string) int {
	m.ensure()
	return m.count(m.skipped, jobKey+"/"+reason)
}

func (m *recordingMetrics) lockRenewFailedCount(jobKey string) int {
	m.ensure()
	return m.count(m.lockRenewFailed, jobKey)
}

func (m *recordingMetrics) ensure() {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.registered == nil {
		m.registered = make(map[string]int)
		m.triggered = make(map[string]int)
		m.started = make(map[string]int)
		m.completed = make(map[string]int)
		m.failed = make(map[string]int)
		m.skipped = make(map[string]int)
		m.lockRenewFailed = make(map[string]int)
	}
}

func (m *recordingMetrics) add(target map[string]int, key string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	target[key]++
}

func (m *recordingMetrics) count(target map[string]int, key string) int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return target[key]
}

type recordingLocker struct {
	lock            Lock
	acquired        bool
	err             error
	lastWaitTimeout atomic.Int64
}

func (l *recordingLocker) Acquire(_ context.Context, _ string, _ time.Duration, waitTimeout time.Duration) (Lock, bool, error) {
	l.lastWaitTimeout.Store(int64(waitTimeout))
	if l.err != nil {
		return nil, false, l.err
	}
	if !l.acquired && l.lock == nil {
		return nil, false, nil
	}
	return l.lock, true, nil
}

type recordingLock struct {
	renewErr   error
	renewCount atomic.Int64
	unlocked   atomic.Bool
}

func (l *recordingLock) Unlock(context.Context) error {
	l.unlocked.Store(true)
	return nil
}

func (l *recordingLock) Renew(context.Context, time.Duration) error {
	l.renewCount.Add(1)
	return l.renewErr
}
