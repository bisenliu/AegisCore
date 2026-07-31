package redis

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/google/uuid"
	rediscmd "github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	permissionapplication "github.com/aegiscore/user-service/internal/features/permission/application"
)

func TestStorePublishPolicyRevisionCachesSuppliedRevisionAndPublishes(t *testing.T) {
	redisServer := miniredis.RunT(t)
	client := rediscmd.NewClient(&rediscmd.Options{Addr: redisServer.Addr()})
	t.Cleanup(func() { _ = client.Close() })
	store := newStore(client, mustKeyCatalog("aegiscore-user-service"), "instance-a", nil)
	pubsub := client.Subscribe(context.Background(), store.keys.PolicyChannel())
	t.Cleanup(func() { _ = pubsub.Close() })
	_, err := pubsub.Receive(context.Background())
	require.NoError(t, err)
	change := permissionapplication.NewPolicyReloadChange("role_permission_added")

	const revision int64 = 42
	event := testPolicyPublicationEvent(revision, change)
	require.NoError(t, store.PublishPolicyRevision(context.Background(), event))
	storedRevision, err := store.CurrentVersion(context.Background())
	require.NoError(t, err)
	require.Equal(t, revision, storedRevision)
	message, err := pubsub.ReceiveMessage(context.Background())
	require.NoError(t, err)
	decoded, err := decodePolicyRefreshMessage(message.Payload)
	require.NoError(t, err)
	require.Equal(t, policyRefreshSchemaVersion, decoded.SchemaVersion)
	require.Equal(t, event.EventID, decoded.EventID)
	require.Equal(t, event.IdempotencyKey, decoded.IdempotencyKey)
	require.Equal(t, revision, decoded.PolicyRevision)
	require.Equal(t, "instance-a", decoded.InstanceID)
	require.Equal(t, policyRefreshKindPolicyChanged, decoded.Kind)
	require.Equal(t, "role_permission_added", decoded.Reason)
}

func TestStorePublishPolicyRevisionDoesNotLowerCachedRevision(t *testing.T) {
	redisServer := miniredis.RunT(t)
	client := rediscmd.NewClient(&rediscmd.Options{Addr: redisServer.Addr()})
	t.Cleanup(func() { _ = client.Close() })
	store := newStore(client, mustKeyCatalog("aegiscore-user-service"), "instance-a", nil)
	require.NoError(t, client.Set(context.Background(), store.keys.PolicyVersionKey(), 50, 0).Err())
	pubsub := client.Subscribe(context.Background(), store.keys.PolicyChannel())
	t.Cleanup(func() { _ = pubsub.Close() })
	_, err := pubsub.Receive(context.Background())
	require.NoError(t, err)

	require.NoError(t, store.PublishPolicyRevision(context.Background(), testPolicyPublicationEvent(41, permissionapplication.NewPolicyReloadChange("role_updated"))))
	storedRevision, err := store.CurrentVersion(context.Background())
	require.NoError(t, err)
	require.Equal(t, int64(50), storedRevision)
	message, err := pubsub.ReceiveMessage(context.Background())
	require.NoError(t, err)
	decoded, err := decodePolicyRefreshMessage(message.Payload)
	require.NoError(t, err)
	require.Equal(t, int64(41), decoded.PolicyRevision)
}

func TestStorePublishPolicyRevisionCachesLargerBigIntRevisionExactly(t *testing.T) {
	redisServer := miniredis.RunT(t)
	client := rediscmd.NewClient(&rediscmd.Options{Addr: redisServer.Addr()})
	t.Cleanup(func() { _ = client.Close() })
	store := newStore(client, mustKeyCatalog("aegiscore-user-service"), "instance-a", nil)
	const previousRevision int64 = 9223372036854775806
	const revision int64 = 9223372036854775807
	require.NoError(t, client.Set(context.Background(), store.keys.PolicyVersionKey(), previousRevision, 0).Err())

	require.NoError(t, store.PublishPolicyRevision(context.Background(), testPolicyPublicationEvent(revision, permissionapplication.NewPolicyReloadChange("role_updated"))))
	storedRevision, err := store.CurrentVersion(context.Background())
	require.NoError(t, err)
	require.Equal(t, revision, storedRevision)
}

