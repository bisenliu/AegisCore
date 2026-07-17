package casbin

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	commonmetrics "github.com/aegiscore/common/runtime/observability/metrics"
	"github.com/aegiscore/user-service/internal/shared/rbacbaseline"
)

func TestEngineEnforceAllowDenyAndDoesNotReload(t *testing.T) {
	ctrl := gomock.NewController(t)
	userID := uuid.MustParse("018f0000-0000-7000-8000-000000000301")
	roleID := uuid.MustParse("018f0000-0000-7000-8000-000000000302")

	loader := NewMockLoader(ctrl)
	loader.EXPECT().LoadPolicies(gomock.Any()).Return(PolicySet{
		PermissionRules: []PermissionRule{{RoleID: roleID, PathTemplate: "/api/v1/users", HTTPMethod: "GET"}},
	}, nil).Times(1)
	roles := NewMockUserRoleResolver(ctrl)
	roles.EXPECT().RolesForUser(gomock.Any(), userID).Return([]uuid.UUID{roleID}, nil).Times(2)
	engine := NewEngine(loader, commonmetrics.NopReloadMetrics(), roles)
	require.NoError(t, engine.Reload(context.Background()))
	allowed, err := engine.Enforce(context.Background(), userID, "/api/v1/users", "GET")
	require.NoError(t, err)
	require.True(t, allowed)
	denied, err := engine.Enforce(context.Background(), userID, "/api/v1/users", "POST")
	require.NoError(t, err)
	require.False(t, denied)
}

func TestEngineSuperAdminWildcard(t *testing.T) {
	ctrl := gomock.NewController(t)
	userID := uuid.MustParse("018f0000-0000-7000-8000-000000000401")
	superAdminRoleID := uuid.MustParse(rbacbaseline.SuperAdminRoleID)
	loader := NewMockLoader(ctrl)
	loader.EXPECT().LoadPolicies(gomock.Any()).Return(PolicySet{
		PermissionRules: []PermissionRule{{RoleID: superAdminRoleID, PathTemplate: policyWildcard, HTTPMethod: policyWildcard}},
	}, nil)
	roles := NewMockUserRoleResolver(ctrl)
	roles.EXPECT().RolesForUser(gomock.Any(), userID).Return([]uuid.UUID{superAdminRoleID}, nil)
	engine := NewEngine(loader, commonmetrics.NopReloadMetrics(), roles)
	require.NoError(t, engine.Reload(context.Background()))
	allowed, err := engine.Enforce(context.Background(), userID, "/api/v1/anything/:id", "DELETE")
	require.NoError(t, err)
	require.True(t, allowed)
}

func TestEngineFailClosedWhenInitialLoadFails(t *testing.T) {
	ctrl := gomock.NewController(t)
	loadErr := errors.New("load failed")
	loader := NewMockLoader(ctrl)
	loader.EXPECT().LoadPolicies(gomock.Any()).Return(PolicySet{}, loadErr)
	metrics := NewMockReloadMetrics(ctrl)
	gomock.InOrder(
		metrics.EXPECT().ReloadFailed(),
		metrics.EXPECT().SetLastStatus(false),
	)
	roles := NewMockUserRoleResolver(ctrl)
	engine := NewEngine(loader, metrics, roles)
	require.NoError(t, engine.Initialize(context.Background()))
	allowed, err := engine.Enforce(context.Background(), uuid.New(), "/api/v1/users", "GET")
	require.NoError(t, err)
	require.False(t, allowed)
	require.ErrorIs(t, engine.LastError(), loadErr)
}

func TestEngineReloadFailurePreservesPreviousPolicy(t *testing.T) {
	ctrl := gomock.NewController(t)
	userID := uuid.MustParse("018f0000-0000-7000-8000-000000000501")
	roleID := uuid.MustParse("018f0000-0000-7000-8000-000000000502")
	loadErr := errors.New("reload failed")

	loader := NewMockLoader(ctrl)
	gomock.InOrder(
		loader.EXPECT().LoadPolicies(gomock.Any()).Return(PolicySet{
			PermissionRules: []PermissionRule{{RoleID: roleID, PathTemplate: "/api/v1/users", HTTPMethod: "GET"}},
		}, nil),
		loader.EXPECT().LoadPolicies(gomock.Any()).Return(PolicySet{}, loadErr),
	)
	roles := NewMockUserRoleResolver(ctrl)
	roles.EXPECT().RolesForUser(gomock.Any(), userID).Return([]uuid.UUID{roleID}, nil)
	metrics := NewMockReloadMetrics(ctrl)
	gomock.InOrder(
		metrics.EXPECT().ReloadSucceeded(),
		metrics.EXPECT().SetLastStatus(true),
		metrics.EXPECT().ReloadFailed(),
		metrics.EXPECT().SetLastStatus(false),
	)
	engine := NewEngine(loader, metrics, roles)
	require.NoError(t, engine.Reload(context.Background()))
	require.ErrorIs(t, engine.Reload(context.Background()), loadErr)
	allowed, err := engine.Enforce(context.Background(), userID, "/api/v1/users", "GET")
	require.NoError(t, err)
	require.True(t, allowed)
}

