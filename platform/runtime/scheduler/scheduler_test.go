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

func TestAddAcceptsSupportedCronFormatsAndRemovesByKey(t *testing.T) {
	formats := map[string]string{
		"five-fields": "0 0 * * *",
		"six-fields":  "0 0 0 * * *",
		"descriptor":  "@daily",
		"cron-tz":     "CRON_TZ=Asia/Shanghai 0 0 * * *",
	}

	for key, spec := range formats {
		t.Run(key, func(t *testing.T) {
			s := newTestScheduler(t, Config{TimeZone: "Asia/Shanghai"})
			require.NoError(t, s.Add(Job{Key: "  " + key + "  ", Spec: spec, Task: successfulTask}))
			require.True(t, s.Remove("  "+key+"  "))
			require.False(t, s.Remove(key))
			require.NoError(t, s.Stop(context.Background()))
		})
	}
}

func TestAddValidatesPublicJobModel(t *testing.T) {
	tests := map[string]Job{
		"empty key":        {Spec: "@daily", Task: successfulTask},
		"empty spec":       {Key: "job", Task: successfulTask},
		"nil task":         {Key: "job", Spec: "@daily"},
		"negative timeout": {Key: "job", Spec: "@daily", Timeout: -time.Second, Task: successfulTask},
	}
	for name, job := range tests {
		t.Run(name, func(t *testing.T) {
			s := newTestScheduler(t, Config{})
			require.ErrorIs(t, s.Add(job), ErrInvalidJob)
		})
	}

	s := newTestScheduler(t, Config{})
	job := Job{Key: "duplicate", Spec: "@daily", Task: successfulTask}
	require.NoError(t, s.Add(job))
	require.ErrorIs(t, s.Add(job), ErrDuplicateJobKey)
}

func TestNewRejectsInvalidSchedulerConfiguration(t *testing.T) {
	_, err := New(Config{TimeZone: "not/a-timezone"})
	require.Error(t, err)
	_, err = New(Config{MaxConcurrentJobs: -1})
	require.ErrorIs(t, err, ErrInvalidJob)
	_, err = New(Config{GlobalConcurrencyPolicy: "queue"})
	require.ErrorIs(t, err, ErrInvalidJob)
	_, err = New(Config{DefaultLockTTL: -time.Second})
	require.ErrorIs(t, err, ErrInvalidLock)
}

func TestNormalizeJobCopiesAndDefaultsLockAndRenewPolicies(t *testing.T) {
	locker := &recordingLocker{}
	s := newTestScheduler(t, Config{Locker: locker, DefaultLockTTL: 30 * time.Second})
	job := Job{
		Key:  "renew-defaults",
		Spec: "@daily",
		Lock: &LockPolicy{Renew: &RenewPolicy{}},
		Task: successfulTask,
	}
	normalized, err := s.normalizeJob(job)
	require.NoError(t, err)
	require.NotSame(t, job.Lock, normalized.Lock)
	require.NotSame(t, job.Lock.Renew, normalized.Lock.Renew)
	require.Zero(t, job.Lock.TTL)
	require.Zero(t, job.Lock.Renew.Interval)
	require.Zero(t, job.Lock.Renew.Timeout)
	require.Equal(t, 30*time.Second, normalized.Lock.TTL)
	require.Equal(t, 10*time.Second, normalized.Lock.Renew.Interval)
	require.Equal(t, defaultLockRenewTimeout, normalized.Lock.Renew.Timeout)

	job.Lock.TTL = time.Minute
	job.Lock.Renew.Interval = 20 * time.Second
	require.Equal(t, 30*time.Second, normalized.Lock.TTL)
	require.Equal(t, 10*time.Second, normalized.Lock.Renew.Interval)

	job.Lock.TTL = 0
	job.Lock.Renew.Interval = 0
	require.NoError(t, s.Add(job))
	require.Zero(t, job.Lock.TTL)
	require.Zero(t, job.Lock.Renew.Interval)
	require.Zero(t, job.Lock.Renew.Timeout)
	job.Lock.TTL = 45 * time.Second
	s.cron.Entry(s.jobs["renew-defaults"]).Job.Run()
	require.Equal(t, 30*time.Second, time.Duration(locker.lastTTL.Load()))

	require.NoError(t, s.Add(Job{Key: "without-lock", Spec: "@daily", Task: successfulTask}))
	withoutLocker := newTestScheduler(t, Config{})
	require.ErrorIs(t, withoutLocker.Add(Job{Key: "missing-locker", Spec: "@daily", Lock: &LockPolicy{TTL: time.Minute}, Task: successfulTask}), ErrInvalidLock)

	invalidWait := Job{Key: "wait", Spec: "@daily", Lock: &LockPolicy{TTL: time.Minute, WaitTimeout: -time.Second}, Task: successfulTask}
	_, err = s.normalizeJob(invalidWait)
	require.ErrorIs(t, err, ErrInvalidLock)
	invalidTTL := Job{Key: "ttl", Spec: "@daily", Timeout: time.Minute, Lock: &LockPolicy{TTL: time.Minute}, Task: successfulTask}
	_, err = s.normalizeJob(invalidTTL)
	require.ErrorIs(t, err, ErrInvalidLock)
}