func TestStorePublishPolicyRevisionReturnsCacheFailureWithoutPublishing(t *testing.T) {
	redisServer := miniredis.RunT(t)
	client := rediscmd.NewClient(&rediscmd.Options{Addr: redisServer.Addr()})
	t.Cleanup(func() { _ = client.Close() })
	cacheErr := errors.New("redis cache failed")
	failingClient := &failingPolicyRedisClient{policyRedisClient: client, evalErr: cacheErr}
	store := newStore(failingClient, mustKeyCatalog("aegiscore-user-service"), "instance-a", nil)

	err := store.PublishPolicyRevision(context.Background(), testPolicyPublicationEvent(43, permissionapplication.NewPolicyReloadChange("role_updated")))

	require.ErrorIs(t, err, cacheErr)
	require.Equal(t, int64(0), failingClient.publishCalls.Load())
	storedRevision, err := store.CurrentVersion(context.Background())
	require.NoError(t, err)
	require.Zero(t, storedRevision)
}

func TestStorePublishPolicyRevisionReturnsPublishFailureAndPreservesCachedRevision(t *testing.T) {
	redisServer := miniredis.RunT(t)
	client := rediscmd.NewClient(&rediscmd.Options{Addr: redisServer.Addr()})
	t.Cleanup(func() { _ = client.Close() })
	publishErr := errors.New("redis publish failed")
	failingClient := &failingPolicyRedisClient{policyRedisClient: client, publishErr: publishErr}
	store := newStore(failingClient, mustKeyCatalog("aegiscore-user-service"), "instance-a", nil)

	err := store.PublishPolicyRevision(context.Background(), testPolicyPublicationEvent(44, permissionapplication.NewPolicyReloadChange("role_updated")))

	require.ErrorIs(t, err, publishErr)
	require.Equal(t, int64(1), failingClient.publishCalls.Load())
	storedRevision, err := store.CurrentVersion(context.Background())
	require.NoError(t, err)
	require.Equal(t, int64(44), storedRevision)
}

func TestVersionTrackerDoesNotMoveBackward(t *testing.T) {
	tracker := NewVersionTracker()
	tracker.MarkApplied(9)
	tracker.MarkApplied(4)

	require.Equal(t, int64(9), tracker.Applied())
}

func TestWatcherHandlePayloadReloadsPolicyForEveryValidEvent(t *testing.T) {
	ctrl := gomock.NewController(t)
	engine := NewMockPolicyReloadEngine(ctrl)
	tracker := NewVersionTracker()
	metrics := NewMockMetrics(ctrl)
	watcher := newWatcherWithMetrics(nil, tracker, engine, nil, time.Second, metrics)
	payload, err := encodePolicyRefreshMessage(newPolicyRefreshMessage(testPolicyPublicationEvent(3, permissionapplication.NewPolicyReloadChange("role_permission_added")), "instance-b"))
	require.NoError(t, err)

	gomock.InOrder(
		metrics.EXPECT().PolicyReloadLagObserved(gomock.Any(), int64(3)),
		engine.EXPECT().Reload(gomock.Any()).Return(nil),
		engine.EXPECT().InvalidateAllUserRoles(),
		metrics.EXPECT().WatcherReloadSucceeded(gomock.Any(), permissionapplication.MetricsSourceWatcherPubSub),
		metrics.EXPECT().PolicyReloadLagObserved(gomock.Any(), int64(0)),
		metrics.EXPECT().PolicyReloadLagObserved(gomock.Any(), int64(0)),
		engine.EXPECT().Reload(gomock.Any()).Return(nil),
		engine.EXPECT().InvalidateAllUserRoles(),
		metrics.EXPECT().WatcherReloadSucceeded(gomock.Any(), permissionapplication.MetricsSourceWatcherPubSub),
		metrics.EXPECT().PolicyReloadLagObserved(gomock.Any(), int64(0)),
	)

	watcher.HandlePayload(context.Background(), payload)
	watcher.HandlePayload(context.Background(), payload)

	require.Equal(t, int64(3), tracker.Applied())
}

