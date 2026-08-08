package redis

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/google/uuid"
	rediscmd "github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"github.com/aegiscore/common/runtime/redispubsub"
	permissionapplication "github.com/aegiscore/user-service/internal/features/permission/application"
)

func newWatcherForTest(source messageSource, revisionSource permissionapplication.LatestPolicyRevisionSource, engine permissionapplication.PolicyReloadEngine, checkInterval time.Duration, metrics permissionapplication.Metrics) *Watcher {
	if source == nil {
		source = newFakeMessageSource(redispubsub.Status{State: redispubsub.StateStopped, ErrorCategory: redispubsub.ErrorNone})
	}
	return newWatcher(source, revisionSource, engine, nil, WatcherSettings{CheckInterval: checkInterval}, metrics)
}

func TestStorePublishPolicyRevisionCachesSuppliedRevisionAndPublishes(t *testing.T) {
	redisServer := miniredis.RunT(t)
	client := rediscmd.NewClient(&rediscmd.Options{Addr: redisServer.Addr()})
	t.Cleanup(func() { _ = client.Close() })
	store := newStore(client, mustKeyCatalog("aegiscore-user-service"), "instance-a", nil)
	require.Equal(t, store.keys.PolicyChannel(), store.PolicyChannel())
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
	require.NotNil(t, decoded.PolicyRevision)
	require.Equal(t, revision, *decoded.PolicyRevision)
	require.Nil(t, decoded.UserRoleRevision)
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
	require.NotNil(t, decoded.PolicyRevision)
	require.Equal(t, int64(41), *decoded.PolicyRevision)
}