func TestNormalizeJobRejectsNegativeLockAndRenewDurations(t *testing.T) {
	s := newTestScheduler(t, Config{Locker: &recordingLocker{}})
	tests := map[string]Job{
		"lock ttl": {
			Key: "job", Spec: "@daily", Lock: &LockPolicy{TTL: -time.Second}, Task: successfulTask,
		},
		"renew interval": {
			Key: "job", Spec: "@daily", Lock: &LockPolicy{TTL: time.Minute, Renew: &RenewPolicy{Interval: -time.Second}}, Task: successfulTask,
		},
		"renew timeout": {
			Key: "job", Spec: "@daily", Lock: &LockPolicy{TTL: time.Minute, Renew: &RenewPolicy{Interval: 10 * time.Second, Timeout: -time.Second}}, Task: successfulTask,
		},
	}
	for name, job := range tests {
		t.Run(name, func(t *testing.T) {
			_, err := s.normalizeJob(job)
			require.ErrorIs(t, err, ErrInvalidLock)
		})
	}
}

func TestStartIsIdempotentAndStopRejectsRestartAndAdd(t *testing.T) {
	s := newTestScheduler(t, Config{})
	require.NoError(t, s.Start())
	require.NoError(t, s.Start())
	require.NoError(t, s.Stop(context.Background()))
	require.ErrorIs(t, s.Start(), ErrSchedulerStopped)
	require.ErrorIs(t, s.Add(Job{Key: "after-stop", Spec: "@daily", Task: successfulTask}), ErrSchedulerStopped)
	require.NoError(t, s.Stop(context.Background()))
}

func TestLocalOverlapDefaultsToSkipAndAllowOverlapBypassesGate(t *testing.T) {
	s := newTestScheduler(t, Config{})
	blockedGate := make(chan struct{}, 1)
	var runs atomic.Int64
	job := Job{Key: "overlap", Spec: "@daily", Task: func(context.Context) error {
		runs.Add(1)
		return nil
	}}

	executeTestJob(s, job, blockedGate)
	require.Zero(t, runs.Load())
	assertSkipped(t, s.metrics.(*recordingMetrics), job.Key, "local_overlap")

	job.AllowOverlap = true
	executeTestJob(s, job, blockedGate)
	require.Equal(t, int64(1), runs.Load())
}

func TestGlobalConcurrencySkipAndWaitReleaseTokens(t *testing.T) {
	t.Run("skip", func(t *testing.T) {
		s := newTestScheduler(t, Config{MaxConcurrentJobs: 1})
		s.globalGate <- struct{}{}
		localGate := newLocalGate()
		executeTestJob(s, Job{Key: "global-skip", Spec: "@daily", Task: successfulTask}, localGate)
		assertSkipped(t, s.metrics.(*recordingMetrics), "global-skip", "global_concurrency_limit")
		require.Len(t, localGate, 1)
	})

	t.Run("wait", func(t *testing.T) {
		s := newTestScheduler(t, Config{MaxConcurrentJobs: 1, GlobalConcurrencyPolicy: GlobalConcurrencyWait})
		s.globalGate <- struct{}{}
		started := make(chan struct{})
		done := make(chan struct{})
		go func() {
			defer close(done)
			executeTestJob(s, Job{Key: "global-wait", Spec: "@daily", AllowOverlap: true, Task: func(context.Context) error {
				close(started)
				return nil
			}}, newLocalGate())
		}()
		require.Never(t, func() bool { return isClosed(started) }, 30*time.Millisecond, 5*time.Millisecond)
		<-s.globalGate
		requireClosed(t, started)
		requireClosed(t, done)
		require.Empty(t, s.globalGate)
	})
}