func TestWatcherHandlePayloadInvalidatesUserRoleWithoutReload(t *testing.T) {
	userID := uuid.MustParse("018f0000-0000-7000-8000-000000000701")
	roleID := uuid.MustParse("018f0000-0000-7000-8000-000000000702")
	ctrl := gomock.NewController(t)
	engine := NewMockPolicyReloadEngine(ctrl)
	tracker := NewVersionTracker()
	metrics := NewMockMetrics(ctrl)
	watcher := newWatcherWithMetrics(nil, tracker, engine, nil, time.Second, metrics)
	payload, err := encodePolicyRefreshMessage(newPolicyRefreshMessage(testPolicyPublicationEvent(4, permissionapplication.NewUserRoleChange("user_role_added", userID, roleID)), "instance-b"))
	require.NoError(t, err)

	gomock.InOrder(
		metrics.EXPECT().PolicyReloadLagObserved(gomock.Any(), int64(4)),
		engine.EXPECT().InvalidateUserRole(userID),
		metrics.EXPECT().PolicyReloadLagObserved(gomock.Any(), int64(0)),
	)

	watcher.HandlePayload(context.Background(), payload)

	require.Equal(t, int64(4), tracker.Applied())
}

func TestWatcherHandlePayloadExecutesOutOfOrderUserRoleEventWithoutMovingTrackerBackward(t *testing.T) {
	userID := uuid.MustParse("018f0000-0000-7000-8000-000000000703")
	roleID := uuid.MustParse("018f0000-0000-7000-8000-000000000704")
	ctrl := gomock.NewController(t)
	engine := NewMockPolicyReloadEngine(ctrl)
	tracker := NewVersionTracker()
	tracker.MarkApplied(9)
	metrics := NewMockMetrics(ctrl)
	watcher := newWatcherWithMetrics(nil, tracker, engine, nil, time.Second, metrics)
	payload, err := encodePolicyRefreshMessage(newPolicyRefreshMessage(testPolicyPublicationEvent(4, permissionapplication.NewUserRoleChange("user_role_removed", userID, roleID)), "instance-b"))
	require.NoError(t, err)

	gomock.InOrder(
		metrics.EXPECT().PolicyReloadLagObserved(gomock.Any(), int64(0)),
		engine.EXPECT().InvalidateUserRole(userID),
		metrics.EXPECT().PolicyReloadLagObserved(gomock.Any(), int64(0)),
	)

	watcher.HandlePayload(context.Background(), payload)

	require.Equal(t, int64(9), tracker.Applied())
}

func TestWatcherHandlePayloadReloadsOutOfOrderPolicyEventWithoutMovingTrackerBackward(t *testing.T) {
	ctrl := gomock.NewController(t)
	engine := NewMockPolicyReloadEngine(ctrl)
	tracker := NewVersionTracker()
	tracker.MarkApplied(9)
	metrics := NewMockMetrics(ctrl)
	watcher := newWatcherWithMetrics(nil, tracker, engine, nil, time.Second, metrics)
	payload, err := encodePolicyRefreshMessage(newPolicyRefreshMessage(testPolicyPublicationEvent(4, permissionapplication.NewPolicyReloadChange("role_updated")), "instance-b"))
	require.NoError(t, err)

	gomock.InOrder(
		metrics.EXPECT().PolicyReloadLagObserved(gomock.Any(), int64(0)),
		engine.EXPECT().Reload(gomock.Any()).Return(nil),
		engine.EXPECT().InvalidateAllUserRoles(),
		metrics.EXPECT().WatcherReloadSucceeded(gomock.Any(), permissionapplication.MetricsSourceWatcherPubSub),
		metrics.EXPECT().PolicyReloadLagObserved(gomock.Any(), int64(0)),
	)

	watcher.HandlePayload(context.Background(), payload)

	require.Equal(t, int64(9), tracker.Applied())
}