func TestEngineReloadSuccessReplacesPolicyAndClearsError(t *testing.T) {
	ctrl := gomock.NewController(t)
	userID := uuid.MustParse("018f0000-0000-7000-8000-000000000503")
	roleID := uuid.MustParse("018f0000-0000-7000-8000-000000000504")
	loadErr := errors.New("initial load failed")

	loader := NewMockLoader(ctrl)
	gomock.InOrder(
		loader.EXPECT().LoadPolicies(gomock.Any()).Return(PolicySet{}, loadErr),
		loader.EXPECT().LoadPolicies(gomock.Any()).Return(PolicySet{
			PermissionRules: []PermissionRule{{RoleID: roleID, PathTemplate: "/api/v1/users", HTTPMethod: "GET"}},
		}, nil),
	)
	roles := NewMockUserRoleResolver(ctrl)
	roles.EXPECT().RolesForUser(gomock.Any(), userID).Return([]uuid.UUID{roleID}, nil)
	metrics := NewMockReloadMetrics(ctrl)
	gomock.InOrder(
		metrics.EXPECT().ReloadFailed(),
		metrics.EXPECT().SetLastStatus(false),
		metrics.EXPECT().ReloadSucceeded(),
		metrics.EXPECT().SetLastStatus(true),
	)
	engine := NewEngine(loader, metrics, roles)
	require.NoError(t, engine.Initialize(context.Background()))
	allowed, err := engine.Enforce(context.Background(), userID, "/api/v1/users", "GET")
	require.NoError(t, err)
	require.False(t, allowed)

	require.NoError(t, engine.Reload(context.Background()))
	allowed, err = engine.Enforce(context.Background(), userID, "/api/v1/users", "GET")
	require.NoError(t, err)
	require.True(t, allowed)
	require.NoError(t, engine.LastError())
}

func TestEngineEnforceReturnsRoleResolverError(t *testing.T) {
	ctrl := gomock.NewController(t)
	userID := uuid.MustParse("018f0000-0000-7000-8000-000000000601")
	roleID := uuid.MustParse("018f0000-0000-7000-8000-000000000602")
	resolveErr := errors.New("resolve roles failed")

	loader := NewMockLoader(ctrl)
	loader.EXPECT().LoadPolicies(gomock.Any()).Return(PolicySet{
		PermissionRules: []PermissionRule{{RoleID: roleID, PathTemplate: "/api/v1/users", HTTPMethod: "GET"}},
	}, nil)
	roles := NewMockUserRoleResolver(ctrl)
	roles.EXPECT().RolesForUser(gomock.Any(), userID).Return(nil, resolveErr)
	engine := NewEngine(loader, commonmetrics.NopReloadMetrics(), roles)
	require.NoError(t, engine.Reload(context.Background()))
	allowed, err := engine.Enforce(context.Background(), userID, "/api/v1/users", "GET")
	require.ErrorIs(t, err, resolveErr)
	require.False(t, allowed)
}

func TestEngineInitialLoadUsesInitializeContext(t *testing.T) {
	ctrl := gomock.NewController(t)
	ctx, cancel := context.WithCancel(context.Background())
	loader := NewMockLoader(ctrl)
	loader.EXPECT().LoadPolicies(ctx).DoAndReturn(func(ctx context.Context) (PolicySet, error) {
		cancel()
		if err := ctx.Err(); err != nil {
			return PolicySet{}, err
		}
		return PolicySet{}, nil
	})
	metrics := NewMockReloadMetrics(ctrl)
	gomock.InOrder(
		metrics.EXPECT().ReloadFailed(),
		metrics.EXPECT().SetLastStatus(false),
	)
	roles := NewMockUserRoleResolver(ctrl)
	engine := NewEngine(loader, metrics, roles)

	require.NoError(t, engine.Initialize(ctx))
	require.ErrorIs(t, engine.LastError(), context.Canceled)
	allowed, err := engine.Enforce(context.Background(), uuid.New(), "/api/v1/users", "GET")
	require.NoError(t, err)
	require.False(t, allowed)
}

func TestEngineInvalidatesUserRoleResolver(t *testing.T) {
	ctrl := gomock.NewController(t)
	userID := uuid.MustParse("018f0000-0000-7000-8000-000000000505")
	loader := NewMockLoader(ctrl)
	roles := NewMockUserRoleResolver(ctrl)
	gomock.InOrder(
		roles.EXPECT().InvalidateUserRole(userID),
		roles.EXPECT().InvalidateAllUserRoles(),
	)
	engine := NewEngine(loader, commonmetrics.NopReloadMetrics(), roles)

	engine.InvalidateUserRole(userID)
	engine.InvalidateAllUserRoles()
}
