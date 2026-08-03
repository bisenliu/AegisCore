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

func TestWatcherHandlePayloadReloadsPolicyForEveryValidEvent(t *testing.T) {
	ctrl := gomock.NewController(t)
	engine := NewMockPolicyReloadEngine(ctrl)
	var applied atomic.Int64
	engine.EXPECT().AppliedRevision().DoAndReturn(func() int64 { return applied.Load() }).AnyTimes()
	engine.EXPECT().RefreshToRevision(gomock.Any(), int64(3)).DoAndReturn(func(context.Context, int64) (int64, error) {
		applied.Store(3)
		return 3, nil
	}).Times(2)
	engine.EXPECT().ProjectionStatus().Return(permissionapplication.PolicyProjectionStatus{Initialized: true, ReloadSucceeded: true, AppliedRevision: 3, TargetRevision: 3}).Times(4)
	engine.EXPECT().InvalidateAllUserRoles().Times(2)
	metrics := NewMockMetrics(ctrl)
	watcher := newWatcherWithMetrics(nil, staticPolicyRevisionSource{revision: 3}, engine, nil, time.Second, metrics)
	payload, err := encodePolicyRefreshMessage(newPolicyRefreshMessage(testPolicyPublicationEvent(3, permissionapplication.NewPolicyReloadChange("role_permission_added")), "instance-b"))
	require.NoError(t, err)

	gomock.InOrder(
		metrics.EXPECT().PolicyReloadLagObserved(gomock.Any(), int64(3)),
		metrics.EXPECT().WatcherVersionMismatch(gomock.Any(), permissionapplication.MetricsSourceWatcherPubSub, permissionapplication.MetricsReasonRevisionMismatch),
		metrics.EXPECT().WatcherReloadSucceeded(gomock.Any(), permissionapplication.MetricsSourceWatcherPubSub),
		metrics.EXPECT().PolicyReloadLagObserved(gomock.Any(), int64(0)),
		metrics.EXPECT().PolicyReloadLagObserved(gomock.Any(), int64(0)),
		metrics.EXPECT().WatcherReloadSucceeded(gomock.Any(), permissionapplication.MetricsSourceWatcherPubSub),
		metrics.EXPECT().PolicyReloadLagObserved(gomock.Any(), int64(0)),
	)

	watcher.HandlePayload(context.Background(), payload)
	watcher.HandlePayload(context.Background(), payload)

	require.Equal(t, int64(3), applied.Load())
}

func TestWatcherHandlePayloadRevisionGapInvalidatesAllUserRoleCaches(t *testing.T) {
	userID := uuid.MustParse("018f0000-0000-7000-8000-000000000701")
	roleID := uuid.MustParse("018f0000-0000-7000-8000-000000000702")
	ctrl := gomock.NewController(t)
	engine := NewMockPolicyReloadEngine(ctrl)
	engine.EXPECT().AppliedRevision().Return(int64(0)).AnyTimes()
	engine.EXPECT().ReloadToRevision(gomock.Any(), int64(4)).Return(int64(4), nil)
	engine.EXPECT().InvalidateAllUserRoles()
	engine.EXPECT().ProjectionStatus().Return(permissionapplication.PolicyProjectionStatus{Initialized: true, ReloadSucceeded: true, AppliedRevision: 4, TargetRevision: 4}).Times(2)
	metrics := NewMockMetrics(ctrl)
	watcher := newWatcherWithMetrics(nil, staticPolicyRevisionSource{revision: 4}, engine, nil, time.Second, metrics)
	payload, err := encodePolicyRefreshMessage(newPolicyRefreshMessage(testPolicyPublicationEvent(4, permissionapplication.NewUserRoleChange("user_role_added", userID, roleID)), "instance-b"))
	require.NoError(t, err)

	gomock.InOrder(
		metrics.EXPECT().PolicyReloadLagObserved(gomock.Any(), int64(4)),
		metrics.EXPECT().WatcherVersionMismatch(gomock.Any(), permissionapplication.MetricsSourceWatcherPubSub, permissionapplication.MetricsReasonRevisionMismatch),
		metrics.EXPECT().WatcherReloadSucceeded(gomock.Any(), permissionapplication.MetricsSourceWatcherPubSub),
		metrics.EXPECT().PolicyReloadLagObserved(gomock.Any(), int64(0)),
	)

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
	engine.EXPECT().ProjectionStatus().Return(permissionapplication.PolicyProjectionStatus{Initialized: true, ReloadSucceeded: true, AppliedRevision: 9, TargetRevision: 9})
	metrics := NewMockMetrics(ctrl)
	watcher := newWatcherWithMetrics(nil, staticPolicyRevisionSource{revision: 9}, engine, nil, time.Second, metrics)
	payload, err := encodePolicyRefreshMessage(newPolicyRefreshMessage(testPolicyPublicationEvent(4, permissionapplication.NewUserRoleChange("user_role_removed", userID, roleID)), "instance-b"))
	require.NoError(t, err)

	metrics.EXPECT().PolicyReloadLagObserved(gomock.Any(), int64(0))

	watcher.HandlePayload(context.Background(), payload)

	require.Equal(t, int64(9), engine.AppliedRevision())
}

