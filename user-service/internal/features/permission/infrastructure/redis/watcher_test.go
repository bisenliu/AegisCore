package redis

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/google/uuid"
	rediscmd "github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/require"
	"go.uber.org/fx/fxtest"
	"go.uber.org/mock/gomock"

	permissionapplication "github.com/aegiscore/user-service/internal/features/permission/application"
)

func TestStorePublishPolicyChangedIncrementsVersionAndPublishes(t *testing.T) {
	redisServer := miniredis.RunT(t)
	client := rediscmd.NewClient(&rediscmd.Options{Addr: redisServer.Addr()})
	t.Cleanup(func() { _ = client.Close() })
	store := newStore(client, MustKeyCatalog("aegiscore-user-services"), "instance-a", nil)
	pubsub := client.Subscribe(context.Background(), store.keys.PolicyChannel())
	t.Cleanup(func() { _ = pubsub.Close() })
	_, err := pubsub.Receive(context.Background())
	require.NoError(t, err)
	change := permissionapplication.NewPolicyReloadChange("role_permission_added")

	version, err := store.PublishPolicyChanged(context.Background(), change)
	require.NoError(t, err)
	require.Equal(t, int64(1), version)
	message, err := pubsub.ReceiveMessage(context.Background())
	require.NoError(t, err)
	decoded, err := decodePolicyRefreshMessage(message.Payload)
	require.NoError(t, err)
	require.Equal(t, int64(1), decoded.Version)
	require.Equal(t, "instance-a", decoded.InstanceID)
	require.Equal(t, permissionapplication.PolicyChangeKindPolicy, decoded.Kind)
	require.Equal(t, "role_permission_added", decoded.Reason)
}

func TestWatcherHandlePayloadReloadsPolicyOnlyForNewerVersions(t *testing.T) {
	ctrl := gomock.NewController(t)
	engine := NewMockPolicyReloadEngine(ctrl)
	tracker := NewVersionTracker()
	metrics := NewMockMetrics(ctrl)
	watcher := newWatcherWithMetrics(nil, tracker, engine, nil, time.Second, metrics)
	payload, err := encodePolicyRefreshMessage(newPolicyRefreshMessage(3, "instance-b", permissionapplication.NewPolicyReloadChange("role_permission_added")))
	require.NoError(t, err)

	gomock.InOrder(
		engine.EXPECT().Reload(gomock.Any()).Return(nil),
		engine.EXPECT().InvalidateAllUserRoles(),
		metrics.EXPECT().WatcherReloadSucceeded(gomock.Any(), permissionapplication.MetricsSourceWatcherPubSub),
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
	payload, err := encodePolicyRefreshMessage(newPolicyRefreshMessage(4, "instance-b", permissionapplication.NewUserRoleChange("user_role_added", userID, roleID)))
	require.NoError(t, err)

	engine.EXPECT().InvalidateUserRole(userID)

	watcher.HandlePayload(context.Background(), payload)

	require.Equal(t, int64(4), tracker.Applied())
}

func TestWatcherCheckVersionCompensatesMissedMessage(t *testing.T) {
	redisServer := miniredis.RunT(t)
	client := rediscmd.NewClient(&rediscmd.Options{Addr: redisServer.Addr()})
	t.Cleanup(func() { _ = client.Close() })
	store := newStore(client, MustKeyCatalog("aegiscore-user-services"), "instance-a", nil)
	require.NoError(t, client.Set(context.Background(), store.keys.PolicyVersionKey(), 8, 0).Err())
	ctrl := gomock.NewController(t)
	engine := NewMockPolicyReloadEngine(ctrl)
	tracker := NewVersionTracker()
	tracker.MarkApplied(4)
	metrics := NewMockMetrics(ctrl)
	watcher := newWatcherWithMetrics(store, tracker, engine, nil, time.Second, metrics)

	gomock.InOrder(
		metrics.EXPECT().WatcherVersionMismatch(gomock.Any(), permissionapplication.MetricsSourceWatcherVersionCheck),
		engine.EXPECT().Reload(gomock.Any()).Return(nil),
		engine.EXPECT().InvalidateAllUserRoles(),
		metrics.EXPECT().WatcherReloadSucceeded(gomock.Any(), permissionapplication.MetricsSourceWatcherVersionCheck),
	)

	watcher.CheckVersion(context.Background())

	require.Equal(t, int64(8), tracker.Applied())
}

func TestWatcherReloadFailurePreservesAppliedVersion(t *testing.T) {
	reloadErr := errors.New("reload failed")
	ctrl := gomock.NewController(t)
	engine := NewMockPolicyReloadEngine(ctrl)
	tracker := NewVersionTracker()
	tracker.MarkApplied(2)
	metrics := NewMockMetrics(ctrl)
	watcher := newWatcherWithMetrics(nil, tracker, engine, nil, time.Second, metrics)
	payload, err := encodePolicyRefreshMessage(newPolicyRefreshMessage(5, "instance-b", permissionapplication.NewPolicyReloadChange("permission_updated")))
	require.NoError(t, err)

	gomock.InOrder(
		engine.EXPECT().Reload(gomock.Any()).Return(reloadErr),
		metrics.EXPECT().WatcherReloadFailed(gomock.Any(), permissionapplication.MetricsSourceWatcherPubSub, permissionapplication.MetricsReasonReloadFailed),
	)

	watcher.HandlePayload(context.Background(), payload)

	require.Equal(t, int64(2), tracker.Applied())
}

func TestWatcherRunningStatus(t *testing.T) {
	redisServer := miniredis.RunT(t)
	client := rediscmd.NewClient(&rediscmd.Options{Addr: redisServer.Addr()})
	t.Cleanup(func() { _ = client.Close() })
	store := newStore(client, MustKeyCatalog("aegiscore-user-services"), "instance-a", nil)
	engine := NewMockPolicyReloadEngine(gomock.NewController(t))
	watcher := newWatcherWithMetrics(store, NewVersionTracker(), engine, nil, time.Hour, nil)

	require.False(t, watcher.Running())
	watcher.Start()
	require.True(t, watcher.Running())
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

func TestWatcherLifecycleStartContextDoesNotControlBackgroundLoop(t *testing.T) {
	redisServer := miniredis.RunT(t)
	client := rediscmd.NewClient(&rediscmd.Options{Addr: redisServer.Addr()})
	t.Cleanup(func() { _ = client.Close() })
	store := newStore(client, MustKeyCatalog("aegiscore-user-services"), "instance-a", nil)
	engine := NewMockPolicyReloadEngine(gomock.NewController(t))
	lifecycle := fxtest.NewLifecycle(t)
	watcher := NewWatcher(WatcherParams{
		Lifecycle: lifecycle,
		Store:     store,
		Tracker:   NewVersionTracker(),
		Engine:    engine,
	})
	t.Cleanup(func() { _ = watcher.Stop(context.Background()) })

	startCtx, cancelStart := context.WithCancel(context.Background())
	require.NoError(t, lifecycle.Start(startCtx))
	cancelStart()

	requireWatcherRunningFor(t, watcher, 100*time.Millisecond)

	require.NoError(t, lifecycle.Stop(context.Background()))
	require.False(t, watcher.Running())
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

func waitForWatcherStopped(t *testing.T, watcher *Watcher) {
	t.Helper()
	require.Eventually(t, func() bool {
		return !watcher.Running()
	}, time.Second, 10*time.Millisecond)
}

func requireWatcherRunningFor(t *testing.T, watcher *Watcher, duration time.Duration) {
	t.Helper()
	require.Never(t, func() bool {
		return !watcher.Running()
	}, duration, 10*time.Millisecond)
}
