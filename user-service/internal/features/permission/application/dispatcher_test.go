package application

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"go.uber.org/zap/zaptest/observer"
)

func TestDispatcherSettingsValidateAndBackoff(t *testing.T) {
	valid := testDispatcherSettings()
	require.NoError(t, valid.Validate())
	require.Equal(t, time.Second, valid.RetryBackoff(1))
	require.Equal(t, 2*time.Second, valid.RetryBackoff(2))
	require.Equal(t, 8*time.Second, valid.RetryBackoff(4))
	require.Equal(t, 8*time.Second, valid.RetryBackoff(1000))

	tests := []DispatcherSettings{
		{BatchSize: 1, ClaimTimeout: time.Second, BackoffInitial: time.Second, BackoffMax: time.Second},
		{PollInterval: time.Second, ClaimTimeout: time.Second, BackoffInitial: time.Second, BackoffMax: time.Second},
		{PollInterval: time.Second, BatchSize: 1, BackoffInitial: time.Second, BackoffMax: time.Second},
		{PollInterval: time.Second, BatchSize: 1, ClaimTimeout: time.Second, BackoffMax: time.Second},
		{PollInterval: time.Second, BatchSize: 1, ClaimTimeout: time.Second, BackoffInitial: 2 * time.Second, BackoffMax: time.Second},
	}
	for _, settings := range tests {
		require.Error(t, settings.Validate())
	}
}

func TestDispatcherDispatchOncePublishesAndAcknowledgesInClaimOrder(t *testing.T) {
	now := time.Date(2026, 7, 31, 4, 0, 0, 0, time.UTC)
	clock := newFakeClock(now)
	first := testOutboxClaim(1, 0)
	second := testOutboxClaim(2, 1)
	store := &fakeOutboxStore{claims: []OutboxClaim{first, second}}
	publisher := &fakeRevisionPublisher{}
	dispatcher := newTestDispatcher(t, store, publisher, clock)

	result, err := dispatcher.DispatchOnce(context.Background())
	require.NoError(t, err)
	require.Equal(t, DispatcherDispatchResult{
		Claimed:         2,
		Delivered:       2,
		Acknowledged:    2,
		StatusRefreshed: true,
		Status:          DispatcherStatus{LastSuccessfulDispatch: &now},
	}, result)
	require.Equal(t, []int64{1, 2}, publisher.revisions)
	require.Equal(t, []uuid.UUID{first.Event.EventID, second.Event.EventID}, store.acked)
	require.Empty(t, store.failed)
	status, err := dispatcher.Status(context.Background())
	require.NoError(t, err)
	require.Equal(t, &now, status.LastSuccessfulDispatch)
}

func TestDispatcherDispatchOnceRecordsFailureBackoffAndContinues(t *testing.T) {
	now := time.Date(2026, 7, 31, 4, 0, 0, 0, time.UTC)
	clock := newFakeClock(now)
	first := testOutboxClaim(1, 2)
	second := testOutboxClaim(2, 0)
	store := &fakeOutboxStore{claims: []OutboxClaim{first, second}}
	publisher := &fakeRevisionPublisher{failRevision: 1, err: errors.New("redis unavailable")}
	dispatcher := newTestDispatcher(t, store, publisher, clock)

	result, err := dispatcher.DispatchOnce(context.Background())
	require.ErrorContains(t, err, "redis unavailable")
	requireDispatchError(t, err, DispatcherDispatchStagePublish, DispatcherErrorPublish)
	require.Equal(t, 2, result.Claimed)
	require.Equal(t, 1, result.Delivered)
	require.Equal(t, 1, result.Acknowledged)
	require.Equal(t, 1, result.Retried)
	require.Equal(t, 1, result.Failed)
	require.True(t, result.StatusRefreshed)
	require.Equal(t, DispatcherErrorPublish, result.Status.LastErrorCategory)
	require.Equal(t, []int64{1, 2}, publisher.revisions)
	require.Equal(t, []uuid.UUID{second.Event.EventID}, store.acked)
	require.Len(t, store.failed, 1)
	require.Equal(t, first.Event.EventID, store.failed[0].eventID)
	require.Equal(t, now.Add(4*time.Second), store.failed[0].nextAttemptAt)
	require.Equal(t, DispatcherErrorPublish, store.failed[0].summary)
	status, statusErr := dispatcher.Status(context.Background())
	require.NoError(t, statusErr)
	require.Equal(t, DispatcherErrorPublish, status.LastErrorCategory)
}

