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
	metrics := &policyMetricsSpy{}
	coordinator := NewPolicyRefreshCoordinator(engine, publisher, tracker, nil, metrics)

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
	if metrics.reloadSuccess[MetricsSourceLocalChange] != 1 || metrics.publishSuccess != 1 {
		t.Fatalf("metrics = %#v, want reload and publish success", metrics)
	}
}

func TestPolicyRefreshCoordinatorSkipsPublishWhenReloadFails(t *testing.T) {
	reloadErr := errors.New("reload failed")
	engine := &stubPolicyEngine{err: reloadErr}
	publisher := &stubPolicyPublisher{version: 12}
	tracker := &stubPolicyTracker{}
	metrics := &policyMetricsSpy{}
	coordinator := NewPolicyRefreshCoordinator(engine, publisher, tracker, nil, metrics)

	coordinator.NotifyPolicyChanged(context.Background(), "permission_updated")

	if publisher.calls != 0 {
		t.Fatalf("publisher calls = %d, want 0", publisher.calls)
	}
	if tracker.applied != 0 {
		t.Fatalf("applied version = %d, want 0", tracker.applied)
	}
	if metrics.reloadFailure[MetricsSourceLocalChange][MetricsReasonReloadFailed] != 1 {
		t.Fatalf("reload failure metrics = %#v", metrics.reloadFailure)
	}
}

func TestPolicyRefreshCoordinatorDoesNotTrackWhenPublishFails(t *testing.T) {
	publishErr := errors.New("publish failed")
	engine := &stubPolicyEngine{}
	publisher := &stubPolicyPublisher{err: publishErr}
	tracker := &stubPolicyTracker{}
	metrics := &policyMetricsSpy{}
	coordinator := NewPolicyRefreshCoordinator(engine, publisher, tracker, nil, metrics)

	coordinator.NotifyPolicyChanged(context.Background(), "permission_active_changed")

	if engine.calls != 1 || publisher.calls != 1 {
		t.Fatalf("calls = reload:%d publish:%d", engine.calls, publisher.calls)
	}
	if tracker.applied != 0 {
		t.Fatalf("applied version = %d, want 0", tracker.applied)
	}
	if metrics.publishFailure[MetricsReasonPublishFailed] != 1 {
		t.Fatalf("publish failure metrics = %#v", metrics.publishFailure)
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

type policyMetricsSpy struct {
	reloadSuccess  map[string]int
	reloadFailure  map[string]map[string]int
	publishSuccess int
	publishFailure map[string]int
}

func (m *policyMetricsSpy) ensure() {
	if m.reloadSuccess == nil {
		m.reloadSuccess = map[string]int{}
	}
	if m.reloadFailure == nil {
		m.reloadFailure = map[string]map[string]int{}
	}
	if m.publishFailure == nil {
		m.publishFailure = map[string]int{}
	}
}

func (m *policyMetricsSpy) PolicyReloadSucceeded(_ context.Context, source string) {
	m.ensure()
	m.reloadSuccess[source]++
}

func (m *policyMetricsSpy) PolicyReloadFailed(_ context.Context, source string, reason string) {
	m.ensure()
	if m.reloadFailure[source] == nil {
		m.reloadFailure[source] = map[string]int{}
	}
	m.reloadFailure[source][reason]++
}

func (m *policyMetricsSpy) PolicyPublishSucceeded(context.Context) {
	m.publishSuccess++
}

func (m *policyMetricsSpy) PolicyPublishFailed(_ context.Context, reason string) {
	m.ensure()
	m.publishFailure[reason]++
}

func (m *policyMetricsSpy) WatcherCheckFailed(context.Context, string)          {}
func (m *policyMetricsSpy) WatcherReloadSucceeded(context.Context, string)      {}
func (m *policyMetricsSpy) WatcherReloadFailed(context.Context, string, string) {}
func (m *policyMetricsSpy) WatcherVersionMismatch(context.Context, string)      {}
func (m *policyMetricsSpy) RouteDiffObserved(context.Context, int, int)         {}