func TestWatcherHandlePayloadReloadsOutOfOrderPolicyEventWithoutMovingAppliedRevision(t *testing.T) {
	ctrl := gomock.NewController(t)
	engine := NewMockPolicyReloadEngine(ctrl)
	engine.EXPECT().AppliedRevision().Return(int64(9)).AnyTimes()
	engine.EXPECT().ProjectionStatus().Return(permissionapplication.PolicyProjectionStatus{Initialized: true, ReloadSucceeded: true, AppliedRevision: 9, TargetRevision: 9}).Times(2)
	engine.EXPECT().RefreshToRevision(gomock.Any(), int64(9)).Return(int64(9), nil)
	engine.EXPECT().InvalidateAllUserRoles()
	metrics := NewMockMetrics(ctrl)
	watcher := newWatcherWithMetrics(nil, staticPolicyRevisionSource{revision: 9}, engine, nil, time.Second, metrics)
	payload, err := encodePolicyRefreshMessage(newPolicyRefreshMessage(testPolicyPublicationEvent(4, permissionapplication.NewPolicyReloadChange("role_updated")), "instance-b"))
	require.NoError(t, err)

	gomock.InOrder(
		metrics.EXPECT().PolicyReloadLagObserved(gomock.Any(), int64(0)),
		metrics.EXPECT().WatcherReloadSucceeded(gomock.Any(), permissionapplication.MetricsSourceWatcherPubSub),
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
	redisServer := miniredis.RunT(t)
	client := rediscmd.NewClient(&rediscmd.Options{Addr: redisServer.Addr()})
	t.Cleanup(func() { _ = client.Close() })
	store := newStore(client, mustKeyCatalog("aegiscore-user-service"), "instance-a", nil)
	require.NoError(t, client.Set(context.Background(), store.keys.PolicyVersionKey(), 1, 0).Err())
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
	watcher := newWatcherWithMetrics(store, staticPolicyRevisionSource{revision: 8}, engine, nil, time.Second, metrics)

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
	watcher := newWatcherWithMetrics(store, staticPolicyRevisionSource{revision: databaseRevision}, engine, nil, time.Second, nil)
	watcher.CheckVersion(context.Background())

	requireEventuallyWatcherProjection(t, engine, databaseRevision)
	require.Equal(t, int64(1), engine.invalidateAllCount.Load())
}

func TestWatcherCheckVersionSkipsWhenProjectionIsAuthoritativelyReady(t *testing.T) {
	redisServer := miniredis.RunT(t)
	client := rediscmd.NewClient(&rediscmd.Options{Addr: redisServer.Addr()})
	t.Cleanup(func() { _ = client.Close() })
	store := newStore(client, mustKeyCatalog("aegiscore-user-service"), "instance-a", nil)
	require.NoError(t, client.Set(context.Background(), store.keys.PolicyVersionKey(), 4, 0).Err())
	engine := NewMockPolicyReloadEngine(gomock.NewController(t))
	engine.EXPECT().AppliedRevision().Return(int64(9))
	engine.EXPECT().ProjectionStatus().Return(permissionapplication.PolicyProjectionStatus{Initialized: true, ReloadSucceeded: true, AppliedRevision: 9, TargetRevision: 9})
	metrics := NewMockMetrics(gomock.NewController(t))
	metrics.EXPECT().PolicyReloadLagObserved(gomock.Any(), int64(0))
	watcher := newWatcherWithMetrics(store, staticPolicyRevisionSource{revision: 4}, engine, nil, time.Second, metrics)

	watcher.CheckVersion(context.Background())

}

func TestWatcherCheckVersionRetriesAfterPriorFailureAtEqualRevision(t *testing.T) {
	redisServer := miniredis.RunT(t)
	client := rediscmd.NewClient(&rediscmd.Options{Addr: redisServer.Addr()})
	t.Cleanup(func() { _ = client.Close() })
	store := newStore(client, mustKeyCatalog("aegiscore-user-service"), "instance-a", nil)
	require.NoError(t, client.Set(context.Background(), store.keys.PolicyVersionKey(), 8, 0).Err())
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
	watcher := newWatcherWithMetrics(store, staticPolicyRevisionSource{revision: 8}, engine, nil, time.Second, metrics)

	watcher.CheckVersion(context.Background())
}

func TestWatcherFaultInjectionReplayAddRemoveReplaceEventsKeepsIdempotentProjection(t *testing.T) {
	userID := uuid.MustParse("018f0000-0000-7000-8000-000000000711")
	roleID := uuid.MustParse("018f0000-0000-7000-8000-000000000712")
	engine := &faultInjectedPolicyReloadEngine{appliedRevision: 1, targetRevision: 1, ready: true}
	revisions := &sequencePolicyRevisionSource{results: []policyRevisionResult{
		{revision: 4}, {revision: 4}, {revision: 4}, {revision: 4},
	}}
	watcher := newWatcherWithMetrics(nil, revisions, engine, nil, time.Second, nil)
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
	require.Equal(t, int64(1), engine.invalidateUserCount.Load())
	require.Equal(t, int64(3), engine.invalidateAllCount.Load())
	require.Equal(t, []int64{4, 4, 4}, engine.reloads())
}

func TestWatcherReloadFailurePreservesAppliedVersion(t *testing.T) {
	reloadErr := errors.New("reload failed")
	ctrl := gomock.NewController(t)
	engine := NewMockPolicyReloadEngine(ctrl)
	engine.EXPECT().AppliedRevision().Return(int64(2)).AnyTimes()
	engine.EXPECT().ProjectionStatus().Return(permissionapplication.PolicyProjectionStatus{Initialized: true, AppliedRevision: 2, TargetRevision: 5, LastError: reloadErr})
	engine.EXPECT().RefreshToRevision(gomock.Any(), int64(5)).Return(int64(2), reloadErr)
	metrics := NewMockMetrics(ctrl)
	watcher := newWatcherWithMetrics(nil, staticPolicyRevisionSource{revision: 5}, engine, nil, time.Second, metrics)
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
	watcher := newWatcherWithMetrics(&failingVersionStore{}, failingPolicyRevisionSource{err: errors.New("database unavailable")}, engine, nil, time.Second, metrics)

	engine.EXPECT().AppliedRevision().Return(int64(2))
	metrics.EXPECT().WatcherCheckFailed(gomock.Any(), permissionapplication.MetricsSourceWatcherRevisionCheck, permissionapplication.MetricsReasonRevisionStoreUnavailable)

	watcher.CheckVersion(context.Background())
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
	watcher := newWatcherWithMetrics(&failingVersionStore{}, revisions, engine, nil, time.Second, metrics)

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
	watcher.CheckVersion(context.Background())
	watcher.CheckVersion(context.Background())
	require.Equal(t, int64(7), applied.Load())
}

func TestWatcherHandlePayloadRevisionSourceFailureDoesNotUseHint(t *testing.T) {
	ctrl := gomock.NewController(t)
	engine := NewMockPolicyReloadEngine(ctrl)
	engine.EXPECT().AppliedRevision().Return(int64(2)).Times(2)
	engine.EXPECT().InvalidateAllUserRoles()
	metrics := NewMockMetrics(ctrl)
	watcher := newWatcherWithMetrics(nil, failingPolicyRevisionSource{err: errors.New("database unavailable")}, engine, nil, time.Second, metrics)
	payload, err := encodePolicyRefreshMessage(newPolicyRefreshMessage(testPolicyPublicationEvent(99, permissionapplication.NewPolicyReloadChange("role_updated")), "instance-b"))
	require.NoError(t, err)
	metrics.EXPECT().WatcherCheckFailed(gomock.Any(), permissionapplication.MetricsSourceWatcherPubSub, permissionapplication.MetricsReasonRevisionStoreUnavailable)

	watcher.HandlePayload(context.Background(), payload)
}

func TestWatcherRunningStatus(t *testing.T) {
	redisServer := miniredis.RunT(t)
	client := rediscmd.NewClient(&rediscmd.Options{Addr: redisServer.Addr()})
	t.Cleanup(func() { _ = client.Close() })
	store := newStore(client, mustKeyCatalog("aegiscore-user-service"), "instance-a", nil)
	engine := NewMockPolicyReloadEngine(gomock.NewController(t))
	watcher := newWatcherWithMetrics(store, staticPolicyRevisionSource{}, engine, nil, time.Hour, nil)

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
	watcher := newWatcherWithMetrics(&closedChannelStore{}, staticPolicyRevisionSource{}, engine, nil, time.Hour, nil)

	watcher.Start()
	waitForWatcherStopped(t, watcher)

	require.False(t, watcher.Running())
	require.Error(t, watcher.LastError())
}

func TestNewWatcherDoesNotStartBackgroundLoop(t *testing.T) {
	engine := NewMockPolicyReloadEngine(gomock.NewController(t))
	watcher := NewWatcher(WatcherParams{
		Engine:         engine,
		RevisionSource: staticPolicyRevisionSource{},
	})

	require.False(t, watcher.Running())
}

func TestWatcherStopHonorsDeadlineAndCanBeRepeated(t *testing.T) {
	release := make(chan struct{})
	closed := &atomic.Int64{}
	subscriber := blockingPolicySubscriber{release: release, closed: closed}
	store := &countingSubscriptionStore{subscriber: subscriber}
	engine := NewMockPolicyReloadEngine(gomock.NewController(t))
	watcher := newWatcherWithMetrics(store, staticPolicyRevisionSource{}, engine, nil, time.Hour, nil)

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

type countingSubscriptionStore struct {
	subscriber    policySubscriber
	subscriptions atomic.Int64
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

func (s *failingVersionStore) Subscribe(context.Context) policySubscriber {
	return closedPolicySubscriber{}
}

type staticPolicyRevisionSource struct {
	revision int64
}

func (s staticPolicyRevisionSource) LatestPolicyRevision(context.Context) (int64, error) {
	return s.revision, nil
}

type failingPolicyRevisionSource struct {
	err error
}

func (s failingPolicyRevisionSource) LatestPolicyRevision(context.Context) (int64, error) {
	return 0, s.err
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