func TestGlobalWaitReturnsWhenRootIsCanceled(t *testing.T) {
	s := newTestScheduler(t, Config{MaxConcurrentJobs: 1, GlobalConcurrencyPolicy: GlobalConcurrencyWait})
	s.globalGate <- struct{}{}
	done := make(chan struct{})
	go func() {
		defer close(done)
		executeTestJob(s, Job{Key: "cancel-wait", Spec: "@daily", AllowOverlap: true, Task: successfulTask}, newLocalGate())
	}()
	require.Eventually(t, func() bool {
		return s.metrics.(*recordingMetrics).triggeredCount("cancel-wait") == 1
	}, time.Second, time.Millisecond)
	s.cancel()
	requireClosed(t, done)
	require.Len(t, s.globalGate, 1)
}

func TestLockStageRecordsBusyAndErrorAndUsesWaitTimeout(t *testing.T) {
	t.Run("busy", func(t *testing.T) {
		locker := &recordingLocker{}
		s := newTestScheduler(t, Config{Locker: locker})
		job := lockedJob("busy", 0, successfulTask)
		job = normalizeTestJob(t, s, job)
		executeTestJob(s, job, newLocalGate())
		assertSkipped(t, s.metrics.(*recordingMetrics), job.Key, "lock_busy")
	})

	t.Run("error", func(t *testing.T) {
		locker := &recordingLocker{err: errors.New("redis unavailable")}
		s := newTestScheduler(t, Config{Locker: locker})
		job := lockedJob("error", 0, successfulTask)
		job = normalizeTestJob(t, s, job)
		executeTestJob(s, job, newLocalGate())
		assertSkipped(t, s.metrics.(*recordingMetrics), job.Key, "lock_error")
	})

	t.Run("wait", func(t *testing.T) {
		locker := &recordingLocker{lock: &recordingLock{}, acquired: true}
		s := newTestScheduler(t, Config{Locker: locker})
		job := lockedJob("wait", 125*time.Millisecond, successfulTask)
		job = normalizeTestJob(t, s, job)
		executeTestJob(s, job, newLocalGate())
		require.Equal(t, 125*time.Millisecond, time.Duration(locker.lastWaitTimeout.Load()))
	})
}

func TestPanicRecordsFailureAndReleasesAllResources(t *testing.T) {
	lock := &recordingLock{renewStarted: make(chan struct{})}
	s := newTestScheduler(t, Config{Locker: &recordingLocker{lock: lock, acquired: true}, MaxConcurrentJobs: 1})
	job := renewingJob("panic", lock, false, func(context.Context) error {
		<-lock.renewStarted
		panic("boom")
	})
	job = normalizeTestJob(t, s, job)
	localGate := newLocalGate()
	require.Panics(t, func() { executeTestJob(s, job, localGate) })

	require.Len(t, localGate, 1)
	require.Empty(t, s.globalGate)
	require.True(t, lock.unlocked.Load())
	require.Equal(t, 1, s.metrics.(*recordingMetrics).failedCount(job.Key))

	job.Task = successfulTask
	executeTestJob(s, job, localGate)
	require.Equal(t, 1, s.metrics.(*recordingMetrics).completedCount(job.Key))
}

func TestTaskErrorReleasesLocalAndGlobalTokens(t *testing.T) {
	s := newTestScheduler(t, Config{MaxConcurrentJobs: 1})
	localGate := newLocalGate()
	executeTestJob(s, Job{Key: "error-cleanup", Spec: "@daily", Task: func(context.Context) error {
		return errors.New("task failed")
	}}, localGate)
	require.Len(t, localGate, 1)
	require.Empty(t, s.globalGate)
	require.Equal(t, 1, s.metrics.(*recordingMetrics).failedCount("error-cleanup"))
}

func TestTaskContextUsesTimeoutAndRootCancellation(t *testing.T) {
	t.Run("timeout", func(t *testing.T) {
		s := newTestScheduler(t, Config{})
		observed := make(chan struct{})
		executeTestJob(s, Job{Key: "timeout", Spec: "@daily", Timeout: 15 * time.Millisecond, Task: func(ctx context.Context) error {
			<-ctx.Done()
			close(observed)
			return ctx.Err()
		}}, newLocalGate())
		requireClosed(t, observed)
		require.Equal(t, 1, s.metrics.(*recordingMetrics).failedCount("timeout"))
	})

	t.Run("root cancellation", func(t *testing.T) {
		s := newTestScheduler(t, Config{})
		s.cancel()
		executeTestJob(s, Job{Key: "root", Spec: "@daily", Task: func(ctx context.Context) error { return ctx.Err() }}, newLocalGate())
		require.Equal(t, 1, s.metrics.(*recordingMetrics).failedCount("root"))
	})
}

