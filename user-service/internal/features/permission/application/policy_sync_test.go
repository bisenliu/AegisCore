package application

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"
	"go.uber.org/mock/gomock"
)

func TestPolicyRefreshCoordinatorReloadsPublishesAndTracksVersion(t *testing.T) {
	engine := NewMockPolicyReloadEngine(gomock.NewController(t))
	engine.EXPECT().Reload(gomock.Any()).Return(nil)
	engine.EXPECT().InvalidateAllUserRoles()

	publisher := NewMockPolicyVersionPublisher(gomock.NewController(t))
	var published PolicyChange
	publisher.EXPECT().PublishPolicyChanged(gomock.Any(), gomock.Any()).DoAndReturn(func(_ context.Context, change PolicyChange) (int64, error) {
		published = change
		return int64(12), nil
	})

	tracker := NewMockPolicyVersionTracker(gomock.NewController(t))
	tracker.EXPECT().MarkApplied(int64(12))

	metrics := NewMockMetrics(gomock.NewController(t))
	metrics.EXPECT().PolicyReloadSucceeded(gomock.Any(), MetricsSourceLocalChange)
	metrics.EXPECT().PolicyPublishSucceeded(gomock.Any())

	coordinator := NewPolicyRefreshCoordinator(engine, publisher, tracker, nil, metrics)

	if err := coordinator.NotifyPolicyChanged(context.Background(), NewPolicyReloadChange("role_permission_added")); err != nil {
		t.Fatalf("NotifyPolicyChanged: %v", err)
	}
	if published.Reason != "role_permission_added" {
		t.Fatalf("published reason = %q", published.Reason)
	}
}

func TestPolicyRefreshCoordinatorSkipsPublishWhenReloadFails(t *testing.T) {
	reloadErr := errors.New("reload failed")
	engine := NewMockPolicyReloadEngine(gomock.NewController(t))
	engine.EXPECT().Reload(gomock.Any()).Return(reloadErr)

	publisher := NewMockPolicyVersionPublisher(gomock.NewController(t))
	tracker := NewMockPolicyVersionTracker(gomock.NewController(t))
	metrics := NewMockMetrics(gomock.NewController(t))
	metrics.EXPECT().PolicyReloadFailed(gomock.Any(), MetricsSourceLocalChange, MetricsReasonReloadFailed)

	coordinator := NewPolicyRefreshCoordinator(engine, publisher, tracker, nil, metrics)

	err := coordinator.NotifyPolicyChanged(context.Background(), NewPolicyReloadChange("permission_updated"))
	if !errors.Is(err, reloadErr) {
		t.Fatalf("err = %v, want reloadErr", err)
	}
}

func TestPolicyRefreshCoordinatorDoesNotTrackWhenPublishFails(t *testing.T) {
	publishErr := errors.New("publish failed")
	engine := NewMockPolicyReloadEngine(gomock.NewController(t))
	engine.EXPECT().Reload(gomock.Any()).Return(nil)
	engine.EXPECT().InvalidateAllUserRoles()

	publisher := NewMockPolicyVersionPublisher(gomock.NewController(t))
	publisher.EXPECT().PublishPolicyChanged(gomock.Any(), gomock.Any()).Return(int64(0), publishErr)

	tracker := NewMockPolicyVersionTracker(gomock.NewController(t))
	metrics := NewMockMetrics(gomock.NewController(t))
	metrics.EXPECT().PolicyReloadSucceeded(gomock.Any(), MetricsSourceLocalChange)
	metrics.EXPECT().PolicyPublishFailed(gomock.Any(), MetricsReasonPublishFailed)

	coordinator := NewPolicyRefreshCoordinator(engine, publisher, tracker, nil, metrics)

	err := coordinator.NotifyPolicyChanged(context.Background(), NewPolicyReloadChange("permission_active_changed"))
	if !errors.Is(err, publishErr) {
		t.Fatalf("err = %v, want publishErr", err)
	}
}

func TestPolicyRefreshCoordinatorUserRoleChangeInvalidatesWithoutReload(t *testing.T) {
	userID := uuid.MustParse("018f0000-0000-7000-8000-000000000901")
	roleID := uuid.MustParse("018f0000-0000-7000-8000-000000000902")
	engine := NewMockPolicyReloadEngine(gomock.NewController(t))
	engine.EXPECT().InvalidateUserRole(userID)

	publisher := NewMockPolicyVersionPublisher(gomock.NewController(t))
	var published PolicyChange
	publisher.EXPECT().PublishPolicyChanged(gomock.Any(), gomock.Any()).DoAndReturn(func(_ context.Context, change PolicyChange) (int64, error) {
		published = change
		return int64(7), nil
	})

	tracker := NewMockPolicyVersionTracker(gomock.NewController(t))
	tracker.EXPECT().MarkApplied(int64(7))

	metrics := NewMockMetrics(gomock.NewController(t))
	metrics.EXPECT().PolicyPublishSucceeded(gomock.Any())

	coordinator := NewPolicyRefreshCoordinator(engine, publisher, tracker, nil, metrics)

	if err := coordinator.NotifyPolicyChanged(context.Background(), NewUserRoleChange("user_role_added", userID, roleID)); err != nil {
		t.Fatalf("NotifyPolicyChanged: %v", err)
	}
	if published.Kind != PolicyChangeKindUserRole || published.UserID != userID || published.RoleID != roleID {
		t.Fatalf("published change = %#v", published)
	}
}