func TestDispatcherSuccessfulDeliveryClearsRecoveredError(t *testing.T) {
	now := time.Date(2026, 7, 31, 4, 0, 0, 0, time.UTC)
	claim := testOutboxClaim(1, 0)
	store := &fakeOutboxStore{claims: []OutboxClaim{claim}}
	publisher := &recoveringRevisionPublisher{err: errors.New("redis unavailable")}
	dispatcher := newTestDispatcher(t, store, publisher, newFakeClock(now))

	_, err := dispatcher.DispatchOnce(context.Background())
	require.Error(t, err)
	status, err := dispatcher.Status(context.Background())
	require.NoError(t, err)
	require.Equal(t, DispatcherErrorPublish, status.LastErrorCategory)

	publisher.err = nil
	result, err := dispatcher.DispatchOnce(context.Background())
	require.NoError(t, err)
	require.Equal(t, 1, result.Delivered)
	require.Equal(t, 1, result.Acknowledged)
	require.True(t, result.StatusRefreshed)
	status, err = dispatcher.Status(context.Background())
	require.NoError(t, err)
	require.Empty(t, status.LastErrorCategory)
}

func TestDispatcherFaultInjectionRetryReplaysAddRemoveReplaceWithoutDroppingNotifications(t *testing.T) {
	now := time.Date(2026, 7, 31, 4, 0, 0, 0, time.UTC)
	add := testOutboxClaimWithReason(21, 0, "user_role_added")
	remove := testOutboxClaimWithReason(22, 0, "user_role_removed")
	replace := testOutboxClaimWithReason(23, 1, "role_permissions_replaced")
	store := &fakeOutboxStore{claims: []OutboxClaim{add, remove, replace}}
	publisher := &sequenceRevisionPublisher{failures: map[int64]error{23: errors.New("redis unavailable")}}
	dispatcher := newTestDispatcher(t, store, publisher, newFakeClock(now))

	result, err := dispatcher.DispatchOnce(context.Background())
	require.ErrorContains(t, err, "redis unavailable")
	requireDispatchError(t, err, DispatcherDispatchStagePublish, DispatcherErrorPublish)
	require.Equal(t, 3, result.Claimed)
	require.Equal(t, 2, result.Delivered)
	require.Equal(t, 2, result.Acknowledged)
	require.Equal(t, 1, result.Retried)
	require.Equal(t, 1, result.Failed)
	require.True(t, result.StatusRefreshed)
	require.Equal(t, []int64{21, 22, 23}, publisher.revisions())
	require.Equal(t, []uuid.UUID{add.Event.EventID, remove.Event.EventID}, store.acked)
	require.Len(t, store.failed, 1)
	require.Equal(t, replace.Event.EventID, store.failed[0].eventID)
	require.Equal(t, DispatcherErrorPublish, store.failed[0].summary)

	store.claims = []OutboxClaim{{Event: replace.Event, ClaimToken: uuid.New(), AttemptCount: 2}}
	publisher.failures = nil
	result, err = dispatcher.DispatchOnce(context.Background())
	require.NoError(t, err)
	require.Equal(t, 1, result.Delivered)
	require.Equal(t, 1, result.Acknowledged)
	require.Equal(t, []int64{21, 22, 23, 23}, publisher.revisions())
	require.Equal(t, []uuid.UUID{add.Event.EventID, remove.Event.EventID, replace.Event.EventID}, store.acked)
	status, statusErr := dispatcher.Status(context.Background())
	require.NoError(t, statusErr)
	require.Empty(t, status.LastErrorCategory)
}

