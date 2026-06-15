package application

import (
	"context"
	"errors"
	"testing"
)

func TestPolicyRefreshCoordinatorReloadsPublishesAndTracksVersion(t *testing.T) {
	engine := &stubPolicyEngine{}
	publisher := &stubPolicyPublisher{version: 12}
	tracker := &stubPolicyTracker{}
	coordinator := NewPolicyRefreshCoordinator(engine, publisher, tracker, nil)

	coordinator.NotifyPolicyChanged(context.Background(), "role_permission_added")

	if engine.calls != 1 {
		t.Fatalf("reload calls = %d, want 1", engine.calls)
	}
	if publisher.reason != "role_permission_added" || publisher.calls != 1 {
		t.Fatalf("publisher calls = %d reason = %q", publisher.calls, publisher.reason)
	}
	if tracker.applied != 12 {
		t.Fatalf("applied version = %d, want 12", tracker.applied)
	}
}

func TestPolicyRefreshCoordinatorSkipsPublishWhenReloadFails(t *testing.T) {
	reloadErr := errors.New("reload failed")
	engine := &stubPolicyEngine{err: reloadErr}
	publisher := &stubPolicyPublisher{version: 12}
	tracker := &stubPolicyTracker{}
	coordinator := NewPolicyRefreshCoordinator(engine, publisher, tracker, nil)

	coordinator.NotifyPolicyChanged(context.Background(), "permission_updated")

	if publisher.calls != 0 {
		t.Fatalf("publisher calls = %d, want 0", publisher.calls)
	}
	if tracker.applied != 0 {
		t.Fatalf("applied version = %d, want 0", tracker.applied)
	}
}

func TestPolicyRefreshCoordinatorDoesNotTrackWhenPublishFails(t *testing.T) {
	publishErr := errors.New("publish failed")
	engine := &stubPolicyEngine{}
	publisher := &stubPolicyPublisher{err: publishErr}
	tracker := &stubPolicyTracker{}
	coordinator := NewPolicyRefreshCoordinator(engine, publisher, tracker, nil)

	coordinator.NotifyPolicyChanged(context.Background(), "permission_active_changed")

	if engine.calls != 1 || publisher.calls != 1 {
		t.Fatalf("calls = reload:%d publish:%d", engine.calls, publisher.calls)
	}
	if tracker.applied != 0 {
		t.Fatalf("applied version = %d, want 0", tracker.applied)
	}
}

type stubPolicyEngine struct {
	calls int
	err   error
}

func (e *stubPolicyEngine) Reload(context.Context) error {
	e.calls++
	return e.err
}

type stubPolicyPublisher struct {
	calls   int
	reason  string
	version int64
	err     error
}

func (p *stubPolicyPublisher) PublishPolicyChanged(_ context.Context, reason string) (int64, error) {
	p.calls++
	p.reason = reason
	return p.version, p.err
}

type stubPolicyTracker struct {
	applied int64
}

func (t *stubPolicyTracker) MarkApplied(version int64) { t.applied = version }

func (t *stubPolicyTracker) Applied() int64 { return t.applied }