func TestRenewFailureCancelsOrContinuesAndMergesErrors(t *testing.T) {
	renewErr := errors.New("renew failed")

	t.Run("cancel", func(t *testing.T) {
		lock := &recordingLock{renewErr: renewErr, renewStarted: make(chan struct{})}
		s := newTestScheduler(t, Config{Locker: &recordingLocker{lock: lock, acquired: true}})
		job := renewingJob("renew-cancel", lock, false, func(ctx context.Context) error {
			<-ctx.Done()
			return nil
		})
		job = normalizeTestJob(t, s, job)
		executeTestJob(s, job, newLocalGate())
		require.Equal(t, 1, s.metrics.(*recordingMetrics).lockRenewFailedCount(job.Key))
		require.Equal(t, 1, s.metrics.(*recordingMetrics).failedCount(job.Key))
	})

	t.Run("continue and join", func(t *testing.T) {
		taskErr := errors.New("task failed")
		lock := &recordingLock{renewErr: renewErr, renewStarted: make(chan struct{})}
		s := newTestScheduler(t, Config{Locker: &recordingLocker{lock: lock, acquired: true}})
		job := renewingJob("renew-continue", lock, true, successfulTask)
		job = normalizeTestJob(t, s, job)

		inv := &invocation{ctx: context.Background(), job: job, lock: lock}
		err := s.renewStage()(func(*invocation) error {
			<-lock.renewStarted
			require.NoError(t, inv.ctx.Err())
			return taskErr
		})(inv)
		require.ErrorIs(t, err, taskErr)
		require.ErrorIs(t, err, renewErr)
	})
}

func TestRenewSuccessCompletesBeforeUnlock(t *testing.T) {
	lock := &recordingLock{renewStarted: make(chan struct{})}
	s := newTestScheduler(t, Config{Locker: &recordingLocker{lock: lock, acquired: true}})
	job := renewingJob("renew-success", lock, false, func(context.Context) error {
		<-lock.renewStarted
		return nil
	})
	job = normalizeTestJob(t, s, job)
	executeTestJob(s, job, newLocalGate())
	require.GreaterOrEqual(t, lock.renewCount.Load(), int64(1))
	require.True(t, lock.unlocked.Load())
	require.Equal(t, 1, s.metrics.(*recordingMetrics).completedCount(job.Key))
}

func TestRenewGuardDrainsBlockedRenewAndUnlockUsesIndependentTimeout(t *testing.T) {
	lock := &recordingLock{renewStarted: make(chan struct{}), blockRenew: true}
	s := newTestScheduler(t, Config{Locker: &recordingLocker{lock: lock, acquired: true}})
	job := renewingJob("renew-drain", lock, true, func(context.Context) error {
		<-lock.renewStarted
		return nil
	})
	job = normalizeTestJob(t, s, job)

	done := make(chan struct{})
	go func() {
		defer close(done)
		executeTestJob(s, job, newLocalGate())
	}()
	requireClosed(t, done)
	require.True(t, lock.unlocked.Load())
	requireTimeoutNear(t, lock.unlockTimeout(), defaultLockUnlockTimeout)
}

func TestTaskTimeoutDoesNotReportRenewFailure(t *testing.T) {
	lock := &recordingLock{renewStarted: make(chan struct{}), blockRenew: true}
	s := newTestScheduler(t, Config{Locker: &recordingLocker{lock: lock, acquired: true}})
	job := renewingJob("renew-timeout", lock, false, func(ctx context.Context) error {
		<-lock.renewStarted
		<-ctx.Done()
		return ctx.Err()
	})
	job.Timeout = 15 * time.Millisecond
	job = normalizeTestJob(t, s, job)
	executeTestJob(s, job, newLocalGate())
	require.Zero(t, s.metrics.(*recordingMetrics).lockRenewFailedCount(job.Key))
	require.Equal(t, 1, s.metrics.(*recordingMetrics).failedCount(job.Key))
}

