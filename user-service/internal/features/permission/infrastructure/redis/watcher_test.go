package redis

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/google/uuid"
	rediscmd "github.com/redis/go-redis/v9"

	permissionapplication "github.com/aegiscore/user-service/internal/features/permission/application"
)

func TestStorePublishPolicyChangedIncrementsVersionAndPublishes(t *testing.T) {
	redisServer := miniredis.RunT(t)
	client := rediscmd.NewClient(&rediscmd.Options{Addr: redisServer.Addr()})
	t.Cleanup(func() { _ = client.Close() })
	store := NewStoreWithInstance(client, "aegiscore-user-services", "instance-a", nil)
	pubsub := client.Subscribe(context.Background(), store.keys.PolicyChannel())
	t.Cleanup(func() { _ = pubsub.Close() })
	if _, err := pubsub.Receive(context.Background()); err != nil {
		t.Fatalf("Receive subscribe: %v", err)
	}
	change := permissionapplication.NewPolicyReloadChange("role_permission_added")

	version, err := store.PublishPolicyChanged(context.Background(), change)
	if err != nil {
		t.Fatalf("PublishPolicyChanged: %v", err)
	}
	if version != 1 {
		t.Fatalf("version = %d, want 1", version)
	}
	message, err := pubsub.ReceiveMessage(context.Background())
	if err != nil {
		t.Fatalf("ReceiveMessage: %v", err)
	}
	decoded, err := decodePolicyRefreshMessage(message.Payload)
	if err != nil {
		t.Fatalf("decodePolicyRefreshMessage: %v", err)
	}
	if decoded.Version != 1 || decoded.InstanceID != "instance-a" || decoded.Kind != permissionapplication.PolicyChangeKindPolicy || decoded.Reason != "role_permission_added" {
		t.Fatalf("message = %#v", decoded)
	}
}

func TestWatcherHandlePayloadReloadsPolicyOnlyForNewerVersions(t *testing.T) {
	engine := &stubReloadEngine{}
	tracker := NewVersionTracker()
	metrics := &watcherMetricsSpy{}
	watcher := NewWatcherForTestWithMetrics(nil, tracker, engine, nil, time.Second, metrics)
	payload, err := encodePolicyRefreshMessage(newPolicyRefreshMessage(3, "instance-b", permissionapplication.NewPolicyReloadChange("role_permission_added")))
	if err != nil {
		t.Fatalf("encodePolicyRefreshMessage: %v", err)
	}

	watcher.HandlePayload(context.Background(), payload)
	watcher.HandlePayload(context.Background(), payload)

	if engine.calls != 1 {
		t.Fatalf("reload calls = %d, want 1", engine.calls)
	}
	if engine.invalidateAll != 1 {
		t.Fatalf("invalidate all calls = %d, want 1", engine.invalidateAll)
	}
	if tracker.Applied() != 3 {
		t.Fatalf("applied version = %d, want 3", tracker.Applied())
	}
	if metrics.reloadSuccess[permissionapplication.MetricsSourceWatcherPubSub] != 1 {
		t.Fatalf("reload success metrics = %#v", metrics.reloadSuccess)
	}
}

func TestWatcherHandlePayloadInvalidatesUserRoleWithoutReload(t *testing.T) {
	userID := uuid.MustParse("018f0000-0000-7000-8000-000000000701")
	roleID := uuid.MustParse("018f0000-0000-7000-8000-000000000702")
	engine := &stubReloadEngine{}
	tracker := NewVersionTracker()
	watcher := NewWatcherForTestWithMetrics(nil, tracker, engine, nil, time.Second, &watcherMetricsSpy{})
	payload, err := encodePolicyRefreshMessage(newPolicyRefreshMessage(4, "instance-b", permissionapplication.NewUserRoleChange("user_role_added", userID, roleID)))
	if err != nil {
		t.Fatalf("encodePolicyRefreshMessage: %v", err)
	}

	watcher.HandlePayload(context.Background(), payload)

	if engine.calls != 0 {
		t.Fatalf("reload calls = %d, want 0", engine.calls)
	}
	if engine.invalidatedUser != userID || engine.invalidateAll != 0 {
		t.Fatalf("invalidation = user:%s all:%d", engine.invalidatedUser, engine.invalidateAll)
	}
	if tracker.Applied() != 4 {
		t.Fatalf("applied version = %d, want 4", tracker.Applied())
	}
}

func TestWatcherCheckVersionCompensatesMissedMessage(t *testing.T) {
	redisServer := miniredis.RunT(t)
	client := rediscmd.NewClient(&rediscmd.Options{Addr: redisServer.Addr()})
	t.Cleanup(func() { _ = client.Close() })
	store := NewStoreWithInstance(client, "aegiscore-user-services", "instance-a", nil)
	if err := client.Set(context.Background(), store.keys.PolicyVersionKey(), 8, 0).Err(); err != nil {
		t.Fatalf("Set version: %v", err)
	}
	engine := &stubReloadEngine{}
	tracker := NewVersionTracker()
	tracker.MarkApplied(4)
	metrics := &watcherMetricsSpy{}
	watcher := NewWatcherForTestWithMetrics(store, tracker, engine, nil, time.Second, metrics)

	watcher.CheckVersion(context.Background())

	if engine.calls != 1 {
		t.Fatalf("reload calls = %d, want 1", engine.calls)
	}
	if engine.invalidateAll != 1 {
		t.Fatalf("invalidate all calls = %d, want 1", engine.invalidateAll)
	}
	if tracker.Applied() != 8 {
		t.Fatalf("applied version = %d, want 8", tracker.Applied())
	}
	if metrics.versionMismatch[permissionapplication.MetricsSourceWatcherVersionCheck] != 1 {
		t.Fatalf("version mismatch metrics = %#v", metrics.versionMismatch)
	}
	if metrics.reloadSuccess[permissionapplication.MetricsSourceWatcherVersionCheck] != 1 {
		t.Fatalf("version check reload metrics = %#v", metrics.reloadSuccess)
	}
}

