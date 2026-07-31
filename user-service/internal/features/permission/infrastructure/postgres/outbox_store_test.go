package postgres

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"

	"entgo.io/ent/dialect"
	entsql "entgo.io/ent/dialect/sql"
	"github.com/google/uuid"
	_ "github.com/mattn/go-sqlite3"
	"github.com/stretchr/testify/require"

	runtimeid "github.com/aegiscore/common/runtime/id"
	"github.com/aegiscore/common/testing/containers"
	permissionapplication "github.com/aegiscore/user-service/internal/features/permission/application"
	"github.com/aegiscore/user-service/internal/persistence/ent"
	"github.com/aegiscore/user-service/internal/persistence/ent/enttest"
	entrbacoutbox "github.com/aegiscore/user-service/internal/persistence/ent/rbacpolicyoutboxevent"
)

func TestOutboxStoreAckRequiresCurrentProcessingClaim(t *testing.T) {
	client, store := newTestOutboxStore(t)
	ctx := context.Background()
	now := time.Date(2026, 7, 31, 5, 0, 0, 0, time.UTC)
	claimToken := uuid.New()
	event := createTestOutboxEvent(t, client, 1, now.Add(-time.Minute), func(builder *ent.RbacPolicyOutboxEventCreate) {
		builder.SetStatus(permissionapplication.OutboxStatusProcessing).SetClaimToken(claimToken).SetClaimedUntil(now.Add(time.Minute).UnixMilli()).SetLastError("old error")
	})

	updated, err := store.Ack(ctx, event.EventID, uuid.New(), now)
	require.NoError(t, err)
	require.False(t, updated)

	updated, err = store.Ack(ctx, event.EventID, claimToken, now)
	require.NoError(t, err)
	require.True(t, updated)

	stored := client.RbacPolicyOutboxEvent.Query().Where(entrbacoutbox.EventIDEQ(event.EventID)).OnlyX(ctx)
	require.Equal(t, permissionapplication.OutboxStatusDelivered, stored.Status)
	require.Equal(t, now.UnixMilli(), *stored.DeliveredAt)
	require.Nil(t, stored.ClaimToken)
	require.Nil(t, stored.ClaimedUntil)
	require.Nil(t, stored.LastError)

	updated, err = store.Ack(ctx, event.EventID, claimToken, now.Add(time.Second))
	require.NoError(t, err)
	require.False(t, updated)
}

func TestOutboxStoreFailRequiresCurrentClaimAndIncrementsAttempt(t *testing.T) {
	client, store := newTestOutboxStore(t)
	ctx := context.Background()
	now := time.Date(2026, 7, 31, 5, 0, 0, 0, time.UTC)
	claimToken := uuid.New()
	event := createTestOutboxEvent(t, client, 2, now.Add(-time.Minute), func(builder *ent.RbacPolicyOutboxEventCreate) {
		builder.SetStatus(permissionapplication.OutboxStatusProcessing).SetAttemptCount(3).SetClaimToken(claimToken).SetClaimedUntil(now.Add(time.Minute).UnixMilli())
	})

	updated, err := store.Fail(ctx, event.EventID, uuid.New(), now, now.Add(time.Minute), "ignored")
	require.NoError(t, err)
	require.False(t, updated)

	longSummary := strings.Repeat("x", 2200)
	updated, err = store.Fail(ctx, event.EventID, claimToken, now, now.Add(4*time.Second), longSummary)
	require.NoError(t, err)
	require.True(t, updated)

	stored := client.RbacPolicyOutboxEvent.Query().Where(entrbacoutbox.EventIDEQ(event.EventID)).OnlyX(ctx)
	require.Equal(t, permissionapplication.OutboxStatusFailed, stored.Status)
	require.Equal(t, 4, stored.AttemptCount)
	require.Equal(t, now.Add(4*time.Second).UnixMilli(), stored.NextAttemptAt)
	require.Equal(t, now.UnixMilli(), stored.UpdatedAt)
	require.Len(t, *stored.LastError, 2048)
	require.Nil(t, stored.ClaimToken)
	require.Nil(t, stored.ClaimedUntil)
}