func TestDispatcherRecordsPublishFailureAndRetryOperations(t *testing.T) {
	now := time.Date(2026, 7, 31, 4, 0, 0, 0, time.UTC)
	claim := testOutboxClaim(1, 2)
	metrics := &recordingDispatcherMetrics{}
	dispatcher, err := NewDispatcher(
		&fakeOutboxStore{claims: []OutboxClaim{claim}},
		&fakeRevisionPublisher{failRevision: 1, err: errors.New("redis unavailable")},
		testDispatcherSettings(),
		newFakeClock(now),
		zap.NewNop(),
		metrics,
	)
	require.NoError(t, err)

	_, err = dispatcher.DispatchOnce(context.Background())
	require.Error(t, err)
	require.Equal(t, []dispatcherMetricEvent{
		{operation: MetricsOperationDispatcherClaim, result: MetricsResultSuccess, reason: MetricsReasonNone, kind: MetricsKindNone},
		{operation: MetricsOperationDispatcherPublish, result: MetricsResultFailure, reason: MetricsReasonPublishFailed, kind: MetricsKindPolicyChanged},
		{operation: MetricsOperationDispatcherFailure, result: MetricsResultSuccess, reason: MetricsReasonNone, kind: MetricsKindPolicyChanged},
		{operation: MetricsOperationDispatcherRetry, result: MetricsResultSuccess, reason: MetricsReasonPublishFailed, kind: MetricsKindPolicyChanged},
	}, metrics.operations)
}

func TestDispatcherDispatchOnceReportsLostClaim(t *testing.T) {
	store := &fakeOutboxStore{claims: []OutboxClaim{testOutboxClaim(1, 0)}, ackUpdated: boolPointer(false)}
	dispatcher := newTestDispatcher(t, store, &fakeRevisionPublisher{}, newFakeClock(time.Now()))

	result, err := dispatcher.DispatchOnce(context.Background())
	require.ErrorIs(t, err, ErrOutboxClaimLost)
	requireDispatchError(t, err, DispatcherDispatchStageAck, DispatcherErrorClaimLost)
	require.Equal(t, 1, result.Claimed)
	require.Equal(t, 1, result.Delivered)
	require.Equal(t, 0, result.Acknowledged)
	require.Equal(t, 1, result.Failed)
	require.True(t, result.StatusRefreshed)
}

func TestDispatcherDispatchOnceReportsAckFailure(t *testing.T) {
	claim := testOutboxClaim(1, 0)
	store := &fakeOutboxStore{claims: []OutboxClaim{claim}, ackErr: errors.New("postgres unavailable")}
	dispatcher := newTestDispatcher(t, store, &fakeRevisionPublisher{}, newFakeClock(time.Now()))

	result, err := dispatcher.DispatchOnce(context.Background())
	require.ErrorContains(t, err, "postgres unavailable")
	requireDispatchError(t, err, DispatcherDispatchStageAck, DispatcherErrorAck)
	require.Equal(t, 1, result.Claimed)
	require.Equal(t, 1, result.Delivered)
	require.Equal(t, 0, result.Acknowledged)
	require.Equal(t, 1, result.Failed)
	require.True(t, result.StatusRefreshed)
}

func TestDispatcherDispatchOnceReportsFailureRecordFailure(t *testing.T) {
	claim := testOutboxClaim(1, 0)
	store := &fakeOutboxStore{claims: []OutboxClaim{claim}, failErr: errors.New("postgres unavailable")}
	publisher := &fakeRevisionPublisher{failRevision: 1, err: errors.New("redis unavailable")}
	dispatcher := newTestDispatcher(t, store, publisher, newFakeClock(time.Now()))

	result, err := dispatcher.DispatchOnce(context.Background())
	require.ErrorContains(t, err, "postgres unavailable")
	requireDispatchError(t, err, DispatcherDispatchStagePublish, DispatcherErrorPublish)
	requireDispatchError(t, err, DispatcherDispatchStageFailureRecord, DispatcherErrorFailureRecord)
	require.Equal(t, 1, result.Claimed)
	require.Equal(t, 0, result.Delivered)
	require.Equal(t, 0, result.Acknowledged)
	require.Equal(t, 0, result.Retried)
	require.Equal(t, 1, result.Failed)
	require.True(t, result.StatusRefreshed)
}

