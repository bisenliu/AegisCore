package scheduler

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
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
	require.Equal(t, 1, metrics.failedCount("panic-job"))
	require.Equal(t, 1, metrics.completedCount("panic-job"))
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

	require.False(t, executed.Load(), "task executed while local gate was not available")
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

	require.False(t, executed.Load(), "task executed while global gate was full")
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
	require.NoError(t, s.validateJob(&cfg))

	s.runJob(cfg, newLocalGate())

	require.Equal(t, time.Duration(0), time.Duration(locker.lastWaitTimeout.Load()))
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
	require.NoError(t, s.validateJob(&cfg))

	s.runJob(cfg, newLocalGate())

	require.Equal(t, waitTimeout, time.Duration(locker.lastWaitTimeout.Load()))
}

func TestUnlockUsesDefaultTimeout(t *testing.T) {
	lock := &recordingLock{}
	locker := &recordingLocker{lock: lock}
	s := newTestScheduler(t, Config{Locker: locker, DefaultLockTTL: time.Minute})

	cfg := JobConfig{
		Key:  "unlock-timeout-job",
		Spec: "@every 1s",
		Lock: LockPolicy{
			Enabled: true,
			Mode:    LockModeSkipIfLocked,
		},
		Task: func(context.Context) error {
			return nil
		},
	}
	require.NoError(t, s.validateJob(&cfg))

	s.runJob(cfg, newLocalGate())

	require.True(t, lock.unlocked.Load())
	requireTimeoutNear(t, lock.unlockTimeout(), defaultLockUnlockTimeout)
}

func TestValidateJobUsesDefaultRenewTimeout(t *testing.T) {
	s := newTestScheduler(t, Config{Locker: &recordingLocker{}, DefaultLockTTL: time.Minute})
	cfg := JobConfig{
		Key:  "renew-timeout-job",
		Spec: "@every 1s",
		Lock: LockPolicy{
			Enabled:   true,
			TTL:       30 * time.Second,
			Mode:      LockModeSkipIfLocked,
			AutoRenew: true,
		},
		Task: func(context.Context) error {
			return nil
		},
	}

	require.NoError(t, s.validateJob(&cfg))

	require.Equal(t, defaultLockRenewTimeout, cfg.Lock.RenewTimeout)
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
	require.Equal(t, 1, metrics.lockRenewFailedCount("renew-failure-job"))
	require.Equal(t, 1, metrics.failedCount("renew-failure-job"))
	require.Zero(t, metrics.completedCount("renew-failure-job"))
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
			require.Eventually(t, func() bool {
				return lock.renewCount.Load() > 0
			}, time.Second, 5*time.Millisecond, "renew was not called with fallback interval")
			return nil
		},
	}

	s.runJob(cfg, newLocalGate())

	require.NotZero(t, lock.renewCount.Load(), "renew was not called with fallback interval")
	requireTimeoutNear(t, lock.renewTimeout(), defaultLockRenewTimeout)
	require.Equal(t, 1, s.metrics.(*recordingMetrics).completedCount("fallback-renew-job"))
}

func TestShutdownRejectsAddJobAfterStop(t *testing.T) {
	s := newTestScheduler(t, Config{})
	require.NoError(t, s.Shutdown(context.Background()))

	_, err := s.AddJob(JobConfig{
		Key:  "after-stop",
		Spec: "@every 1s",
		Task: func(context.Context) error {
			return nil
		},
	})
	require.ErrorIs(t, err, ErrSchedulerStopped)
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

	_, err := s.AddJob(cfg)
	require.NoError(t, err)
	_, err = s.AddJob(cfg)
	require.ErrorIs(t, err, ErrDuplicateJobKey)
}

func newTestScheduler(t *testing.T, cfg Config) *Scheduler {
	t.Helper()
	cfg.Logger = zap.NewNop()
	cfg.Metrics = &recordingMetrics{}
	s, err := New(cfg)
	require.NoError(t, err)
	return s
}

func newLocalGate() chan struct{} {
	gate := make(chan struct{}, 1)
	gate <- struct{}{}
	return gate
}

func assertSkipped(t *testing.T, metrics *recordingMetrics, jobKey string, reason string) {
	t.Helper()
	require.Equal(t, 1, metrics.skippedCount(jobKey, reason))
}

func requireTimeoutNear(t *testing.T, timeout time.Duration, want time.Duration) {
	t.Helper()
	require.Greater(t, timeout, want-500*time.Millisecond)
	require.LessOrEqual(t, timeout, want)
}

type recordingMetrics struct {
	mu              sync.Mutex
	triggered       map[string]int
	started         map[string]int
	completed       map[string]int
	failed          map[string]int
	skipped         map[string]int
	lockRenewFailed map[string]int
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

	if m.triggered == nil {
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
	mu             sync.Mutex
	renewErr       error
	renewCount     atomic.Int64
	unlocked       atomic.Bool
	unlockDeadline time.Time
	renewDeadline  time.Time
}

func (l *recordingLock) Unlock(ctx context.Context) error {
	l.mu.Lock()
	l.unlockDeadline, _ = ctx.Deadline()
	l.mu.Unlock()
	l.unlocked.Store(true)
	return nil
}

func (l *recordingLock) Renew(ctx context.Context, _ time.Duration) error {
	l.mu.Lock()
	l.renewDeadline, _ = ctx.Deadline()
	l.mu.Unlock()
	l.renewCount.Add(1)
	return l.renewErr
}

func (l *recordingLock) unlockTimeout() time.Duration {
	l.mu.Lock()
	defer l.mu.Unlock()
	return time.Until(l.unlockDeadline)
}

func (l *recordingLock) renewTimeout() time.Duration {
	l.mu.Lock()
	defer l.mu.Unlock()
	return time.Until(l.renewDeadline)
}