func TestOutboxStoreBacklogIsReadOnlyAndIncludesExpiredLease(t *testing.T) {
	client, store := newTestOutboxStore(t)
	ctx := context.Background()
	now := time.Date(2026, 7, 31, 5, 0, 0, 0, time.UTC)
	oldest := now.Add(-10 * time.Minute)
	createTestOutboxEvent(t, client, 1, oldest, func(builder *ent.RbacPolicyOutboxEventCreate) {
		builder.SetNextAttemptAt(now.Add(-time.Second).UnixMilli())
	})
	processing := createTestOutboxEvent(t, client, 2, now.Add(-5*time.Minute), func(builder *ent.RbacPolicyOutboxEventCreate) {
		builder.SetStatus(permissionapplication.OutboxStatusProcessing).SetClaimToken(uuid.New()).SetClaimedUntil(now.Add(-time.Second).UnixMilli())
	})
	createTestOutboxEvent(t, client, 3, now.Add(-time.Minute), func(builder *ent.RbacPolicyOutboxEventCreate) {
		builder.SetStatus(permissionapplication.OutboxStatusFailed).SetNextAttemptAt(now.Add(time.Minute).UnixMilli())
	})
	createTestOutboxEvent(t, client, 4, now.Add(-time.Hour), func(builder *ent.RbacPolicyOutboxEventCreate) {
		builder.SetStatus(permissionapplication.OutboxStatusDelivered).SetDeliveredAt(now.Add(-30 * time.Minute).UnixMilli())
	})

	backlog, err := store.Backlog(ctx, now)
	require.NoError(t, err)
	require.Equal(t, 2, backlog.DueCount)
	require.True(t, oldest.Equal(*backlog.OldestCreatedAt))

	storedProcessing := client.RbacPolicyOutboxEvent.Query().Where(entrbacoutbox.EventIDEQ(processing.EventID)).OnlyX(ctx)
	require.Equal(t, permissionapplication.OutboxStatusProcessing, storedProcessing.Status)
	require.NotNil(t, storedProcessing.ClaimToken)
}

func TestOutboxDispatcherRetriesPersistedFailureAfterPublisherRecovery(t *testing.T) {
	_, client, store := newPostgresTestOutboxStore(t)
	ctx := context.Background()
	now := time.Date(2026, 7, 31, 5, 0, 0, 0, time.UTC)
	event := createTestOutboxEvent(t, client, 1, now.Add(-time.Minute), func(builder *ent.RbacPolicyOutboxEventCreate) {
		builder.SetNextAttemptAt(now.UnixMilli())
	})
	clock := &outboxTestClock{now: now}
	publisher := &recoveringOutboxPublisher{err: errors.New("redis unavailable")}
	dispatcher, err := permissionapplication.NewDispatcher(store, publisher, permissionapplication.DispatcherSettings{
		PollInterval:   time.Second,
		BatchSize:      10,
		ClaimTimeout:   time.Minute,
		BackoffInitial: time.Second,
		BackoffMax:     time.Minute,
	}, clock, nil, permissionapplication.NopMetrics())
	require.NoError(t, err)

	require.ErrorContains(t, dispatcher.DispatchOnce(ctx), "redis unavailable")
	failed := client.RbacPolicyOutboxEvent.Query().Where(entrbacoutbox.EventIDEQ(event.EventID)).OnlyX(ctx)
	require.Equal(t, permissionapplication.OutboxStatusFailed, failed.Status)
	require.Equal(t, 1, failed.AttemptCount)
	require.Equal(t, now.Add(time.Second).UnixMilli(), failed.NextAttemptAt)
	require.Nil(t, failed.ClaimToken)

	clock.now = now.Add(time.Second)
	publisher.err = nil
	require.NoError(t, dispatcher.DispatchOnce(ctx))
	delivered := client.RbacPolicyOutboxEvent.Query().Where(entrbacoutbox.EventIDEQ(event.EventID)).OnlyX(ctx)
	require.Equal(t, permissionapplication.OutboxStatusDelivered, delivered.Status)
	require.NotNil(t, delivered.DeliveredAt)
	require.Equal(t, 2, publisher.calls)
}