func TestDecodePolicyRefreshMessageRejectsInvalidEnvelope(t *testing.T) {
	valid, err := encodePolicyRefreshMessage(newPolicyRefreshMessage(testPolicyPublicationEvent(6, permissionapplication.NewPolicyReloadChange("role_updated")), "instance-b"))
	require.NoError(t, err)

	tests := map[string]string{
		"legacy payload":      `{"version":6,"kind":"policy"}`,
		"unknown schema":      `{"schema_version":2,"event_id":"018f0000-0000-7000-8000-000000000801","idempotency_key":"rbac:6","policy_revision":6,"kind":"policy_changed","reason":"role_updated","publisher_instance_id":"instance-b"}`,
		"missing event id":    `{"schema_version":1,"idempotency_key":"rbac:6","policy_revision":6,"kind":"policy_changed","reason":"role_updated","publisher_instance_id":"instance-b"}`,
		"invalid event id":    `{"schema_version":1,"event_id":"not-a-uuid","idempotency_key":"rbac:6","policy_revision":6,"kind":"policy_changed","reason":"role_updated","publisher_instance_id":"instance-b"}`,
		"missing idempotency": `{"schema_version":1,"event_id":"018f0000-0000-7000-8000-000000000801","policy_revision":6,"kind":"policy_changed","reason":"role_updated","publisher_instance_id":"instance-b"}`,
		"missing user id":     `{"schema_version":1,"event_id":"018f0000-0000-7000-8000-000000000801","idempotency_key":"rbac:6","policy_revision":6,"kind":"user_role_changed","reason":"user_role_added","publisher_instance_id":"instance-b"}`,
		"unknown field":       valid[:len(valid)-1] + `,"published_at":123}`,
	}
	for name, payload := range tests {
		t.Run(name, func(t *testing.T) {
			_, err := decodePolicyRefreshMessage(payload)
			require.Error(t, err)
		})
	}
}

func TestWatcherCheckVersionCompensatesMissedMessage(t *testing.T) {
	redisServer := miniredis.RunT(t)
	client := rediscmd.NewClient(&rediscmd.Options{Addr: redisServer.Addr()})
	t.Cleanup(func() { _ = client.Close() })
	store := newStore(client, mustKeyCatalog("aegiscore-user-service"), "instance-a", nil)
	require.NoError(t, client.Set(context.Background(), store.keys.PolicyVersionKey(), 8, 0).Err())
	ctrl := gomock.NewController(t)
	engine := NewMockPolicyReloadEngine(ctrl)
	tracker := NewVersionTracker()
	tracker.MarkApplied(4)
	metrics := NewMockMetrics(ctrl)
	watcher := newWatcherWithMetrics(store, tracker, engine, nil, time.Second, metrics)

	gomock.InOrder(
		metrics.EXPECT().PolicyReloadLagObserved(gomock.Any(), int64(4)),
		metrics.EXPECT().WatcherVersionMismatch(gomock.Any(), permissionapplication.MetricsSourceWatcherVersionCheck),
		metrics.EXPECT().PolicyReloadLagObserved(gomock.Any(), int64(4)),
		engine.EXPECT().Reload(gomock.Any()).Return(nil),
		engine.EXPECT().InvalidateAllUserRoles(),
		metrics.EXPECT().WatcherReloadSucceeded(gomock.Any(), permissionapplication.MetricsSourceWatcherVersionCheck),
		metrics.EXPECT().PolicyReloadLagObserved(gomock.Any(), int64(0)),
	)

	watcher.CheckVersion(context.Background())

	require.Equal(t, int64(8), tracker.Applied())
}

func TestWatcherCheckVersionRemainsGatedByAppliedVersion(t *testing.T) {
	redisServer := miniredis.RunT(t)
	client := rediscmd.NewClient(&rediscmd.Options{Addr: redisServer.Addr()})
	t.Cleanup(func() { _ = client.Close() })
	store := newStore(client, mustKeyCatalog("aegiscore-user-service"), "instance-a", nil)
	require.NoError(t, client.Set(context.Background(), store.keys.PolicyVersionKey(), 4, 0).Err())
	tracker := NewVersionTracker()
	tracker.MarkApplied(9)
	metrics := NewMockMetrics(gomock.NewController(t))
	metrics.EXPECT().PolicyReloadLagObserved(gomock.Any(), int64(0))
	watcher := newWatcherWithMetrics(store, tracker, NewMockPolicyReloadEngine(gomock.NewController(t)), nil, time.Second, metrics)

	watcher.CheckVersion(context.Background())

	require.Equal(t, int64(9), tracker.Applied())
}

