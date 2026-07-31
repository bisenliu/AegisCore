package application

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

func TestPolicyRefreshCoordinatorReloadsAndTracksRevisionLocally(t *testing.T) {
	const revision int64 = 12
	engine := NewMockPolicyReloadEngine(gomock.NewController(t))
	engine.EXPECT().Reload(gomock.Any()).Return(nil)
	engine.EXPECT().InvalidateAllUserRoles()
	tracker := NewMockPolicyVersionTracker(gomock.NewController(t))
	tracker.EXPECT().MarkApplied(revision)
	metrics := NewMockMetrics(gomock.NewController(t))
	metrics.EXPECT().PolicyReloadSucceeded(gomock.Any(), MetricsSourceLocalChange)
	coordinator := NewPolicyRefreshCoordinator(engine, tracker, nil, metrics)

	require.NoError(t, coordinator.NotifyPolicyChanged(context.Background(), revision, NewPolicyReloadChange("role_permission_added")))
}

func TestPolicyRefreshCoordinatorDoesNotTrackWhenReloadFails(t *testing.T) {
	const revision int64 = 13
	reloadErr := errors.New("reload failed")
	engine := NewMockPolicyReloadEngine(gomock.NewController(t))
	engine.EXPECT().Reload(gomock.Any()).Return(reloadErr)
	tracker := NewMockPolicyVersionTracker(gomock.NewController(t))
	metrics := NewMockMetrics(gomock.NewController(t))
	metrics.EXPECT().PolicyReloadFailed(gomock.Any(), MetricsSourceLocalChange, MetricsReasonReloadFailed)
	coordinator := NewPolicyRefreshCoordinator(engine, tracker, nil, metrics)

	err := coordinator.NotifyPolicyChanged(context.Background(), revision, NewPolicyReloadChange("permission_updated"))
	require.ErrorIs(t, err, reloadErr)
}

func TestPolicyRefreshCoordinatorUserRoleChangeInvalidatesWithoutReload(t *testing.T) {
	const revision int64 = 7
	userID := uuid.MustParse("018f0000-0000-7000-8000-000000000901")
	roleID := uuid.MustParse("018f0000-0000-7000-8000-000000000902")
	engine := NewMockPolicyReloadEngine(gomock.NewController(t))
	engine.EXPECT().InvalidateUserRole(userID)
	tracker := NewMockPolicyVersionTracker(gomock.NewController(t))
	tracker.EXPECT().MarkApplied(revision)
	metrics := NewMockMetrics(gomock.NewController(t))
	coordinator := NewPolicyRefreshCoordinator(engine, tracker, nil, metrics)

	require.NoError(t, coordinator.NotifyPolicyChanged(context.Background(), revision, NewUserRoleChange("user_role_added", userID, roleID)))
}