func TestOutboxStorePostgresConcurrentClaimsDoNotOverlap(t *testing.T) {
	_, client, store := newPostgresTestOutboxStore(t)
	ctx := context.Background()
	now := time.Date(2026, 7, 31, 5, 0, 0, 0, time.UTC)
	for revision := int64(1); revision <= 6; revision++ {
		createTestOutboxEvent(t, client, revision, now.Add(-time.Minute), nil)
	}

	start := make(chan struct{})
	results := make(chan []permissionapplication.OutboxClaim, 2)
	errs := make(chan error, 2)
	var workers sync.WaitGroup
	workers.Add(2)
	for range 2 {
		go func() {
			defer workers.Done()
			<-start
			claims, err := store.Claim(ctx, now, 3, time.Minute)
			results <- claims
			errs <- err
		}()
	}
	close(start)
	workers.Wait()
	close(results)
	close(errs)

	for err := range errs {
		require.NoError(t, err)
	}
	claimedRevisions := make(map[int64]struct{}, 6)
	claimTokens := make(map[uuid.UUID]struct{}, 2)
	claimCount := 0
	for claims := range results {
		require.Len(t, claims, 3)
		claimCount += len(claims)
		for _, claim := range claims {
			_, duplicate := claimedRevisions[claim.Event.Revision]
			require.False(t, duplicate, "revision %d was claimed twice", claim.Event.Revision)
			claimedRevisions[claim.Event.Revision] = struct{}{}
			claimTokens[claim.ClaimToken] = struct{}{}
		}
	}
	require.Equal(t, 6, claimCount)
	require.Len(t, claimTokens, 2)
}

func TestOutboxStorePostgresClaimSkipsLockedFirstRow(t *testing.T) {
	db, client, store := newPostgresTestOutboxStore(t)
	ctx := context.Background()
	now := time.Date(2026, 7, 31, 5, 0, 0, 0, time.UTC)
	createTestOutboxEvent(t, client, 1, now.Add(-time.Minute), nil)
	createTestOutboxEvent(t, client, 2, now.Add(-time.Minute), nil)

	tx, err := db.BeginTx(ctx, nil)
	require.NoError(t, err)
	t.Cleanup(func() { _ = tx.Rollback() })
	var lockedID int64
	err = tx.QueryRowContext(ctx, "SELECT id FROM rbac_policy_outbox_events WHERE revision = $1 FOR UPDATE", 1).Scan(&lockedID)
	require.NoError(t, err)
	require.Positive(t, lockedID)

	claims, err := store.Claim(ctx, now, 1, time.Minute)
	require.NoError(t, err)
	require.Len(t, claims, 1)
	require.Equal(t, int64(2), claims[0].Event.Revision)
}

func TestOutboxStorePostgresReclaimsExpiredLeaseAndRejectsStaleToken(t *testing.T) {
	_, client, store := newPostgresTestOutboxStore(t)
	ctx := context.Background()
	now := time.Date(2026, 7, 31, 5, 0, 0, 0, time.UTC)
	staleToken := uuid.New()
	event := createTestOutboxEvent(t, client, 1, now.Add(-time.Minute), func(builder *ent.RbacPolicyOutboxEventCreate) {
		builder.SetStatus(permissionapplication.OutboxStatusProcessing).SetClaimToken(staleToken).SetClaimedUntil(now.Add(-time.Second).UnixMilli())
	})

	claims, err := store.Claim(ctx, now, 1, time.Minute)
	require.NoError(t, err)
	require.Len(t, claims, 1)
	require.Equal(t, event.EventID, claims[0].Event.EventID)
	require.NotEqual(t, staleToken, claims[0].ClaimToken)

	updated, err := store.Ack(ctx, event.EventID, staleToken, now.Add(time.Second))
	require.NoError(t, err)
	require.False(t, updated)
	updated, err = store.Fail(ctx, event.EventID, staleToken, now.Add(time.Second), now.Add(time.Minute), "stale owner")
	require.NoError(t, err)
	require.False(t, updated)

	stored := client.RbacPolicyOutboxEvent.Query().Where(entrbacoutbox.EventIDEQ(event.EventID)).OnlyX(ctx)
	require.Equal(t, permissionapplication.OutboxStatusProcessing, stored.Status)
	require.Equal(t, claims[0].ClaimToken, *stored.ClaimToken)
	require.Equal(t, now.Add(time.Minute).UnixMilli(), *stored.ClaimedUntil)
}