func TestWatcherReloadFailurePreservesAppliedVersion(t *testing.T) {
	reloadErr := errors.New("reload failed")
	ctrl := gomock.NewController(t)
	engine := NewMockPolicyReloadEngine(ctrl)
	tracker := NewVersionTracker()
	tracker.MarkApplied(2)
	metrics := NewMockMetrics(ctrl)
	watcher := newWatcherWithMetrics(nil, tracker, engine, nil, time.Second, metrics)
	payload, err := encodePolicyRefreshMessage(newPolicyRefreshMessage(testPolicyPublicationEvent(5, permissionapplication.NewPolicyReloadChange("permission_updated")), "instance-b"))
	require.NoError(t, err)

	gomock.InOrder(
		metrics.EXPECT().PolicyReloadLagObserved(gomock.Any(), int64(3)),
		engine.EXPECT().Reload(gomock.Any()).Return(reloadErr),
		metrics.EXPECT().WatcherReloadFailed(gomock.Any(), permissionapplication.MetricsSourceWatcherPubSub, permissionapplication.MetricsReasonReloadFailed),
	)

	watcher.HandlePayload(context.Background(), payload)

	require.Equal(t, int64(2), tracker.Applied())
}

func TestWatcherCheckVersionFailureDoesNotClearLag(t *testing.T) {
	ctrl := gomock.NewController(t)
	engine := NewMockPolicyReloadEngine(ctrl)
	tracker := NewVersionTracker()
	metrics := NewMockMetrics(ctrl)
	watcher := newWatcherWithMetrics(&failingVersionStore{}, tracker, engine, nil, time.Second, metrics)

	metrics.EXPECT().WatcherCheckFailed(gomock.Any(), permissionapplication.MetricsReasonStoreUnavailable)

	watcher.CheckVersion(context.Background())
}

func TestWatcherRunningStatus(t *testing.T) {
	redisServer := miniredis.RunT(t)
	client := rediscmd.NewClient(&rediscmd.Options{Addr: redisServer.Addr()})
	t.Cleanup(func() { _ = client.Close() })
	store := newStore(client, mustKeyCatalog("aegiscore-user-service"), "instance-a", nil)
	engine := NewMockPolicyReloadEngine(gomock.NewController(t))
	watcher := newWatcherWithMetrics(store, NewVersionTracker(), engine, nil, time.Hour, nil)

	require.False(t, watcher.Running())
	watcher.Start()
	watcher.Start()
	require.True(t, watcher.Running())
	require.NoError(t, watcher.Stop(context.Background()))
	require.NoError(t, watcher.Stop(context.Background()))
	require.False(t, watcher.Running())
	require.NoError(t, watcher.LastError())
}

func TestWatcherRecordsUnexpectedChannelClose(t *testing.T) {
	engine := NewMockPolicyReloadEngine(gomock.NewController(t))
	watcher := newWatcherWithMetrics(&closedChannelStore{}, NewVersionTracker(), engine, nil, time.Hour, nil)

	watcher.Start()
	waitForWatcherStopped(t, watcher)

	require.False(t, watcher.Running())
	require.Error(t, watcher.LastError())
}

func TestNewWatcherDoesNotStartBackgroundLoop(t *testing.T) {
	engine := NewMockPolicyReloadEngine(gomock.NewController(t))
	watcher := NewWatcher(WatcherParams{
		Tracker: NewVersionTracker(),
		Engine:  engine,
	})

	require.False(t, watcher.Running())
}

func TestWatcherStopHonorsDeadlineAndCanBeRepeated(t *testing.T) {
	release := make(chan struct{})
	closed := &atomic.Int64{}
	subscriber := blockingPolicySubscriber{release: release, closed: closed}
	store := &countingSubscriptionStore{subscriber: subscriber}
	engine := NewMockPolicyReloadEngine(gomock.NewController(t))
	watcher := newWatcherWithMetrics(store, NewVersionTracker(), engine, nil, time.Hour, nil)

	watcher.Start()
	require.True(t, watcher.Running())
	require.Eventually(t, func() bool { return store.subscriptions.Load() == 1 }, time.Second, 10*time.Millisecond)

	stopCtx, cancel := context.WithTimeout(context.Background(), time.Nanosecond)
	defer cancel()
	stopDone := make(chan error, 1)
	go func() { stopDone <- watcher.Stop(stopCtx) }()
	select {
	case err := <-stopDone:
		require.ErrorIs(t, err, context.DeadlineExceeded)
	case <-time.After(time.Second):
		t.Fatal("watcher stop did not honor deadline")
	}
	require.True(t, watcher.Running())
	require.Equal(t, int64(0), closed.Load())

	close(release)

	require.NoError(t, watcher.Stop(context.Background()))
	require.NoError(t, watcher.Stop(context.Background()))
	require.False(t, watcher.Running())
	require.Equal(t, int64(1), closed.Load())
}