func TestDispatcherDispatchOnceReportsClaimFailure(t *testing.T) {
	store := &fakeOutboxStore{claimErr: errors.New("postgres unavailable")}
	dispatcher := newTestDispatcher(t, store, &fakeRevisionPublisher{}, newFakeClock(time.Now()))

	result, err := dispatcher.DispatchOnce(context.Background())
	require.ErrorContains(t, err, "postgres unavailable")
	requireDispatchError(t, err, DispatcherDispatchStageClaim, DispatcherErrorClaim)
	require.Zero(t, result)
}

func TestDispatcherDispatchOnceReportsBacklogRefreshFailure(t *testing.T) {
	now := time.Date(2026, 7, 31, 4, 0, 0, 0, time.UTC)
	claim := testOutboxClaim(1, 0)
	store := &fakeOutboxStore{claims: []OutboxClaim{claim}, backlogErr: errors.New("postgres unavailable")}
	dispatcher := newTestDispatcher(t, store, &fakeRevisionPublisher{}, newFakeClock(now))

	result, err := dispatcher.DispatchOnce(context.Background())
	require.ErrorContains(t, err, "postgres unavailable")
	requireDispatchError(t, err, DispatcherDispatchStageStatus, DispatcherErrorBacklog)
	require.Equal(t, 1, result.Claimed)
	require.Equal(t, 1, result.Delivered)
	require.Equal(t, 1, result.Acknowledged)
	require.False(t, result.StatusRefreshed)
}

func TestDispatcherCancellationLeavesClaimForLeaseRecovery(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	claim := testOutboxClaim(1, 0)
	store := &fakeOutboxStore{claims: []OutboxClaim{claim}}
	publisher := revisionPublisherFunc(func(context.Context, OutboxEvent) error {
		cancel()
		return context.Canceled
	})
	dispatcher := newTestDispatcher(t, store, publisher, newFakeClock(time.Now()))

	result, err := dispatcher.DispatchOnce(ctx)
	require.ErrorIs(t, err, context.Canceled)
	requireDispatchError(t, err, DispatcherDispatchStageContext, DispatcherErrorContext)
	require.Equal(t, 1, result.Claimed)
	require.Equal(t, 0, result.Delivered)
	require.Equal(t, 0, result.Acknowledged)
	require.Equal(t, 0, result.Failed)
	require.False(t, result.StatusRefreshed)
	require.Empty(t, store.acked)
	require.Empty(t, store.failed)
}

func TestDispatcherStartStopAreIdempotent(t *testing.T) {
	clock := newFakeClock(time.Now())
	store := &fakeOutboxStore{}
	dispatcher := newTestDispatcher(t, store, &fakeRevisionPublisher{}, clock)
	startCtx := context.WithValue(context.Background(), dispatcherTestContextKey{}, "lifecycle")

	require.NoError(t, dispatcher.Start(startCtx))
	require.NoError(t, dispatcher.Start(context.WithValue(context.Background(), dispatcherTestContextKey{}, "second")))
	require.Eventually(t, func() bool { return store.claimCount() == 1 }, time.Second, time.Millisecond)
	require.Equal(t, []any{"lifecycle"}, store.recordedClaimContextValues())
	status, err := dispatcher.Status(context.Background())
	require.NoError(t, err)
	require.True(t, status.Running)
	require.NoError(t, dispatcher.Stop(context.Background()))
	require.NoError(t, dispatcher.Stop(context.Background()))
	status, err = dispatcher.Status(context.Background())
	require.NoError(t, err)
	require.False(t, status.Running)
	require.Equal(t, 1, clock.tickerCount())
}