func TestOutboxStorePostgresClaimExcludesDeliveredEvents(t *testing.T) {
	_, client, store := newPostgresTestOutboxStore(t)
	ctx := context.Background()
	now := time.Date(2026, 7, 31, 5, 0, 0, 0, time.UTC)
	createTestOutboxEvent(t, client, 1, now.Add(-time.Minute), func(builder *ent.RbacPolicyOutboxEventCreate) {
		builder.SetStatus(permissionapplication.OutboxStatusDelivered).SetDeliveredAt(now.Add(-time.Second).UnixMilli())
	})
	createTestOutboxEvent(t, client, 2, now.Add(-time.Minute), nil)

	claims, err := store.Claim(ctx, now, 10, time.Minute)
	require.NoError(t, err)
	require.Len(t, claims, 1)
	require.Equal(t, int64(2), claims[0].Event.Revision)
}

func newTestOutboxStore(t *testing.T) (*ent.Client, *OutboxStore) {
	t.Helper()
	client := enttest.Open(t, "sqlite3", fmt.Sprintf("file:permission_outbox_store_test_%s?mode=memory&cache=shared&_fk=1", runtimeid.MustNewUUIDString()))
	t.Cleanup(func() { _ = client.Close() })
	return client, NewOutboxStore(client)
}

func newPostgresTestOutboxStore(t *testing.T) (*sql.DB, *ent.Client, *OutboxStore) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	t.Cleanup(cancel)
	postgres := containers.StartPostgres(ctx, t, containers.PostgresOptions{})
	db, err := sql.Open("pgx", postgres.DSN)
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })
	_, err = db.ExecContext(ctx, "CREATE EXTENSION IF NOT EXISTS pg_trgm")
	require.NoError(t, err)
	client := ent.NewClient(ent.Driver(entsql.OpenDB(dialect.Postgres, db)))
	require.NoError(t, client.Schema.Create(ctx))
	t.Cleanup(func() { _ = client.Close() })
	return db, client, NewOutboxStore(client)
}

func createTestOutboxEvent(t *testing.T, client *ent.Client, revision int64, createdAt time.Time, configure func(*ent.RbacPolicyOutboxEventCreate)) *ent.RbacPolicyOutboxEvent {
	t.Helper()
	ctx := context.Background()
	policyRevision, err := client.RbacPolicyRevision.Create().
		SetID(revision).
		SetReason("role_updated").
		SetCreatedAt(createdAt.UnixMilli()).
		Save(ctx)
	require.NoError(t, err)
	builder := client.RbacPolicyOutboxEvent.Create().
		SetEventID(uuid.New()).
		SetRevision(policyRevision.ID).
		SetKind("policy_changed").
		SetReason("role_updated").
		SetIdempotencyKey(fmt.Sprintf("rbac-policy-revision:%d", revision)).
		SetCreatedAt(createdAt.UnixMilli()).
		SetUpdatedAt(createdAt.UnixMilli()).
		SetPolicyRevisionID(policyRevision.ID)
	if configure != nil {
		configure(builder)
	}
	event, err := builder.Save(ctx)
	require.NoError(t, err)
	return event
}

type recoveringOutboxPublisher struct {
	err   error
	calls int
}

func (p *recoveringOutboxPublisher) PublishPolicyRevision(context.Context, permissionapplication.OutboxEvent) error {
	p.calls++
	return p.err
}

type outboxTestClock struct {
	now time.Time
}

func (c *outboxTestClock) Now() time.Time { return c.now }

func (*outboxTestClock) NewTicker(time.Duration) permissionapplication.Ticker {
	return outboxTestTicker{channel: make(chan time.Time)}
}

type outboxTestTicker struct {
	channel chan time.Time
}

func (t outboxTestTicker) C() <-chan time.Time { return t.channel }
func (outboxTestTicker) Stop()                 {}