func TestWatcherReloadFailurePreservesAppliedVersion(t *testing.T) {
	engine := &stubReloadEngine{err: errors.New("reload failed")}
	tracker := NewVersionTracker()
	tracker.MarkApplied(2)
	metrics := &watcherMetricsSpy{}
	watcher := NewWatcherForTestWithMetrics(nil, tracker, engine, nil, time.Second, metrics)
	payload, err := encodePolicyRefreshMessage(newPolicyRefreshMessage(5, "instance-b", permissionapplication.NewPolicyReloadChange("permission_updated")))
	if err != nil {
		t.Fatalf("encodePolicyRefreshMessage: %v", err)
	}

	watcher.HandlePayload(context.Background(), payload)

	if engine.calls != 1 {
		t.Fatalf("reload calls = %d, want 1", engine.calls)
	}
	if tracker.Applied() != 2 {
		t.Fatalf("applied version = %d, want 2", tracker.Applied())
	}
	if metrics.reloadFailure[permissionapplication.MetricsSourceWatcherPubSub][permissionapplication.MetricsReasonReloadFailed] != 1 {
		t.Fatalf("reload failure metrics = %#v", metrics.reloadFailure)
	}
}

func TestWatcherRunningStatus(t *testing.T) {
	redisServer := miniredis.RunT(t)
	client := rediscmd.NewClient(&rediscmd.Options{Addr: redisServer.Addr()})
	t.Cleanup(func() { _ = client.Close() })
	store := NewStoreWithInstance(client, "aegiscore-user-services", "instance-a", nil)
	watcher := NewWatcherForTest(store, NewVersionTracker(), &stubReloadEngine{}, nil, time.Hour)

	if watcher.Running() {
		t.Fatal("Running = true before start, want false")
	}
	watcher.Start(context.Background())
	if !watcher.Running() {
		t.Fatal("Running = false after start, want true")
	}
	if err := watcher.Stop(context.Background()); err != nil {
		t.Fatalf("Stop: %v", err)
	}
	if watcher.Running() {
		t.Fatal("Running = true after stop, want false")
	}
	if watcher.LastError() != nil {
		t.Fatalf("LastError = %v, want nil for normal stop", watcher.LastError())
	}
}

func TestWatcherRecordsUnexpectedChannelClose(t *testing.T) {
	watcher := NewWatcherForTest(&closedChannelStore{}, NewVersionTracker(), &stubReloadEngine{}, nil, time.Hour)

	watcher.Start(context.Background())
	waitForWatcherStopped(t, watcher)

	if watcher.Running() {
		t.Fatal("Running = true after channel close, want false")
	}
	if watcher.LastError() == nil {
		t.Fatal("LastError = nil, want channel close error")
	}
}

type stubReloadEngine struct {
	calls           int
	err             error
	invalidatedUser uuid.UUID
	invalidateAll   int
}

func (e *stubReloadEngine) Reload(context.Context) error {
	e.calls++
	return e.err
}

func (e *stubReloadEngine) InvalidateUserRole(userID uuid.UUID) {
	e.invalidatedUser = userID
}

func (e *stubReloadEngine) InvalidateAllUserRoles() {
	e.invalidateAll++
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

type watcherMetricsSpy struct {
	permissionapplication.Metrics
	reloadSuccess   map[string]int
	reloadFailure   map[string]map[string]int
	versionMismatch map[string]int
}

func (m *watcherMetricsSpy) ensure() {
	if m.reloadSuccess == nil {
		m.reloadSuccess = map[string]int{}
	}
	if m.reloadFailure == nil {
		m.reloadFailure = map[string]map[string]int{}
	}
	if m.versionMismatch == nil {
		m.versionMismatch = map[string]int{}
	}
}

func (m *watcherMetricsSpy) WatcherReloadSucceeded(_ context.Context, source string) {
	m.ensure()
	m.reloadSuccess[source]++
}

func (m *watcherMetricsSpy) WatcherReloadFailed(_ context.Context, source string, reason string) {
	m.ensure()
	if m.reloadFailure[source] == nil {
		m.reloadFailure[source] = map[string]int{}
	}
	m.reloadFailure[source][reason]++
}

func (m *watcherMetricsSpy) WatcherVersionMismatch(_ context.Context, source string) {
	m.ensure()
	m.versionMismatch[source]++
}

func waitForWatcherStopped(t *testing.T, watcher *Watcher) {
	t.Helper()
	deadline := time.After(time.Second)
	ticker := time.NewTicker(10 * time.Millisecond)
	defer ticker.Stop()
	for {
		select {
		case <-deadline:
			t.Fatal("watcher did not stop")
		case <-ticker.C:
			if !watcher.Running() {
				return
			}
		}
	}
}