func TestDispatcherUnexpectedExitLogsRecoveryContextAndUpdatesStatus(t *testing.T) {
	core, observed := observer.New(zapcore.ErrorLevel)
	metrics := &recordingDispatcherMetrics{}
	dispatcher, err := NewDispatcher(
		&panicOutboxStore{},
		&fakeRevisionPublisher{},
		testDispatcherSettings(),
		newFakeClock(time.Now()),
		zap.New(core),
		metrics,
	)
	require.NoError(t, err)

	require.NoError(t, dispatcher.Start(context.Background()))
	require.Eventually(t, func() bool {
		status, err := dispatcher.Status(context.Background())
		return err == nil && !status.Running && status.LastErrorCategory == DispatcherErrorUnexpectedExit
	}, time.Second, time.Millisecond)
	require.NoError(t, dispatcher.Stop(context.Background()))
	require.Eventually(t, func() bool {
		running := metrics.recordedRunning()
		return len(running) == 2 && running[0] && !running[1]
	}, time.Second, time.Millisecond)

	entries := observed.FilterMessage("rbac policy outbox dispatcher exited unexpectedly").All()
	require.Len(t, entries, 1)
	fields := entries[0].ContextMap()
	require.Equal(t, DispatcherErrorUnexpectedExit, fields["error_category"])
	require.Equal(t, "test panic", fields["recovered"])
	stacktrace, ok := fields["stacktrace"].(string)
	require.True(t, ok)
	require.Contains(t, stacktrace, "(*Dispatcher).run")
}

func TestDispatcherStatusUsesReadOnlyBacklog(t *testing.T) {
	now := time.Date(2026, 7, 31, 4, 0, 0, 0, time.UTC)
	oldest := now.Add(-3 * time.Minute)
	store := &fakeOutboxStore{backlog: OutboxBacklog{DueCount: 7, OldestCreatedAt: &oldest}}
	dispatcher := newTestDispatcher(t, store, &fakeRevisionPublisher{}, newFakeClock(now))

	status, err := dispatcher.Status(context.Background())
	require.NoError(t, err)
	require.Equal(t, 7, status.DueCount)
	require.Equal(t, 3*time.Minute, status.OldestUnfinishedAge)
	require.Equal(t, 0, store.claimCount())
}

func TestDispatcherRecordsOperationsBacklogAndRunningState(t *testing.T) {
	now := time.Date(2026, 7, 31, 4, 0, 0, 0, time.UTC)
	oldest := now.Add(-time.Minute)
	claim := testOutboxClaim(1, 0)
	store := &fakeOutboxStore{claims: []OutboxClaim{claim}, backlog: OutboxBacklog{DueCount: 2, OldestCreatedAt: &oldest}}
	metrics := &recordingDispatcherMetrics{}
	dispatcher, err := NewDispatcher(store, &fakeRevisionPublisher{}, testDispatcherSettings(), newFakeClock(now), zap.NewNop(), metrics)
	require.NoError(t, err)

	result, err := dispatcher.DispatchOnce(context.Background())
	require.NoError(t, err)
	require.Equal(t, 1, result.Claimed)
	require.Equal(t, 1, result.Delivered)
	require.Equal(t, 1, result.Acknowledged)
	require.True(t, result.StatusRefreshed)
	require.Equal(t, 2, result.Status.DueCount)
	require.Equal(t, time.Minute, result.Status.OldestUnfinishedAge)
	status, err := dispatcher.Status(context.Background())
	require.NoError(t, err)
	require.Equal(t, 2, status.DueCount)
	require.Equal(t, time.Minute, status.OldestUnfinishedAge)
	require.Equal(t, []dispatcherMetricEvent{
		{operation: MetricsOperationDispatcherClaim, result: MetricsResultSuccess, reason: MetricsReasonNone, kind: MetricsKindNone},
		{operation: MetricsOperationDispatcherPublish, result: MetricsResultSuccess, reason: MetricsReasonNone, kind: MetricsKindPolicyChanged},
		{operation: MetricsOperationDispatcherAck, result: MetricsResultSuccess, reason: MetricsReasonNone, kind: MetricsKindPolicyChanged},
	}, metrics.operations)
	require.Equal(t, 2, metrics.dueCount)
	require.Equal(t, time.Minute, metrics.oldestAge)

	startCtx := context.WithValue(context.Background(), dispatcherTestContextKey{}, "metrics")
	require.NoError(t, dispatcher.Start(startCtx))
	require.Equal(t, []bool{true}, metrics.running)
	require.Equal(t, []any{"metrics"}, metrics.runningContextValues)
	require.NoError(t, dispatcher.Stop(context.Background()))
	require.Equal(t, []bool{true, false}, metrics.running)
	require.Equal(t, []any{"metrics", "metrics"}, metrics.runningContextValues)
}

