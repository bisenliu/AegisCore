package redis

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	rediscmd "github.com/redis/go-redis/v9"
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

	version, err := store.PublishPolicyChanged(context.Background(), "role_permission_added")
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
	if decoded.Version != 1 || decoded.InstanceID != "instance-a" || decoded.Reason != "role_permission_added" {
		t.Fatalf("message = %#v", decoded)
	}
}

func TestWatcherHandlePayloadReloadsOnlyNewerVersions(t *testing.T) {
	engine := &stubReloadEngine{}
	tracker := NewVersionTracker()
	watcher := NewWatcherForTest(nil, tracker, engine, nil, time.Second)
	payload, err := encodePolicyRefreshMessage(newPolicyRefreshMessage(3, "instance-b", "user_role_added"))
	if err != nil {
		t.Fatalf("encodePolicyRefreshMessage: %v", err)
	}

	watcher.HandlePayload(context.Background(), payload)
	watcher.HandlePayload(context.Background(), payload)

	if engine.calls != 1 {
		t.Fatalf("reload calls = %d, want 1", engine.calls)
	}
	if tracker.Applied() != 3 {
		t.Fatalf("applied version = %d, want 3", tracker.Applied())
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
	watcher := NewWatcherForTest(store, tracker, engine, nil, time.Second)

	watcher.CheckVersion(context.Background())

	if engine.calls != 1 {
		t.Fatalf("reload calls = %d, want 1", engine.calls)
	}
	if tracker.Applied() != 8 {
		t.Fatalf("applied version = %d, want 8", tracker.Applied())
	}
}

func TestWatcherReloadFailurePreservesAppliedVersion(t *testing.T) {
	engine := &stubReloadEngine{err: errors.New("reload failed")}
	tracker := NewVersionTracker()
	tracker.MarkApplied(2)
	watcher := NewWatcherForTest(nil, tracker, engine, nil, time.Second)
	payload, err := encodePolicyRefreshMessage(newPolicyRefreshMessage(5, "instance-b", "permission_updated"))
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
}

type stubReloadEngine struct {
	calls int
	err   error
}

func (e *stubReloadEngine) Reload(context.Context) error {
	e.calls++
	return e.err
}
