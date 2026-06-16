package casbin

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"

	"github.com/aegiscore/user-service/internal/shared/rbacbaseline"
)

func TestEngineEnforceAllowDenyAndDoesNotReload(t *testing.T) {
	userID := uuid.MustParse("018f0000-0000-7000-8000-000000000301")
	roleID := uuid.MustParse("018f0000-0000-7000-8000-000000000302")
	loader := &fakeLoader{policies: PolicySet{
		GroupingPolicies: []GroupingPolicy{{UserID: userID, RoleID: roleID}},
		PermissionRules:  []PermissionRule{{RoleID: roleID, PathTemplate: "/api/v1/users", HTTPMethod: "GET"}},
	}}
	engine := NewEngine(Params{Loader: loader})
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
	if loader.calls != 1 {
		t.Fatalf("loader calls = %d, want 1", loader.calls)
	}
}

func TestEngineSuperAdminWildcard(t *testing.T) {
	userID := uuid.MustParse("018f0000-0000-7000-8000-000000000401")
	superAdminRoleID := uuid.MustParse(rbacbaseline.SuperAdminRoleID)
	engine := NewEngine(Params{Loader: &fakeLoader{policies: PolicySet{
		GroupingPolicies: []GroupingPolicy{{UserID: userID, RoleID: superAdminRoleID}},
		PermissionRules:  []PermissionRule{{RoleID: superAdminRoleID, PathTemplate: policyWildcard, HTTPMethod: policyWildcard}},
	}}})
	allowed, err := engine.Enforce(context.Background(), userID, "/api/v1/anything/:id", "DELETE")
	if err != nil {
		t.Fatalf("Enforce: %v", err)
	}
	if !allowed {
		t.Fatal("super admin wildcard denied")
	}
}

func TestEngineFailClosedWhenInitialLoadFails(t *testing.T) {
	loadErr := errors.New("load failed")
	engine := NewEngine(Params{Loader: &fakeLoader{err: loadErr}})
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
	userID := uuid.MustParse("018f0000-0000-7000-8000-000000000501")
	roleID := uuid.MustParse("018f0000-0000-7000-8000-000000000502")
	loadErr := errors.New("reload failed")
	loader := &fakeLoader{policies: PolicySet{
		GroupingPolicies: []GroupingPolicy{{UserID: userID, RoleID: roleID}},
		PermissionRules:  []PermissionRule{{RoleID: roleID, PathTemplate: "/api/v1/users", HTTPMethod: "GET"}},
	}}
	engine := NewEngine(Params{Loader: loader})
	loader.err = loadErr
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
	userID := uuid.MustParse("018f0000-0000-7000-8000-000000000503")
	roleID := uuid.MustParse("018f0000-0000-7000-8000-000000000504")
	loader := &fakeLoader{err: errors.New("initial load failed")}
	engine := NewEngine(Params{Loader: loader})
	allowed, err := engine.Enforce(context.Background(), userID, "/api/v1/users", "GET")
	if err != nil {
		t.Fatalf("Enforce after failed init: %v", err)
	}
	if allowed {
		t.Fatal("failed initialization allowed request")
	}

	loader.err = nil
	loader.policies = PolicySet{
		GroupingPolicies: []GroupingPolicy{{UserID: userID, RoleID: roleID}},
		PermissionRules:  []PermissionRule{{RoleID: roleID, PathTemplate: "/api/v1/users", HTTPMethod: "GET"}},
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

type fakeLoader struct {
	policies PolicySet
	err      error
	calls    int
}

func (l *fakeLoader) LoadPolicies(context.Context) (PolicySet, error) {
	l.calls++
	if l.err != nil {
		return PolicySet{}, l.err
	}
	return l.policies, nil
}