func TestStorePublishUserRoleRevisionPublishesWithoutPolicyVersionCache(t *testing.T) {
	redisServer := miniredis.RunT(t)
	client := rediscmd.NewClient(&rediscmd.Options{Addr: redisServer.Addr()})
	t.Cleanup(func() { _ = client.Close() })
	store := newStore(client, mustKeyCatalog("aegiscore-user-service"), "instance-a", nil)
	pubsub := client.Subscribe(context.Background(), store.keys.PolicyChannel())
	t.Cleanup(func() { _ = pubsub.Close() })
	_, err := pubsub.Receive(context.Background())
	require.NoError(t, err)
	userID := uuid.MustParse("018f0000-0000-7000-8000-000000000717")
	roleID := uuid.MustParse("018f0000-0000-7000-8000-000000000718")

	const revision int64 = 17
	event := testPolicyPublicationEvent(revision, permissionapplication.NewUserRoleChange("user_role_added", userID, roleID))
	require.NoError(t, store.PublishPolicyRevision(context.Background(), event))
	storedRevision, err := store.CurrentVersion(context.Background())
	require.NoError(t, err)
	require.Zero(t, storedRevision)
	message, err := pubsub.ReceiveMessage(context.Background())
	require.NoError(t, err)
	decoded, err := decodePolicyRefreshMessage(message.Payload)
	require.NoError(t, err)
	require.Nil(t, decoded.PolicyRevision)
	require.NotNil(t, decoded.UserRoleRevision)
	require.Equal(t, revision, *decoded.UserRoleRevision)
	require.Equal(t, userID, *decoded.UserID)
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

func TestWatcherHandlePayloadSkipsReloadForDuplicateAppliedPolicyEvent(t *testing.T) {
	ctrl := gomock.NewController(t)
	engine := NewMockPolicyReloadEngine(ctrl)
	var applied atomic.Int64
	engine.EXPECT().AppliedRevision().DoAndReturn(func() int64 { return applied.Load() }).AnyTimes()
	engine.EXPECT().RefreshToRevision(gomock.Any(), int64(3)).DoAndReturn(func(context.Context, int64) (int64, error) {
		applied.Store(3)
		return 3, nil
	}).Times(1)
	engine.EXPECT().ProjectionStatus().Return(permissionapplication.PolicyProjectionStatus{Initialized: true, ReloadSucceeded: true, AppliedRevision: 3, TargetRevision: 3}).Times(3)
	engine.EXPECT().InvalidateAllUserRoles().Times(2)
	metrics := NewMockMetrics(ctrl)
	watcher := newWatcherForTest(nil, staticPolicyRevisionSource{revision: 3}, engine, time.Second, metrics)
	payload, err := encodePolicyRefreshMessage(newPolicyRefreshMessage(testPolicyPublicationEvent(3, permissionapplication.NewPolicyReloadChange("role_permission_added")), "instance-b"))
	require.NoError(t, err)

	gomock.InOrder(
		metrics.EXPECT().PolicyReloadLagObserved(gomock.Any(), int64(3)),
		metrics.EXPECT().WatcherVersionMismatch(gomock.Any(), permissionapplication.MetricsSourceWatcherPubSub, permissionapplication.MetricsReasonRevisionMismatch),
		metrics.EXPECT().WatcherReloadSucceeded(gomock.Any(), permissionapplication.MetricsSourceWatcherPubSub),
		metrics.EXPECT().PolicyReloadLagObserved(gomock.Any(), int64(0)),
		metrics.EXPECT().PolicyReloadLagObserved(gomock.Any(), int64(0)),
	)

	watcher.HandlePayload(context.Background(), payload)
	watcher.HandlePayload(context.Background(), payload)

	require.Equal(t, int64(3), applied.Load())
}

func TestWatcherHandlePayloadUserRoleEventOnlyInvalidatesTargetUser(t *testing.T) {
	userID := uuid.MustParse("018f0000-0000-7000-8000-000000000701")
	roleID := uuid.MustParse("018f0000-0000-7000-8000-000000000702")
	ctrl := gomock.NewController(t)
	engine := NewMockPolicyReloadEngine(ctrl)
	engine.EXPECT().AppliedRevision().Return(int64(0)).AnyTimes()
	engine.EXPECT().InvalidateUserRole(userID)
	metrics := NewMockMetrics(ctrl)
	watcher := newWatcherForTest(nil, staticPolicyRevisionSource{revision: 4}, engine, time.Second, metrics)
	payload, err := encodePolicyRefreshMessage(newPolicyRefreshMessage(testPolicyPublicationEvent(4, permissionapplication.NewUserRoleChange("user_role_added", userID, roleID)), "instance-b"))
	require.NoError(t, err)

	watcher.HandlePayload(context.Background(), payload)

	require.Zero(t, engine.AppliedRevision())
}

func TestWatcherHandlePayloadExecutesOutOfOrderUserRoleEventWithoutMovingAppliedRevision(t *testing.T) {
	userID := uuid.MustParse("018f0000-0000-7000-8000-000000000703")
	roleID := uuid.MustParse("018f0000-0000-7000-8000-000000000704")
	ctrl := gomock.NewController(t)
	engine := NewMockPolicyReloadEngine(ctrl)
	engine.EXPECT().AppliedRevision().Return(int64(9)).AnyTimes()
	engine.EXPECT().InvalidateUserRole(userID)
	metrics := NewMockMetrics(ctrl)
	watcher := newWatcherForTest(nil, staticPolicyRevisionSource{revision: 9}, engine, time.Second, metrics)
	payload, err := encodePolicyRefreshMessage(newPolicyRefreshMessage(testPolicyPublicationEvent(4, permissionapplication.NewUserRoleChange("user_role_removed", userID, roleID)), "instance-b"))
	require.NoError(t, err)

	watcher.HandlePayload(context.Background(), payload)

	require.Equal(t, int64(9), engine.AppliedRevision())
}

func TestWatcherHandlePayloadReloadsOutOfOrderPolicyEventWithoutMovingAppliedRevision(t *testing.T) {
	ctrl := gomock.NewController(t)
	engine := NewMockPolicyReloadEngine(ctrl)
	engine.EXPECT().AppliedRevision().Return(int64(9)).AnyTimes()
	engine.EXPECT().ProjectionStatus().Return(permissionapplication.PolicyProjectionStatus{Initialized: true, ReloadSucceeded: true, AppliedRevision: 9, TargetRevision: 9})
	engine.EXPECT().InvalidateAllUserRoles()
	metrics := NewMockMetrics(ctrl)
	watcher := newWatcherForTest(nil, staticPolicyRevisionSource{revision: 9}, engine, time.Second, metrics)
	payload, err := encodePolicyRefreshMessage(newPolicyRefreshMessage(testPolicyPublicationEvent(4, permissionapplication.NewPolicyReloadChange("role_updated")), "instance-b"))
	require.NoError(t, err)

	gomock.InOrder(
		metrics.EXPECT().PolicyReloadLagObserved(gomock.Any(), int64(0)),
	)

	watcher.HandlePayload(context.Background(), payload)

	require.Equal(t, int64(9), engine.AppliedRevision())
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
	ctrl := gomock.NewController(t)
	engine := NewMockPolicyReloadEngine(ctrl)
	var applied atomic.Int64
	applied.Store(4)
	engine.EXPECT().AppliedRevision().DoAndReturn(func() int64 { return applied.Load() }).AnyTimes()
	engine.EXPECT().ProjectionStatus().Return(permissionapplication.PolicyProjectionStatus{Initialized: true, ReloadSucceeded: true, AppliedRevision: 4, TargetRevision: 4}).Times(2)
	engine.EXPECT().RefreshToRevision(gomock.Any(), int64(8)).DoAndReturn(func(context.Context, int64) (int64, error) {
		applied.Store(8)
		return 8, nil
	})
	engine.EXPECT().ProjectionStatus().Return(permissionapplication.PolicyProjectionStatus{Initialized: true, ReloadSucceeded: true, AppliedRevision: 8, TargetRevision: 8})
	engine.EXPECT().InvalidateAllUserRoles()
	metrics := NewMockMetrics(ctrl)
	watcher := newWatcherForTest(nil, staticPolicyRevisionSource{revision: 8}, engine, time.Second, metrics)

	gomock.InOrder(
		metrics.EXPECT().PolicyReloadLagObserved(gomock.Any(), int64(4)),
		metrics.EXPECT().WatcherVersionMismatch(gomock.Any(), permissionapplication.MetricsSourceWatcherRevisionCheck, permissionapplication.MetricsReasonRevisionMismatch),
		metrics.EXPECT().WatcherReloadSucceeded(gomock.Any(), permissionapplication.MetricsSourceWatcherRevisionCheck),
		metrics.EXPECT().PolicyReloadLagObserved(gomock.Any(), int64(0)),
	)

	watcher.CheckVersion(context.Background())

	require.Equal(t, int64(8), applied.Load())
}

func TestWatcherFaultInjectionRedisFailureRecoveryConvergesWithoutNewWrite(t *testing.T) {
	redisServer := miniredis.RunT(t)
	client := rediscmd.NewClient(&rediscmd.Options{Addr: redisServer.Addr()})
	t.Cleanup(func() { _ = client.Close() })
	store := newStore(client, mustKeyCatalog("aegiscore-user-service"), "instance-a", nil)
	publishErr := errors.New("redis publish failed")
	failingClient := &failingPolicyRedisClient{policyRedisClient: client, publishErr: publishErr}
	failingStore := newStore(failingClient, mustKeyCatalog("aegiscore-user-service"), "instance-a", nil)
	change := permissionapplication.NewPolicyReloadChange("role_permission_added")
	const databaseRevision int64 = 11

	err := failingStore.PublishPolicyRevision(context.Background(), testPolicyPublicationEvent(databaseRevision, change))
	require.ErrorIs(t, err, publishErr)
	storedRevision, err := store.CurrentVersion(context.Background())
	require.NoError(t, err)
	require.Equal(t, databaseRevision, storedRevision)

	engine := &faultInjectedPolicyReloadEngine{appliedRevision: 4, targetRevision: 4, ready: true}
	watcher := newWatcherForTest(nil, staticPolicyRevisionSource{revision: databaseRevision}, engine, time.Second, nil)
	watcher.CheckVersion(context.Background())

	requireEventuallyWatcherProjection(t, engine, databaseRevision)
	require.Equal(t, int64(1), engine.invalidateAllCount.Load())
}

func TestWatcherCheckVersionSkipsWhenProjectionIsAuthoritativelyReady(t *testing.T) {
	engine := NewMockPolicyReloadEngine(gomock.NewController(t))
	engine.EXPECT().AppliedRevision().Return(int64(9))
	engine.EXPECT().ProjectionStatus().Return(permissionapplication.PolicyProjectionStatus{Initialized: true, ReloadSucceeded: true, AppliedRevision: 9, TargetRevision: 9})
	metrics := NewMockMetrics(gomock.NewController(t))
	metrics.EXPECT().PolicyReloadLagObserved(gomock.Any(), int64(0))
	watcher := newWatcherForTest(nil, staticPolicyRevisionSource{revision: 4}, engine, time.Second, metrics)

	watcher.CheckVersion(context.Background())

}

func TestWatcherCheckVersionRetriesAfterPriorFailureAtEqualRevision(t *testing.T) {
	ctrl := gomock.NewController(t)
	engine := NewMockPolicyReloadEngine(ctrl)
	engine.EXPECT().AppliedRevision().Return(int64(8)).AnyTimes()
	engine.EXPECT().ProjectionStatus().Return(permissionapplication.PolicyProjectionStatus{Initialized: true, AppliedRevision: 8, TargetRevision: 8, LastError: errors.New("prior reload failed")}).Times(2)
	engine.EXPECT().RefreshToRevision(gomock.Any(), int64(8)).Return(int64(8), nil)
	engine.EXPECT().ProjectionStatus().Return(permissionapplication.PolicyProjectionStatus{Initialized: true, ReloadSucceeded: true, AppliedRevision: 8, TargetRevision: 8})
	engine.EXPECT().InvalidateAllUserRoles()
	metrics := NewMockMetrics(ctrl)
	metrics.EXPECT().PolicyReloadLagObserved(gomock.Any(), int64(0)).Times(2)
	metrics.EXPECT().WatcherVersionMismatch(gomock.Any(), permissionapplication.MetricsSourceWatcherRevisionCheck, permissionapplication.MetricsReasonRevisionMismatch)
	metrics.EXPECT().WatcherReloadSucceeded(gomock.Any(), permissionapplication.MetricsSourceWatcherRevisionCheck)
	watcher := newWatcherForTest(nil, staticPolicyRevisionSource{revision: 8}, engine, time.Second, metrics)

	watcher.CheckVersion(context.Background())
}

func TestWatcherFaultInjectionReplayAddRemoveReplaceEventsKeepsIdempotentProjection(t *testing.T) {
	userID := uuid.MustParse("018f0000-0000-7000-8000-000000000711")
	roleID := uuid.MustParse("018f0000-0000-7000-8000-000000000712")
	engine := &faultInjectedPolicyReloadEngine{appliedRevision: 1, targetRevision: 1, ready: true}
	revisions := &sequencePolicyRevisionSource{results: []policyRevisionResult{
		{revision: 4}, {revision: 4}, {revision: 4}, {revision: 4},
	}}
	watcher := newWatcherForTest(nil, revisions, engine, time.Second, nil)
	payloads := []string{
		mustPolicyPayload(t, testPolicyPublicationEvent(2, permissionapplication.NewUserRoleChange("user_role_added", userID, roleID))),
		mustPolicyPayload(t, testPolicyPublicationEvent(3, permissionapplication.NewUserRoleChange("user_role_removed", userID, roleID))),
		mustPolicyPayload(t, testPolicyPublicationEvent(4, permissionapplication.NewPolicyReloadChange("role_permissions_replaced"))),
		mustPolicyPayload(t, testPolicyPublicationEvent(4, permissionapplication.NewPolicyReloadChange("role_permissions_replaced"))),
	}

	for _, payload := range payloads {
		watcher.HandlePayload(context.Background(), payload)
	}

	requireEventuallyWatcherProjection(t, engine, 4)
	require.Equal(t, int64(2), engine.invalidateUserCount.Load())
	require.Equal(t, int64(2), engine.invalidateAllCount.Load())
	require.Equal(t, []int64{4}, engine.reloads())
}

func TestWatcherLoopConcurrentHintsAndTickerConvergesToAuthoritativeRevision(t *testing.T) {
	userID := uuid.MustParse("018f0000-0000-7000-8000-000000000713")
	roleID := uuid.MustParse("018f0000-0000-7000-8000-000000000714")
	source := newFakeMessageSource(redispubsub.Status{State: redispubsub.StateCreated, ErrorCategory: redispubsub.ErrorNone})
	revisions := &atomicPolicyRevisionSource{}
	revisions.Store(1)
	engine := &faultInjectedPolicyReloadEngine{appliedRevision: 1, targetRevision: 1, ready: true}
	watcher := newWatcherForTest(source, revisions, engine, time.Millisecond, nil)

	require.NoError(t, watcher.Start(context.Background()))
	require.Eventually(t, func() bool {
		return !watcher.Status().LastReconcileSuccessAt.IsZero()
	}, time.Second, time.Millisecond)

	revisions.Store(12)
	payloads := []string{
		mustPolicyPayload(t, testPolicyPublicationEvent(3, permissionapplication.NewUserRoleChange("user_role_added", userID, roleID))),
		mustPolicyPayload(t, testPolicyPublicationEvent(8, permissionapplication.NewPolicyReloadChange("role_permissions_replaced"))),
		mustPolicyPayload(t, testPolicyPublicationEvent(12, permissionapplication.NewUserRoleChange("user_role_removed", userID, roleID))),
		mustPolicyPayload(t, testPolicyPublicationEvent(6, permissionapplication.NewPolicyReloadChange("role_updated"))),
	}
	var wg sync.WaitGroup
	for _, payload := range payloads {
		wg.Add(1)
		go func(payload string) {
			defer wg.Done()
			source.messages <- redispubsub.Message{Channel: "rbac", Payload: payload}
		}(payload)
	}
	wg.Wait()

	requireEventuallyWatcherProjection(t, engine, 12)
	require.GreaterOrEqual(t, engine.invalidateAllCount.Load(), int64(1))
	require.Equal(t, int64(12), engine.AppliedRevision())
	require.NoError(t, watcher.Stop(context.Background()))
	status := watcher.Status()
	require.False(t, status.Running)
	require.Equal(t, permissionapplication.PolicyWatcherSubscriptionStopped, status.SubscriptionState)
}

func TestWatcherCoalescesDuplicatePolicyNotificationsToSingleReload(t *testing.T) {
	engine := &faultInjectedPolicyReloadEngine{appliedRevision: 1, targetRevision: 1, ready: true}
	watcher := newWatcherForTest(nil, staticPolicyRevisionSource{revision: 10}, engine, time.Second, nil)
	payloads := make([]string, 0, 100)
	for range 100 {
		payloads = append(payloads, mustPolicyPayload(t, testPolicyPublicationEvent(10, permissionapplication.NewPolicyReloadChange("role_permissions_replaced"))))
	}

	watcher.HandlePayloads(context.Background(), payloads)

	requireEventuallyWatcherProjection(t, engine, 10)
	require.Equal(t, []int64{10}, engine.reloads())
}

func TestWatcherPureUserRoleNotificationsDoNotReloadPolicy(t *testing.T) {
	userID := uuid.MustParse("018f0000-0000-7000-8000-000000000715")
	roleID := uuid.MustParse("018f0000-0000-7000-8000-000000000716")
	engine := &faultInjectedPolicyReloadEngine{appliedRevision: 10, targetRevision: 10, ready: true}
	watcher := newWatcherForTest(nil, staticPolicyRevisionSource{revision: 10}, engine, time.Second, nil)
	payloads := make([]string, 0, 100)
	for index := range 100 {
		payloads = append(payloads, mustPolicyPayload(t, testPolicyPublicationEvent(int64(index+1), permissionapplication.NewUserRoleChange("user_role_added", userID, roleID))))
	}

	watcher.HandlePayloads(context.Background(), payloads)

	require.Empty(t, engine.reloads())
	require.Equal(t, int64(100), engine.invalidateUserCount.Load())
	require.Equal(t, int64(0), engine.invalidateAllCount.Load())
}

func TestWatcherReloadFailurePreservesAppliedVersion(t *testing.T) {
	reloadErr := errors.New("reload failed")
	ctrl := gomock.NewController(t)
	engine := NewMockPolicyReloadEngine(ctrl)
	engine.EXPECT().AppliedRevision().Return(int64(2)).AnyTimes()
	engine.EXPECT().ProjectionStatus().Return(permissionapplication.PolicyProjectionStatus{Initialized: true, AppliedRevision: 2, TargetRevision: 5, LastError: reloadErr})
	engine.EXPECT().RefreshToRevision(gomock.Any(), int64(5)).Return(int64(2), reloadErr)
	metrics := NewMockMetrics(ctrl)
	watcher := newWatcherForTest(nil, staticPolicyRevisionSource{revision: 5}, engine, time.Second, metrics)
	payload, err := encodePolicyRefreshMessage(newPolicyRefreshMessage(testPolicyPublicationEvent(5, permissionapplication.NewPolicyReloadChange("permission_updated")), "instance-b"))
	require.NoError(t, err)

	gomock.InOrder(
		metrics.EXPECT().PolicyReloadLagObserved(gomock.Any(), int64(3)),
		metrics.EXPECT().WatcherVersionMismatch(gomock.Any(), permissionapplication.MetricsSourceWatcherPubSub, permissionapplication.MetricsReasonRevisionMismatch),
		metrics.EXPECT().WatcherReloadFailed(gomock.Any(), permissionapplication.MetricsSourceWatcherPubSub, permissionapplication.MetricsReasonReloadFailed),
		metrics.EXPECT().PolicyReloadLagObserved(gomock.Any(), int64(3)),
	)

	watcher.HandlePayload(context.Background(), payload)

	require.Equal(t, int64(2), engine.AppliedRevision())
}

func TestWatcherCheckVersionFailureDoesNotClearLag(t *testing.T) {
	ctrl := gomock.NewController(t)
	engine := NewMockPolicyReloadEngine(ctrl)
	metrics := NewMockMetrics(ctrl)
	watcher := newWatcherForTest(nil, failingPolicyRevisionSource{err: errors.New("database unavailable")}, engine, time.Second, metrics)

	engine.EXPECT().AppliedRevision().Return(int64(2))
	metrics.EXPECT().WatcherCheckFailed(gomock.Any(), permissionapplication.MetricsSourceWatcherRevisionCheck, permissionapplication.MetricsReasonRevisionStoreUnavailable)

	watcher.CheckVersion(context.Background())
}

func TestWatcherCheckVersionCancellationDoesNotRecordFailure(t *testing.T) {
	ctrl := gomock.NewController(t)
	engine := NewMockPolicyReloadEngine(ctrl)
	metrics := NewMockMetrics(ctrl)
	watcher := newWatcherForTest(nil, failingPolicyRevisionSource{err: context.Canceled}, engine, time.Second, metrics)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	watcher.CheckVersion(ctx)

	status := watcher.Status()
	require.Equal(t, permissionapplication.PolicyWatcherErrorNone, status.ReconcileErrorCategory)
	require.True(t, status.LastFailureAt.IsZero())
}

func TestWatcherCheckVersionRecoversFromRevisionSourceAndReloadFailure(t *testing.T) {
	revisionErr := errors.New("database unavailable")
	reloadErr := errors.New("reload failed")
	revisions := &sequencePolicyRevisionSource{results: []policyRevisionResult{{err: revisionErr}, {revision: 7}, {revision: 7}}}
	ctrl := gomock.NewController(t)
	engine := NewMockPolicyReloadEngine(ctrl)
	var applied atomic.Int64
	applied.Store(3)
	engine.EXPECT().AppliedRevision().DoAndReturn(func() int64 { return applied.Load() }).AnyTimes()
	engine.EXPECT().ProjectionStatus().Return(permissionapplication.PolicyProjectionStatus{Initialized: true, ReloadSucceeded: true, AppliedRevision: 3, TargetRevision: 3})
	engine.EXPECT().RefreshToRevision(gomock.Any(), int64(7)).Return(int64(3), reloadErr)
	engine.EXPECT().ProjectionStatus().Return(permissionapplication.PolicyProjectionStatus{Initialized: true, AppliedRevision: 3, TargetRevision: 7, LastError: reloadErr}).Times(3)
	engine.EXPECT().RefreshToRevision(gomock.Any(), int64(7)).DoAndReturn(func(context.Context, int64) (int64, error) {
		applied.Store(7)
		return 7, nil
	})
	engine.EXPECT().ProjectionStatus().Return(permissionapplication.PolicyProjectionStatus{Initialized: true, ReloadSucceeded: true, AppliedRevision: 7, TargetRevision: 7})
	engine.EXPECT().InvalidateAllUserRoles()
	metrics := NewMockMetrics(ctrl)
	watcher := newWatcherForTest(nil, revisions, engine, time.Second, metrics)

	gomock.InOrder(
		metrics.EXPECT().WatcherCheckFailed(gomock.Any(), permissionapplication.MetricsSourceWatcherRevisionCheck, permissionapplication.MetricsReasonRevisionStoreUnavailable),
		metrics.EXPECT().PolicyReloadLagObserved(gomock.Any(), int64(4)),
		metrics.EXPECT().WatcherVersionMismatch(gomock.Any(), permissionapplication.MetricsSourceWatcherRevisionCheck, permissionapplication.MetricsReasonRevisionMismatch),
		metrics.EXPECT().WatcherReloadFailed(gomock.Any(), permissionapplication.MetricsSourceWatcherRevisionCheck, permissionapplication.MetricsReasonReloadFailed),
		metrics.EXPECT().PolicyReloadLagObserved(gomock.Any(), int64(4)),
		metrics.EXPECT().PolicyReloadLagObserved(gomock.Any(), int64(4)),
		metrics.EXPECT().WatcherVersionMismatch(gomock.Any(), permissionapplication.MetricsSourceWatcherRevisionCheck, permissionapplication.MetricsReasonRevisionMismatch),
		metrics.EXPECT().WatcherReloadSucceeded(gomock.Any(), permissionapplication.MetricsSourceWatcherRevisionCheck),
		metrics.EXPECT().PolicyReloadLagObserved(gomock.Any(), int64(0)),
	)

	watcher.CheckVersion(context.Background())
	status := watcher.Status()
	require.Equal(t, permissionapplication.PolicyWatcherErrorRevisionSource, status.ReconcileErrorCategory)
	require.True(t, status.LastReconcileSuccessAt.IsZero())
	require.False(t, status.LastFailureAt.IsZero())
	watcher.CheckVersion(context.Background())
	status = watcher.Status()
	require.Equal(t, permissionapplication.PolicyWatcherErrorReload, status.ReconcileErrorCategory)
	require.True(t, status.LastReconcileSuccessAt.IsZero())
	watcher.CheckVersion(context.Background())
	require.Equal(t, int64(7), applied.Load())
	status = watcher.Status()
	require.Equal(t, permissionapplication.PolicyWatcherErrorNone, status.ReconcileErrorCategory)
	require.False(t, status.LastReconcileSuccessAt.IsZero())
	require.False(t, status.LastFailureAt.IsZero())
}

func TestWatcherHandlePayloadRevisionSourceFailureDoesNotUseHint(t *testing.T) {
	ctrl := gomock.NewController(t)
	engine := NewMockPolicyReloadEngine(ctrl)
	engine.EXPECT().AppliedRevision().Return(int64(2)).Times(3)
	engine.EXPECT().InvalidateAllUserRoles()
	metrics := NewMockMetrics(ctrl)
	watcher := newWatcherForTest(nil, failingPolicyRevisionSource{err: errors.New("database unavailable")}, engine, time.Second, metrics)
	payload, err := encodePolicyRefreshMessage(newPolicyRefreshMessage(testPolicyPublicationEvent(99, permissionapplication.NewPolicyReloadChange("role_updated")), "instance-b"))
	require.NoError(t, err)
	metrics.EXPECT().WatcherCheckFailed(gomock.Any(), permissionapplication.MetricsSourceWatcherPubSub, permissionapplication.MetricsReasonRevisionStoreUnavailable)

	watcher.HandlePayload(context.Background(), payload)
	status := watcher.Status()
	require.Equal(t, permissionapplication.PolicyWatcherErrorRevisionSource, status.ReconcileErrorCategory)
	require.Equal(t, permissionapplication.PolicyWatcherErrorNone, status.SubscriptionErrorCategory)
}

func TestWatcherRunningStatus(t *testing.T) {
	source := newFakeMessageSource(redispubsub.Status{State: redispubsub.StateCreated, ErrorCategory: redispubsub.ErrorNone})
	engine := &faultInjectedPolicyReloadEngine{ready: true}
	watcher := newWatcherForTest(source, staticPolicyRevisionSource{}, engine, time.Hour, nil)

	require.False(t, watcher.Status().Running)
	require.NoError(t, watcher.Start(context.Background()))
	require.NoError(t, watcher.Start(context.Background()))
	require.Eventually(t, func() bool {
		status := watcher.Status()
		return status.Running && status.SubscriptionState == permissionapplication.PolicyWatcherSubscriptionConnected && !status.LastReconcileSuccessAt.IsZero()
	}, time.Second, time.Millisecond)
	require.Equal(t, int64(1), source.startCalls.Load())
	require.NoError(t, watcher.Stop(context.Background()))
	require.NoError(t, watcher.Stop(context.Background()))
	status := watcher.Status()
	require.False(t, status.Running)
	require.Equal(t, permissionapplication.PolicyWatcherSubscriptionStopped, status.SubscriptionState)
	require.Equal(t, permissionapplication.PolicyWatcherErrorNone, status.SubscriptionErrorCategory)
	require.ErrorIs(t, watcher.Start(context.Background()), redispubsub.ErrStopped)
}

func TestWatcherStartDerivesRunContextFromLifecycleContext(t *testing.T) {
	const contextValue = "lifecycle"

	source := newFakeMessageSource(redispubsub.Status{State: redispubsub.StateCreated, ErrorCategory: redispubsub.ErrorNone})
	revisions := &capturingPolicyRevisionSource{value: make(chan any, 1)}
	engine := &faultInjectedPolicyReloadEngine{ready: true}
	watcher := newWatcherForTest(source, revisions, engine, time.Hour, nil)
	startCtx, cancel := context.WithCancel(context.WithValue(context.Background(), watcherLifecycleContextKey{}, contextValue))

	require.NoError(t, watcher.Start(startCtx))

	select {
	case value := <-revisions.value:
		require.Equal(t, contextValue, value)
	case <-time.After(time.Second):
		t.Fatal("watcher did not use lifecycle context for revision check")
	}
	require.True(t, watcher.Status().Running)
	cancel()
	require.Eventually(t, func() bool { return !watcher.Status().Running }, time.Second, time.Millisecond)
	require.NoError(t, watcher.Stop(context.Background()))
}

func TestWatcherReconcilesWhileSubscriptionIsReconnecting(t *testing.T) {
	failureAt := time.Now().Add(-time.Second)
	source := newFakeMessageSource(redispubsub.Status{
		Running: true, State: redispubsub.StateReconnecting, ErrorCategory: redispubsub.ErrorSubscribe,
		LastFailureAt: failureAt, Reconnects: 3,
	})
	engine := &faultInjectedPolicyReloadEngine{appliedRevision: 2, targetRevision: 2, ready: true}
	watcher := newWatcherForTest(source, staticPolicyRevisionSource{revision: 8}, engine, 5*time.Millisecond, nil)

	require.NoError(t, watcher.Start(context.Background()))
	source.setStatus(redispubsub.Status{
		Running: true, State: redispubsub.StateReconnecting, ErrorCategory: redispubsub.ErrorSubscribe,
		LastFailureAt: failureAt, Reconnects: 3,
	})
	requireEventuallyWatcherProjection(t, engine, 8)
	require.Eventually(t, func() bool {
		status := watcher.Status()
		return status.Running && status.SubscriptionState == permissionapplication.PolicyWatcherSubscriptionReconnecting &&
			status.SubscriptionErrorCategory == permissionapplication.PolicyWatcherErrorSubscribe &&
			status.ReconnectAttempts == 3 && !status.LastReconcileSuccessAt.IsZero()
	}, time.Second, time.Millisecond)
	connectedAt := time.Now()
	source.setStatus(redispubsub.Status{
		Running: true, State: redispubsub.StateConnected, ErrorCategory: redispubsub.ErrorNone,
		LastConnectedAt: connectedAt, Reconnects: 3,
	})
	require.Eventually(t, func() bool {
		status := watcher.Status()
		return status.Running && status.SubscriptionState == permissionapplication.PolicyWatcherSubscriptionConnected &&
			status.SubscriptionErrorCategory == permissionapplication.PolicyWatcherErrorNone &&
			status.LastSubscriptionSuccessAt.Equal(connectedAt)
	}, time.Second, time.Millisecond)
	require.NoError(t, watcher.Stop(context.Background()))
}

func TestWatcherKeepsReconcilingAfterMessageChannelCloses(t *testing.T) {
	source := newFakeMessageSource(redispubsub.Status{State: redispubsub.StateCreated, ErrorCategory: redispubsub.ErrorNone})
	revisions := &atomicPolicyRevisionSource{}
	revisions.Store(2)
	engine := &faultInjectedPolicyReloadEngine{appliedRevision: 2, targetRevision: 2, ready: true}
	watcher := newWatcherForTest(source, revisions, engine, time.Millisecond, nil)

	require.NoError(t, watcher.Start(context.Background()))
	require.Eventually(t, func() bool {
		return watcher.Status().Running && !watcher.Status().LastReconcileSuccessAt.IsZero()
	}, time.Second, time.Millisecond)

	source.closeOnce.Do(func() { close(source.messages) })
	revisions.Store(9)

	requireEventuallyWatcherProjection(t, engine, 9)
	status := watcher.Status()
	require.True(t, status.Running)
	require.Equal(t, permissionapplication.PolicyWatcherSubscriptionConnected, status.SubscriptionState)
	require.Equal(t, permissionapplication.PolicyWatcherErrorNone, status.ReconcileErrorCategory)
	require.NoError(t, watcher.Stop(context.Background()))
}

func TestWatcherStopCancelsBlockedPayloadWithoutRecordingFailure(t *testing.T) {
	payload := mustPolicyPayload(t, testPolicyPublicationEvent(1, permissionapplication.NewPolicyReloadChange("role_updated")))
	source := newFakeMessageSource(redispubsub.Status{State: redispubsub.StateCreated, ErrorCategory: redispubsub.ErrorNone})
	revisions := newBlockingAfterFirstRevisionSource()
	engine := &faultInjectedPolicyReloadEngine{ready: true}
	watcher := newWatcherForTest(source, revisions, engine, time.Hour, nil)

	require.NoError(t, watcher.Start(context.Background()))
	source.messages <- redispubsub.Message{Channel: "rbac", Payload: payload}
	select {
	case <-revisions.blocked:
	case <-time.After(time.Second):
		t.Fatal("watcher payload did not reach the blocking revision query")
	}
	require.NoError(t, watcher.Stop(context.Background()))
	status := watcher.Status()
	require.False(t, status.Running)
	require.True(t, status.LastFailureAt.IsZero())
}

func TestWatcherStopCancelsBlockedReloadWithoutRecordingFailure(t *testing.T) {
	payload := mustPolicyPayload(t, testPolicyPublicationEvent(9, permissionapplication.NewPolicyReloadChange("role_updated")))
	source := newFakeMessageSource(redispubsub.Status{State: redispubsub.StateCreated, ErrorCategory: redispubsub.ErrorNone})
	revisions := &sequencePolicyRevisionSource{results: []policyRevisionResult{{revision: 0}, {revision: 9}}}
	engine := newBlockingPolicyReloadEngine()
	watcher := newWatcherForTest(source, revisions, engine, time.Hour, nil)

	require.NoError(t, watcher.Start(context.Background()))
	source.messages <- redispubsub.Message{Channel: "rbac", Payload: payload}
	select {
	case <-engine.started:
	case <-time.After(time.Second):
		t.Fatal("watcher reload did not reach the blocking engine")
	}

	require.NoError(t, watcher.Stop(context.Background()))
	status := watcher.Status()
	require.False(t, status.Running)
	require.Equal(t, permissionapplication.PolicyWatcherSubscriptionStopped, status.SubscriptionState)
	require.Equal(t, permissionapplication.PolicyWatcherErrorNone, status.ReconcileErrorCategory)
	require.True(t, status.LastFailureAt.IsZero())
	require.Equal(t, int64(0), engine.AppliedRevision())
}

func TestNewWatcherDoesNotStartBackgroundLoop(t *testing.T) {
	source := newFakeMessageSource(redispubsub.Status{State: redispubsub.StateCreated, ErrorCategory: redispubsub.ErrorNone})
	engine := NewMockPolicyReloadEngine(gomock.NewController(t))
	watcher := NewWatcher(WatcherParams{
		Subscriber:     nil,
		Engine:         engine,
		RevisionSource: staticPolicyRevisionSource{},
	})
	watcher.source = source

	require.False(t, watcher.Status().Running)
	require.Equal(t, int64(0), source.startCalls.Load())
}

func TestWatcherStartPropagatesMessageSourceError(t *testing.T) {
	startErr := errors.New("subscriber start failed")
	source := newFakeMessageSource(redispubsub.Status{State: redispubsub.StateStopped, ErrorCategory: redispubsub.ErrorNone})
	source.startErr = startErr
	engine := &faultInjectedPolicyReloadEngine{ready: true}
	watcher := newWatcherForTest(source, staticPolicyRevisionSource{}, engine, time.Hour, nil)

	require.ErrorIs(t, watcher.Start(context.Background()), startErr)
	require.False(t, watcher.Status().Running)
	require.Equal(t, int64(1), source.startCalls.Load())
}

func TestWatcherMapsSubscriptionStatusAndUsesNewestFailure(t *testing.T) {
	subscriptionFailure := time.Now().Add(-2 * time.Minute)
	connectedAt := time.Now().Add(-time.Minute)
	source := newFakeMessageSource(redispubsub.Status{
		Running: true, State: redispubsub.StateReconnecting, ErrorCategory: redispubsub.ErrorProtocol,
		LastConnectedAt: connectedAt, LastFailureAt: subscriptionFailure, Reconnects: 7,
	})
	watcher := newWatcherForTest(source, staticPolicyRevisionSource{}, &faultInjectedPolicyReloadEngine{ready: true}, time.Hour, nil)
	reconcileFailure := time.Now()
	watcher.mu.Lock()
	watcher.status.Running = true
	watcher.status.ReconcileErrorCategory = permissionapplication.PolicyWatcherErrorReload
	watcher.lastReconcileFailureAt = reconcileFailure
	watcher.mu.Unlock()

	status := watcher.Status()
	require.True(t, status.Running)
	require.Equal(t, permissionapplication.PolicyWatcherSubscriptionReconnecting, status.SubscriptionState)
	require.Equal(t, permissionapplication.PolicyWatcherErrorProtocol, status.SubscriptionErrorCategory)
	require.Equal(t, permissionapplication.PolicyWatcherErrorReload, status.ReconcileErrorCategory)
	require.Equal(t, connectedAt, status.LastSubscriptionSuccessAt)
	require.Equal(t, reconcileFailure, status.LastFailureAt)
	require.Equal(t, uint64(7), status.ReconnectAttempts)
}

func TestWatcherStopHonorsDeadlineAndCanBeRepeated(t *testing.T) {
	release := make(chan struct{})
	source := newFakeMessageSource(redispubsub.Status{State: redispubsub.StateCreated, ErrorCategory: redispubsub.ErrorNone})
	source.stopRelease = release
	engine := &faultInjectedPolicyReloadEngine{ready: true}
	watcher := newWatcherForTest(source, staticPolicyRevisionSource{}, engine, time.Hour, nil)

	require.NoError(t, watcher.Start(context.Background()))
	require.True(t, watcher.Status().Running)

	stopCtx, cancel := context.WithTimeout(context.Background(), 10*time.Millisecond)
	defer cancel()
	require.ErrorIs(t, watcher.Stop(stopCtx), context.DeadlineExceeded)

	close(release)
	require.NoError(t, watcher.Stop(context.Background()))
	require.NoError(t, watcher.Stop(context.Background()))
	require.False(t, watcher.Status().Running)
}

type faultInjectedPolicyReloadEngine struct {
	mu                  sync.Mutex
	appliedRevision     int64
	targetRevision      int64
	ready               bool
	reloadRevisions     []int64
	invalidateUserCount atomic.Int64
	invalidateAllCount  atomic.Int64
}

func (e *faultInjectedPolicyReloadEngine) ObserveTargetRevision(targetRevision int64) {
	e.mu.Lock()
	defer e.mu.Unlock()
	if targetRevision > e.targetRevision {
		e.targetRevision = targetRevision
	}
}

func (e *faultInjectedPolicyReloadEngine) ReloadToRevision(_ context.Context, targetRevision int64) (int64, error) {
	return e.reload(targetRevision, false)
}

func (e *faultInjectedPolicyReloadEngine) RefreshToRevision(_ context.Context, targetRevision int64) (int64, error) {
	return e.reload(targetRevision, true)
}

func (e *faultInjectedPolicyReloadEngine) reload(targetRevision int64, force bool) (int64, error) {
	e.mu.Lock()
	defer e.mu.Unlock()
	if targetRevision > e.targetRevision {
		e.targetRevision = targetRevision
	}
	advanced := targetRevision > e.appliedRevision
	if advanced {
		e.appliedRevision = targetRevision
	}
	if advanced || force {
		e.reloadRevisions = append(e.reloadRevisions, e.appliedRevision)
	}
	e.ready = true
	return e.appliedRevision, nil
}

func (e *faultInjectedPolicyReloadEngine) ProjectionStatus() permissionapplication.PolicyProjectionStatus {
	e.mu.Lock()
	defer e.mu.Unlock()
	return permissionapplication.PolicyProjectionStatus{Initialized: true, ReloadSucceeded: e.ready, AppliedRevision: e.appliedRevision, TargetRevision: e.targetRevision}
}

func (e *faultInjectedPolicyReloadEngine) AppliedRevision() int64 {
	e.mu.Lock()
	defer e.mu.Unlock()
	return e.appliedRevision
}

func (e *faultInjectedPolicyReloadEngine) InvalidateUserRole(uuid.UUID) { e.invalidateUserCount.Add(1) }
func (e *faultInjectedPolicyReloadEngine) InvalidateAllUserRoles()      { e.invalidateAllCount.Add(1) }

func (e *faultInjectedPolicyReloadEngine) reloads() []int64 {
	e.mu.Lock()
	defer e.mu.Unlock()
	return append([]int64(nil), e.reloadRevisions...)
}

type blockingPolicyReloadEngine struct {
	mu              sync.Mutex
	started         chan struct{}
	startOnce       sync.Once
	appliedRevision int64
	targetRevision  int64
}

func newBlockingPolicyReloadEngine() *blockingPolicyReloadEngine {
	return &blockingPolicyReloadEngine{started: make(chan struct{})}
}

func (e *blockingPolicyReloadEngine) ObserveTargetRevision(targetRevision int64) {
	e.mu.Lock()
	defer e.mu.Unlock()
	if targetRevision > e.targetRevision {
		e.targetRevision = targetRevision
	}
}

func (e *blockingPolicyReloadEngine) ReloadToRevision(ctx context.Context, targetRevision int64) (int64, error) {
	return e.reload(ctx, targetRevision)
}

func (e *blockingPolicyReloadEngine) RefreshToRevision(ctx context.Context, targetRevision int64) (int64, error) {
	return e.reload(ctx, targetRevision)
}

func (e *blockingPolicyReloadEngine) reload(ctx context.Context, targetRevision int64) (int64, error) {
	e.ObserveTargetRevision(targetRevision)
	e.startOnce.Do(func() { close(e.started) })
	<-ctx.Done()
	return e.AppliedRevision(), ctx.Err()
}

func (e *blockingPolicyReloadEngine) ProjectionStatus() permissionapplication.PolicyProjectionStatus {
	e.mu.Lock()
	defer e.mu.Unlock()
	return permissionapplication.PolicyProjectionStatus{Initialized: true, ReloadSucceeded: e.appliedRevision >= e.targetRevision, AppliedRevision: e.appliedRevision, TargetRevision: e.targetRevision}
}

func (e *blockingPolicyReloadEngine) AppliedRevision() int64 {
	e.mu.Lock()
	defer e.mu.Unlock()
	return e.appliedRevision
}

func (*blockingPolicyReloadEngine) InvalidateUserRole(uuid.UUID) {}
func (*blockingPolicyReloadEngine) InvalidateAllUserRoles()      {}

func requireEventuallyWatcherProjection(t *testing.T, engine *faultInjectedPolicyReloadEngine, revision int64) {
	t.Helper()
	require.Eventually(t, func() bool {
		status := engine.ProjectionStatus()
		return status.Ready() && status.AppliedRevision == revision && status.TargetRevision == revision
	}, time.Second, time.Millisecond, "projection status: %+v", engine.ProjectionStatus())
}

func mustPolicyPayload(t *testing.T, event permissionapplication.OutboxEvent) string {
	t.Helper()
	payload, err := encodePolicyRefreshMessage(newPolicyRefreshMessage(event, "instance-b"))
	require.NoError(t, err)
	return payload
}

type fakeMessageSource struct {
	mu          sync.Mutex
	status      redispubsub.Status
	messages    chan redispubsub.Message
	startErr    error
	stopRelease <-chan struct{}
	startCalls  atomic.Int64
	stopCalls   atomic.Int64
	closeOnce   sync.Once
}

func newFakeMessageSource(status redispubsub.Status) *fakeMessageSource {
	return &fakeMessageSource{status: status, messages: make(chan redispubsub.Message, 8)}
}

func (s *fakeMessageSource) Start(context.Context) error {
	s.startCalls.Add(1)
	if s.startErr != nil {
		return s.startErr
	}
	s.mu.Lock()
	s.status.Running = true
	s.status.State = redispubsub.StateConnected
	s.status.ErrorCategory = redispubsub.ErrorNone
	if s.status.LastConnectedAt.IsZero() {
		s.status.LastConnectedAt = time.Now()
	}
	s.mu.Unlock()
	return nil
}

func (s *fakeMessageSource) Stop(ctx context.Context) error {
	s.stopCalls.Add(1)
	if s.stopRelease != nil {
		select {
		case <-s.stopRelease:
		case <-ctx.Done():
			return ctx.Err()
		}
	}
	s.mu.Lock()
	s.status.Running = false
	s.status.State = redispubsub.StateStopped
	s.status.ErrorCategory = redispubsub.ErrorNone
	s.mu.Unlock()
	s.closeOnce.Do(func() { close(s.messages) })
	return nil
}

func (s *fakeMessageSource) Messages() <-chan redispubsub.Message {
	return s.messages
}

func (s *fakeMessageSource) Status() redispubsub.Status {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.status
}

func (s *fakeMessageSource) setStatus(status redispubsub.Status) {
	s.mu.Lock()
	s.status = status
	s.mu.Unlock()
}

type staticPolicyRevisionSource struct {
	revision int64
}

func (s staticPolicyRevisionSource) LatestPolicyRevision(context.Context) (int64, error) {
	return s.revision, nil
}

type atomicPolicyRevisionSource struct {
	revision atomic.Int64
}

func (s *atomicPolicyRevisionSource) Store(revision int64) {
	s.revision.Store(revision)
}

func (s *atomicPolicyRevisionSource) LatestPolicyRevision(context.Context) (int64, error) {
	return s.revision.Load(), nil
}

type watcherLifecycleContextKey struct{}

type capturingPolicyRevisionSource struct {
	value chan any
}

func (s *capturingPolicyRevisionSource) LatestPolicyRevision(ctx context.Context) (int64, error) {
	select {
	case s.value <- ctx.Value(watcherLifecycleContextKey{}):
	default:
	}
	return 0, nil
}

type failingPolicyRevisionSource struct {
	err error
}

func (s failingPolicyRevisionSource) LatestPolicyRevision(context.Context) (int64, error) {
	return 0, s.err
}

type blockingAfterFirstRevisionSource struct {
	calls   atomic.Int64
	blocked chan struct{}
	once    sync.Once
}

func newBlockingAfterFirstRevisionSource() *blockingAfterFirstRevisionSource {
	return &blockingAfterFirstRevisionSource{blocked: make(chan struct{})}
}

func (s *blockingAfterFirstRevisionSource) LatestPolicyRevision(ctx context.Context) (int64, error) {
	if s.calls.Add(1) == 1 {
		return 0, nil
	}
	s.once.Do(func() { close(s.blocked) })
	<-ctx.Done()
	return 0, ctx.Err()
}

type policyRevisionResult struct {
	revision int64
	err      error
}

type sequencePolicyRevisionSource struct {
	results []policyRevisionResult
	calls   atomic.Int64
}

func (s *sequencePolicyRevisionSource) LatestPolicyRevision(context.Context) (int64, error) {
	index := int(s.calls.Add(1) - 1)
	return s.results[index].revision, s.results[index].err
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
		Reason:         change.ReasonText(),
	}
	if change.Kind == permissionapplication.PolicyChangeKindUserRole {
		event.Kind = policyRefreshKindUserRoleChanged
		event.UserRoleRevision = int64Pointer(revision)
	} else {
		event.Kind = policyRefreshKindPolicyChanged
		event.PolicyRevision = int64Pointer(revision)
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

func int64Pointer(value int64) *int64 {
	return &value
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