func testDispatcherSettings() DispatcherSettings {
	return DispatcherSettings{PollInterval: time.Hour, BatchSize: 10, ClaimTimeout: 30 * time.Second, BackoffInitial: time.Second, BackoffMax: 8 * time.Second}
}

func newTestDispatcher(t *testing.T, store OutboxStore, publisher PolicyRevisionPublisher, clock Clock) *Dispatcher {
	t.Helper()
	dispatcher, err := NewDispatcher(store, publisher, testDispatcherSettings(), clock, zap.NewNop(), NopMetrics())
	require.NoError(t, err)
	return dispatcher
}

func testOutboxClaim(revision int64, attempt int) OutboxClaim {
	return OutboxClaim{
		Event:        OutboxEvent{EventID: uuid.New(), Revision: revision, Kind: "policy_changed", Reason: "role_updated", IdempotencyKey: "key"},
		ClaimToken:   uuid.New(),
		AttemptCount: attempt,
	}
}

func testOutboxClaimWithReason(revision int64, attempt int, reason string) OutboxClaim {
	claim := testOutboxClaim(revision, attempt)
	claim.Event.Reason = reason
	claim.Event.IdempotencyKey = "rbac-policy:" + reason
	return claim
}

type fakeOutboxStore struct {
	mu                 sync.Mutex
	claims             []OutboxClaim
	claimCalls         int
	claimContextValues []any
	claimErr           error
	acked              []uuid.UUID
	ackErr             error
	failed             []fakeFailure
	failErr            error
	failUpdated        *bool
	ackUpdated         *bool
	backlog            OutboxBacklog
	backlogErr         error
}

type fakeFailure struct {
	eventID       uuid.UUID
	nextAttemptAt time.Time
	summary       string
}

func (s *fakeOutboxStore) Claim(ctx context.Context, _ time.Time, _ int, _ time.Duration) ([]OutboxClaim, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.claimCalls++
	s.claimContextValues = append(s.claimContextValues, ctx.Value(dispatcherTestContextKey{}))
	if s.claimErr != nil {
		return nil, s.claimErr
	}
	return append([]OutboxClaim(nil), s.claims...), nil
}

func (s *fakeOutboxStore) Ack(_ context.Context, eventID uuid.UUID, _ uuid.UUID, _ time.Time) (bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.acked = append(s.acked, eventID)
	if s.ackErr != nil {
		return false, s.ackErr
	}
	if s.ackUpdated != nil {
		return *s.ackUpdated, nil
	}
	return true, nil
}

func (s *fakeOutboxStore) Fail(_ context.Context, eventID uuid.UUID, _ uuid.UUID, _ time.Time, nextAttemptAt time.Time, summary string) (bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.failed = append(s.failed, fakeFailure{eventID: eventID, nextAttemptAt: nextAttemptAt, summary: summary})
	if s.failErr != nil {
		return false, s.failErr
	}
	if s.failUpdated != nil {
		return *s.failUpdated, nil
	}
	return true, nil
}

func (s *fakeOutboxStore) Backlog(context.Context, time.Time) (OutboxBacklog, error) {
	if s.backlogErr != nil {
		return OutboxBacklog{}, s.backlogErr
	}
	return s.backlog, nil
}

func (s *fakeOutboxStore) claimCount() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.claimCalls
}

func (s *fakeOutboxStore) recordedClaimContextValues() []any {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]any(nil), s.claimContextValues...)
}

type fakeRevisionPublisher struct {
	mu           sync.Mutex
	revisions    []int64
	failRevision int64
	err          error
}

type revisionPublisherFunc func(context.Context, OutboxEvent) error

type recoveringRevisionPublisher struct {
	err error
}

type sequenceRevisionPublisher struct {
	mu       sync.Mutex
	seen     []int64
	failures map[int64]error
}

func (p *recoveringRevisionPublisher) PublishPolicyRevision(context.Context, OutboxEvent) error {
	return p.err
}

