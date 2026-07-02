package casbin

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"
	"go.uber.org/fx/fxtest"
	"go.uber.org/mock/gomock"

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
	engine := NewEngine(Params{Loader: loader, UserRoles: roles})
	if err := engine.Reload(context.Background()); err != nil {
		t.Fatalf("Reload: %v", err)
	}
	allowed, err := engine.Enforce(context.Background(), userID, "/api/v1/users", "GET")
	if err != nil {
		t.Fatalf("Enforce allow: %v", err)
	}
	if !allowed {
		t.Fatal("matching policy denied")
	}
	denied, err := engine.Enforce(context.Background(), userID, "/api/v1/users", "POST")
	if err != nil {
		t.Fatalf("Enforce deny: %v", err)
	}
	if denied {
		t.Fatal("missing policy allowed")
	}
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
	engine := NewEngine(Params{Loader: loader, UserRoles: roles})
	if err := engine.Reload(context.Background()); err != nil {
		t.Fatalf("Reload: %v", err)
	}
	allowed, err := engine.Enforce(context.Background(), userID, "/api/v1/anything/:id", "DELETE")
	if err != nil {
		t.Fatalf("Enforce: %v", err)
	}
	if !allowed {
		t.Fatal("super admin wildcard denied")
	}
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
	engine := NewEngine(Params{Loader: loader, Metrics: metrics, UserRoles: roles})
	lc := fxtest.NewLifecycle(t)
	RegisterInitialLoad(lc, engine)
	if err := lc.Start(context.Background()); err != nil {
		t.Fatalf("lifecycle Start: %v", err)
	}
	allowed, err := engine.Enforce(context.Background(), uuid.New(), "/api/v1/users", "GET")
	if err != nil {
		t.Fatalf("Enforce: %v", err)
	}
	if allowed {
		t.Fatal("failed initialization allowed request")
	}
	if !errors.Is(engine.LastError(), loadErr) {
		t.Fatalf("LastError = %v, want %v", engine.LastError(), loadErr)
	}
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
	engine := NewEngine(Params{Loader: loader, Metrics: metrics, UserRoles: roles})
	if err := engine.Reload(context.Background()); err != nil {
		t.Fatalf("initial Reload: %v", err)
	}
	if err := engine.Reload(context.Background()); !errors.Is(err, loadErr) {
		t.Fatalf("Reload err = %v, want %v", err, loadErr)
	}
	allowed, err := engine.Enforce(context.Background(), userID, "/api/v1/users", "GET")
	if err != nil {
		t.Fatalf("Enforce: %v", err)
	}
	if !allowed {
		t.Fatal("previous policy was not preserved after reload failure")
	}
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
	engine := NewEngine(Params{Loader: loader, Metrics: metrics, UserRoles: roles})
	lc := fxtest.NewLifecycle(t)
	RegisterInitialLoad(lc, engine)
	if err := lc.Start(context.Background()); err != nil {
		t.Fatalf("lifecycle Start: %v", err)
	}
	allowed, err := engine.Enforce(context.Background(), userID, "/api/v1/users", "GET")
	if err != nil {
		t.Fatalf("Enforce after failed init: %v", err)
	}
	if allowed {
		t.Fatal("failed initialization allowed request")
	}

	if err := engine.Reload(context.Background()); err != nil {
		t.Fatalf("Reload: %v", err)
	}
	allowed, err = engine.Enforce(context.Background(), userID, "/api/v1/users", "GET")
	if err != nil {
		t.Fatalf("Enforce after reload: %v", err)
	}
	if !allowed {
		t.Fatal("reloaded policy denied matching request")
	}
	if engine.LastError() != nil {
		t.Fatalf("LastError = %v, want nil", engine.LastError())
	}
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
	engine := NewEngine(Params{Loader: loader, UserRoles: roles})
	if err := engine.Reload(context.Background()); err != nil {
		t.Fatalf("Reload: %v", err)
	}
	allowed, err := engine.Enforce(context.Background(), userID, "/api/v1/users", "GET")
	if !errors.Is(err, resolveErr) {
		t.Fatalf("Enforce err = %v, want %v", err, resolveErr)
	}
	if allowed {
		t.Fatal("resolver error allowed request")
	}
}

func TestEngineInitialLoadUsesLifecycleContext(t *testing.T) {
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
	engine := NewEngine(Params{Loader: loader, Metrics: metrics, UserRoles: roles})
	lc := fxtest.NewLifecycle(t)
	RegisterInitialLoad(lc, engine)

	if err := lc.Start(ctx); err != nil {
		t.Fatalf("lifecycle Start: %v", err)
	}
	if !errors.Is(engine.LastError(), context.Canceled) {
		t.Fatalf("LastError = %v, want context.Canceled", engine.LastError())
	}
	allowed, err := engine.Enforce(context.Background(), uuid.New(), "/api/v1/users", "GET")
	if err != nil {
		t.Fatalf("Enforce: %v", err)
	}
	if allowed {
		t.Fatal("canceled initialization allowed request")
	}
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
	engine := NewEngine(Params{Loader: loader, UserRoles: roles})

	engine.InvalidateUserRole(userID)
	engine.InvalidateAllUserRoles()
}
