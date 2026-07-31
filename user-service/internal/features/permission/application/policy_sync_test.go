package application

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

func TestPolicyRefreshCoordinatorReloadsToDatabaseRevision(t *testing.T) {
	const revision int64 = 12
	engine := NewMockPolicyReloadEngine(gomock.NewController(t))
	engine.EXPECT().ReloadToRevision(gomock.Any(), revision).Return(int64(14), nil)
	engine.EXPECT().ProjectionStatus().Return(PolicyProjectionStatus{Initialized: true, ReloadSucceeded: true, AppliedRevision: 14, TargetRevision: revision})
	engine.EXPECT().InvalidateAllUserRoles()
	engine.EXPECT().AppliedRevision().Return(int64(14))
	metrics := NewMockMetrics(gomock.NewController(t))
	metrics.EXPECT().PolicyReloadSucceeded(gomock.Any(), MetricsSourceLocalChange)
	coordinator := NewPolicyRefreshCoordinator(engine, nil, metrics)

	require.NoError(t, coordinator.NotifyPolicyChanged(context.Background(), revision, NewPolicyReloadChange("role_permission_added")))
}

func TestPolicyRefreshCoordinatorReturnsReloadFailureWithoutInvalidation(t *testing.T) {
	const revision int64 = 13
	reloadErr := errors.New("reload failed")
	engine := NewMockPolicyReloadEngine(gomock.NewController(t))
	engine.EXPECT().ReloadToRevision(gomock.Any(), revision).Return(int64(5), reloadErr)
	metrics := NewMockMetrics(gomock.NewController(t))
	metrics.EXPECT().PolicyReloadFailed(gomock.Any(), MetricsSourceLocalChange, MetricsReasonReloadFailed)
	coordinator := NewPolicyRefreshCoordinator(engine, nil, metrics)

	err := coordinator.NotifyPolicyChanged(context.Background(), revision, NewPolicyReloadChange("permission_updated"))
	require.ErrorIs(t, err, reloadErr)
}

func TestPolicyRefreshCoordinatorRejectsIncompleteProjectionStatus(t *testing.T) {
	const revision int64 = 13
	engine := NewMockPolicyReloadEngine(gomock.NewController(t))
	engine.EXPECT().ReloadToRevision(gomock.Any(), revision).Return(int64(12), nil)
	engine.EXPECT().ProjectionStatus().Return(PolicyProjectionStatus{Initialized: true, AppliedRevision: 12, TargetRevision: revision})
	metrics := NewMockMetrics(gomock.NewController(t))
	metrics.EXPECT().PolicyReloadFailed(gomock.Any(), MetricsSourceLocalChange, MetricsReasonReloadFailed)
	coordinator := NewPolicyRefreshCoordinator(engine, nil, metrics)

	require.ErrorContains(t, coordinator.NotifyPolicyChanged(context.Background(), revision, NewPolicyReloadChange("permission_updated")), "did not reach target revision")
}

func TestPolicyRefreshCoordinatorUserRoleChangeInvalidatesWithoutReload(t *testing.T) {
	const revision int64 = 7
	userID := uuid.MustParse("018f0000-0000-7000-8000-000000000901")
	roleID := uuid.MustParse("018f0000-0000-7000-8000-000000000902")
	engine := NewMockPolicyReloadEngine(gomock.NewController(t))
	engine.EXPECT().ObserveTargetRevision(revision)
	engine.EXPECT().InvalidateUserRole(userID)
	engine.EXPECT().AppliedRevision().Return(int64(3))
	metrics := NewMockMetrics(gomock.NewController(t))
	coordinator := NewPolicyRefreshCoordinator(engine, nil, metrics)

	require.NoError(t, coordinator.NotifyPolicyChanged(context.Background(), revision, NewUserRoleChange("user_role_added", userID, roleID)))
}