func TestDurationExcludesGlobalGateWait(t *testing.T) {
	s := newTestScheduler(t, Config{MaxConcurrentJobs: 1, GlobalConcurrencyPolicy: GlobalConcurrencyWait})
	s.globalGate <- struct{}{}
	done := make(chan struct{})
	go func() {
		defer close(done)
		executeTestJob(s, Job{Key: "duration", Spec: "@daily", AllowOverlap: true, Task: successfulTask}, newLocalGate())
	}()
	require.Eventually(t, func() bool {
		return s.metrics.(*recordingMetrics).triggeredCount("duration") == 1
	}, time.Second, time.Millisecond)

	timer := time.NewTimer(60 * time.Millisecond)
	defer timer.Stop()
	<-timer.C
	<-s.globalGate
	requireClosed(t, done)
	duration := s.metrics.(*recordingMetrics).completedDuration("duration")
	require.Less(t, duration, 30*time.Millisecond)
}

func TestDurationExcludesLockWait(t *testing.T) {
	locker := &blockingLocker{started: make(chan struct{}), release: make(chan struct{}), lock: &recordingLock{}}
	s := newTestScheduler(t, Config{Locker: locker})
	job := lockedJob("lock-duration", time.Second, successfulTask)
	job = normalizeTestJob(t, s, job)
	done := make(chan struct{})
	go func() {
		defer close(done)
		executeTestJob(s, job, newLocalGate())
	}()
	requireClosed(t, locker.started)
	timer := time.NewTimer(60 * time.Millisecond)
	defer timer.Stop()
	<-timer.C
	close(locker.release)
	requireClosed(t, done)
	require.Less(t, s.metrics.(*recordingMetrics).completedDuration(job.Key), 30*time.Millisecond)
}

func TestStopTimeoutKeepsSharedDrainAndCancelsActiveTask(t *testing.T) {
	s := newTestScheduler(t, Config{})
	started := make(chan struct{})
	canceled := make(chan struct{})
	release := make(chan struct{})
	var runs atomic.Int64
	require.NoError(t, s.Add(Job{Key: "drain", Spec: "* * * * * *", Task: func(ctx context.Context) error {
		runs.Add(1)
		close(started)
		<-ctx.Done()
		close(canceled)
		<-release
		return nil
	}}))
	require.NoError(t, s.Start())
	requireClosed(t, started)

	firstCtx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()
	err := s.Stop(firstCtx)
	require.ErrorIs(t, err, context.DeadlineExceeded)
	requireClosed(t, canceled)
	require.ErrorIs(t, s.Start(), ErrSchedulerStopped)
	require.ErrorIs(t, s.Add(Job{Key: "late", Spec: "@daily", Task: successfulTask}), ErrSchedulerStopped)

	close(release)
	secondCtx, secondCancel := context.WithTimeout(context.Background(), time.Second)
	defer secondCancel()
	require.NoError(t, s.Stop(secondCtx))
	completedRuns := runs.Load()
	noTriggerTimer := time.NewTimer(1100 * time.Millisecond)
	defer noTriggerTimer.Stop()
	<-noTriggerTimer.C
	require.Equal(t, completedRuns, runs.Load())
}

func successfulTask(context.Context) error { return nil }

func normalizeTestJob(t *testing.T, s *Scheduler, job Job) Job {
	t.Helper()
	normalized, err := s.normalizeJob(job)
	require.NoError(t, err)
	return normalized
}

func executeTestJob(s *Scheduler, job Job, localGate chan struct{}) {
	_ = s.buildPipeline(localGate)(&invocation{ctx: s.root, job: job})
}

func lockedJob(key string, wait time.Duration, task func(context.Context) error) Job {
	return Job{Key: key, Spec: "@daily", Lock: &LockPolicy{TTL: time.Minute, WaitTimeout: wait}, Task: task}
}

func renewingJob(key string, _ Lock, continueOnFailure bool, task func(context.Context) error) Job {
	return Job{
		Key:  key,
		Spec: "@daily",
		Lock: &LockPolicy{
			TTL: time.Second,
			Renew: &RenewPolicy{
				Interval:          5 * time.Millisecond,
				Timeout:           50 * time.Millisecond,
				ContinueOnFailure: continueOnFailure,
			},
		},
		Task: task,
	}
}