func (p *sequenceRevisionPublisher) PublishPolicyRevision(_ context.Context, event OutboxEvent) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.seen = append(p.seen, event.Revision)
	return p.failures[event.Revision]
}

func (p *sequenceRevisionPublisher) revisions() []int64 {
	p.mu.Lock()
	defer p.mu.Unlock()
	return append([]int64(nil), p.seen...)
}

func (f revisionPublisherFunc) PublishPolicyRevision(ctx context.Context, event OutboxEvent) error {
	return f(ctx, event)
}

func (p *fakeRevisionPublisher) PublishPolicyRevision(_ context.Context, event OutboxEvent) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.revisions = append(p.revisions, event.Revision)
	if event.Revision == p.failRevision {
		return p.err
	}
	return nil
}

type panicOutboxStore struct {
	fakeOutboxStore
}

func (*panicOutboxStore) Claim(context.Context, time.Time, int, time.Duration) ([]OutboxClaim, error) {
	panic("test panic")
}

type fakeClock struct {
	mu      sync.Mutex
	now     time.Time
	tickers []*fakeTicker
}

func newFakeClock(now time.Time) *fakeClock { return &fakeClock{now: now} }

func (c *fakeClock) Now() time.Time {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.now
}

func (c *fakeClock) NewTicker(time.Duration) Ticker {
	c.mu.Lock()
	defer c.mu.Unlock()
	ticker := &fakeTicker{ch: make(chan time.Time)}
	c.tickers = append(c.tickers, ticker)
	return ticker
}

func (c *fakeClock) tickerCount() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return len(c.tickers)
}

type fakeTicker struct{ ch chan time.Time }

func (t *fakeTicker) C() <-chan time.Time { return t.ch }
func (t *fakeTicker) Stop()               {}

type dispatcherMetricEvent struct {
	operation string
	result    string
	reason    string
	kind      string
}

type recordingDispatcherMetrics struct {
	nopMetrics
	mu                   sync.Mutex
	operations           []dispatcherMetricEvent
	dueCount             int
	oldestAge            time.Duration
	running              []bool
	runningContextValues []any
}

func (m *recordingDispatcherMetrics) DispatcherOperationObserved(_ context.Context, operation string, result string, reason string, kind string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.operations = append(m.operations, dispatcherMetricEvent{operation: operation, result: result, reason: reason, kind: kind})
}

func (m *recordingDispatcherMetrics) DispatcherBacklogObserved(_ context.Context, dueCount int, oldestAge time.Duration) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.dueCount = dueCount
	m.oldestAge = oldestAge
}

func (m *recordingDispatcherMetrics) DispatcherRunningObserved(ctx context.Context, running bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.running = append(m.running, running)
	m.runningContextValues = append(m.runningContextValues, ctx.Value(dispatcherTestContextKey{}))
}

func (m *recordingDispatcherMetrics) recordedRunning() []bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	return append([]bool(nil), m.running...)
}

type dispatcherTestContextKey struct{}

func boolPointer(value bool) *bool { return &value }

func requireDispatchError(t *testing.T, err error, stage DispatcherDispatchStage, category string) {
	t.Helper()
	dispatchErr := findDispatchError(err, stage, category)
	require.NotNil(t, dispatchErr, "expected dispatch error stage=%s category=%s in %v", stage, category, err)
}

func findDispatchError(err error, stage DispatcherDispatchStage, category string) *DispatcherDispatchError {
	if err == nil {
		return nil
	}
	var dispatchErr *DispatcherDispatchError
	if errors.As(err, &dispatchErr) && dispatchErr.Stage == stage && dispatchErr.Category == category {
		return dispatchErr
	}
	type multiUnwrapper interface {
		Unwrap() []error
	}
	if unwrapped, ok := err.(multiUnwrapper); ok {
		for _, child := range unwrapped.Unwrap() {
			if found := findDispatchError(child, stage, category); found != nil {
				return found
			}
		}
	}
	type singleUnwrapper interface {
		Unwrap() error
	}
	if unwrapped, ok := err.(singleUnwrapper); ok {
		return findDispatchError(unwrapped.Unwrap(), stage, category)
	}
	return nil
}