type countingSubscriptionStore struct {
	subscriber    policySubscriber
	subscriptions atomic.Int64
}

func (s *countingSubscriptionStore) CurrentVersion(context.Context) (int64, error) {
	return 0, nil
}

func (s *countingSubscriptionStore) Subscribe(context.Context) policySubscriber {
	s.subscriptions.Add(1)
	return s.subscriber
}

type blockingPolicySubscriber struct {
	release <-chan struct{}
	closed  *atomic.Int64
}

func (s blockingPolicySubscriber) Receive(context.Context) (any, error) {
	<-s.release
	return nil, context.Canceled
}

func (s blockingPolicySubscriber) Channel(...rediscmd.ChannelOption) <-chan *rediscmd.Message {
	ch := make(chan *rediscmd.Message)
	close(ch)
	return ch
}

func (s blockingPolicySubscriber) Close() error {
	s.closed.Add(1)
	return nil
}

type closedChannelStore struct{}

func (s *closedChannelStore) CurrentVersion(context.Context) (int64, error) {
	return 0, nil
}

func (s *closedChannelStore) Subscribe(context.Context) policySubscriber {
	return closedPolicySubscriber{}
}

type closedPolicySubscriber struct{}

func (s closedPolicySubscriber) Receive(context.Context) (any, error) {
	return nil, nil
}

func (s closedPolicySubscriber) Channel(...rediscmd.ChannelOption) <-chan *rediscmd.Message {
	ch := make(chan *rediscmd.Message)
	close(ch)
	return ch
}

func (s closedPolicySubscriber) Close() error {
	return nil
}

type failingVersionStore struct{}

func (s *failingVersionStore) CurrentVersion(context.Context) (int64, error) {
	return 0, errors.New("redis unavailable")
}

func (s *failingVersionStore) Subscribe(context.Context) policySubscriber {
	return closedPolicySubscriber{}
}

type failingPolicyRedisClient struct {
	policyRedisClient
	evalErr      error
	publishErr   error
	publishCalls atomic.Int64
}

func testPolicyPublicationEvent(revision int64, change permissionapplication.PolicyChange) permissionapplication.OutboxEvent {
	event := permissionapplication.OutboxEvent{
		EventID:        uuid.MustParse("018f0000-0000-7000-8000-000000000801"),
		IdempotencyKey: "rbac-policy:" + change.ReasonText(),
		Revision:       revision,
		Reason:         change.ReasonText(),
	}
	if change.Kind == permissionapplication.PolicyChangeKindUserRole {
		event.Kind = policyRefreshKindUserRoleChanged
	} else {
		event.Kind = policyRefreshKindPolicyChanged
	}
	if change.UserID != uuid.Nil {
		event.UserID = &change.UserID
	}
	if change.RoleID != uuid.Nil {
		event.RoleID = &change.RoleID
	}
	if change.PermissionID != uuid.Nil {
		event.PermissionID = &change.PermissionID
	}
	return event
}

func (c *failingPolicyRedisClient) Eval(ctx context.Context, script string, keys []string, args ...any) *rediscmd.Cmd {
	if c.evalErr != nil {
		return rediscmd.NewCmdResult(nil, c.evalErr)
	}
	return c.policyRedisClient.Eval(ctx, script, keys, args...)
}

func (c *failingPolicyRedisClient) Publish(ctx context.Context, channel string, message any) *rediscmd.IntCmd {
	c.publishCalls.Add(1)
	if c.publishErr != nil {
		return rediscmd.NewIntResult(0, c.publishErr)
	}
	return c.policyRedisClient.Publish(ctx, channel, message)
}

func waitForWatcherStopped(t *testing.T, watcher *Watcher) {
	t.Helper()
	require.Eventually(t, func() bool {
		return !watcher.Running()
	}, time.Second, 10*time.Millisecond)
}