func newTestScheduler(t *testing.T, cfg Config) *Scheduler {
	t.Helper()
	cfg.Logger = zap.NewNop()
	cfg.Metrics = newRecordingMetrics()
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

func requireClosed(t *testing.T, channel <-chan struct{}) {
	t.Helper()
	select {
	case <-channel:
	case <-time.After(2 * time.Second):
		t.Fatal("deadline exceeded while waiting for channel")
	}
}

func isClosed(channel <-chan struct{}) bool {
	select {
	case <-channel:
		return true
	default:
		return false
	}
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
	durations       map[string][]time.Duration
}

func newRecordingMetrics() *recordingMetrics {
	return &recordingMetrics{
		triggered:       make(map[string]int),
		started:         make(map[string]int),
		completed:       make(map[string]int),
		failed:          make(map[string]int),
		skipped:         make(map[string]int),
		lockRenewFailed: make(map[string]int),
		durations:       make(map[string][]time.Duration),
	}
}

func (m *recordingMetrics) JobTriggered(key string) { m.add(m.triggered, key) }
func (m *recordingMetrics) JobStarted(key string)   { m.add(m.started, key) }
func (m *recordingMetrics) JobCompleted(key string, duration time.Duration) {
	m.addDuration(m.completed, key, duration)
}
func (m *recordingMetrics) JobFailed(key string, duration time.Duration) {
	m.addDuration(m.failed, key, duration)
}
func (m *recordingMetrics) JobSkipped(key, reason string) { m.add(m.skipped, key+"/"+reason) }
func (m *recordingMetrics) JobLockRenewFailed(key string) { m.add(m.lockRenewFailed, key) }

func (m *recordingMetrics) add(target map[string]int, key string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	target[key]++
}

func (m *recordingMetrics) addDuration(target map[string]int, key string, duration time.Duration) {
	m.mu.Lock()
	defer m.mu.Unlock()
	target[key]++
	m.durations[key] = append(m.durations[key], duration)
}

func (m *recordingMetrics) count(target map[string]int, key string) int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return target[key]
}

func (m *recordingMetrics) triggeredCount(key string) int { return m.count(m.triggered, key) }
func (m *recordingMetrics) completedCount(key string) int { return m.count(m.completed, key) }
func (m *recordingMetrics) failedCount(key string) int    { return m.count(m.failed, key) }
func (m *recordingMetrics) skippedCount(key, reason string) int {
	return m.count(m.skipped, key+"/"+reason)
}
func (m *recordingMetrics) lockRenewFailedCount(key string) int {
	return m.count(m.lockRenewFailed, key)
}
func (m *recordingMetrics) completedDuration(key string) time.Duration {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.durations[key][0]
}

type recordingLocker struct {
	lock            Lock
	acquired        bool
	err             error
	lastTTL         atomic.Int64
	lastWaitTimeout atomic.Int64
}

func (l *recordingLocker) Acquire(_ context.Context, _ string, ttl time.Duration, wait time.Duration) (Lock, bool, error) {
	l.lastTTL.Store(int64(ttl))
	l.lastWaitTimeout.Store(int64(wait))
	return l.lock, l.acquired, l.err
}

type blockingLocker struct {
	started chan struct{}
	release chan struct{}
	lock    Lock
	once    sync.Once
}

func (l *blockingLocker) Acquire(ctx context.Context, _ string, _ time.Duration, _ time.Duration) (Lock, bool, error) {
	l.once.Do(func() { close(l.started) })
	select {
	case <-l.release:
		return l.lock, true, nil
	case <-ctx.Done():
		return nil, false, ctx.Err()
	}
}

type recordingLock struct {
	mu             sync.Mutex
	renewErr       error
	renewStarted   chan struct{}
	renewOnce      sync.Once
	blockRenew     bool
	renewCount     atomic.Int64
	unlocked       atomic.Bool
	unlockDeadline time.Time
}

func (l *recordingLock) Unlock(ctx context.Context) error {
	l.mu.Lock()
	l.unlockDeadline, _ = ctx.Deadline()
	l.mu.Unlock()
	l.unlocked.Store(true)
	return nil
}

func (l *recordingLock) Renew(ctx context.Context, _ time.Duration) error {
	l.renewCount.Add(1)
	if l.renewStarted != nil {
		l.renewOnce.Do(func() { close(l.renewStarted) })
	}
	if l.blockRenew {
		<-ctx.Done()
		return ctx.Err()
	}
	return l.renewErr
}

func (l *recordingLock) unlockTimeout() time.Duration {
	l.mu.Lock()
	defer l.mu.Unlock()
	return time.Until(l.unlockDeadline)
}
