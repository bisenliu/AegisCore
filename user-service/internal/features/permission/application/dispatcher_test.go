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

	require.NoError(t, dispatcher.DispatchOnce(context.Background()))
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

	err := dispatcher.DispatchOnce(context.Background())
	require.ErrorContains(t, err, "redis unavailable")
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

	require.Error(t, dispatcher.DispatchOnce(context.Background()))
	status, err := dispatcher.Status(context.Background())
	require.NoError(t, err)
	require.Equal(t, DispatcherErrorPublish, status.LastErrorCategory)

	publisher.err = nil
	require.NoError(t, dispatcher.DispatchOnce(context.Background()))
	status, err = dispatcher.Status(context.Background())
	require.NoError(t, err)
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

	require.Error(t, dispatcher.DispatchOnce(context.Background()))
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

	err := dispatcher.DispatchOnce(context.Background())
	require.ErrorIs(t, err, ErrOutboxClaimLost)
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

	err := dispatcher.DispatchOnce(ctx)
	require.ErrorIs(t, err, context.Canceled)
	require.Empty(t, store.acked)
	require.Empty(t, store.failed)
}

func TestDispatcherStartStopAreIdempotent(t *testing.T) {
	clock := newFakeClock(time.Now())
	store := &fakeOutboxStore{}
	dispatcher := newTestDispatcher(t, store, &fakeRevisionPublisher{}, clock)

	require.NoError(t, dispatcher.Start())
	require.NoError(t, dispatcher.Start())
	require.Eventually(t, func() bool { return store.claimCount() == 1 }, time.Second, time.Millisecond)
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

func TestDispatcherUnexpectedExitUpdatesStatus(t *testing.T) {
	dispatcher := newTestDispatcher(t, &panicOutboxStore{}, &fakeRevisionPublisher{}, newFakeClock(time.Now()))

	require.NoError(t, dispatcher.Start())
	require.Eventually(t, func() bool {
		status, err := dispatcher.Status(context.Background())
		return err == nil && !status.Running && status.LastErrorCategory == DispatcherErrorUnexpectedExit
	}, time.Second, time.Millisecond)
	require.NoError(t, dispatcher.Stop(context.Background()))
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

	require.NoError(t, dispatcher.DispatchOnce(context.Background()))
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

	require.NoError(t, dispatcher.Start())
	require.Equal(t, []bool{true}, metrics.running)
	require.NoError(t, dispatcher.Stop(context.Background()))
	require.Equal(t, []bool{true, false}, metrics.running)
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

type fakeOutboxStore struct {
	mu         sync.Mutex
	claims     []OutboxClaim
	claimCalls int
	acked      []uuid.UUID
	failed     []fakeFailure
	ackUpdated *bool
	backlog    OutboxBacklog
}

type fakeFailure struct {
	eventID       uuid.UUID
	nextAttemptAt time.Time
	summary       string
}

func (s *fakeOutboxStore) Claim(context.Context, time.Time, int, time.Duration) ([]OutboxClaim, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.claimCalls++
	return append([]OutboxClaim(nil), s.claims...), nil
}

func (s *fakeOutboxStore) Ack(_ context.Context, eventID uuid.UUID, _ uuid.UUID, _ time.Time) (bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.acked = append(s.acked, eventID)
	if s.ackUpdated != nil {
		return *s.ackUpdated, nil
	}
	return true, nil
}

func (s *fakeOutboxStore) Fail(_ context.Context, eventID uuid.UUID, _ uuid.UUID, _ time.Time, nextAttemptAt time.Time, summary string) (bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.failed = append(s.failed, fakeFailure{eventID: eventID, nextAttemptAt: nextAttemptAt, summary: summary})
	return true, nil
}

func (s *fakeOutboxStore) Backlog(context.Context, time.Time) (OutboxBacklog, error) {
	return s.backlog, nil
}

func (s *fakeOutboxStore) claimCount() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.claimCalls
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

func (p *recoveringRevisionPublisher) PublishPolicyRevision(context.Context, OutboxEvent) error {
	return p.err
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
	mu         sync.Mutex
	operations []dispatcherMetricEvent
	dueCount   int
	oldestAge  time.Duration
	running    []bool
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

func (m *recordingDispatcherMetrics) DispatcherRunningObserved(_ context.Context, running bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.running = append(m.running, running)
}

func boolPointer(value bool) *bool { return &value }
