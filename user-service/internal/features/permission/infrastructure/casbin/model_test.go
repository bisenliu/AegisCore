package casbin

import (
	"testing"

	casbinlib "github.com/casbin/casbin/v2"
	"github.com/google/uuid"
)

func TestEmbeddedModelWildcardMatcher(t *testing.T) {
	model, err := newModel()
	if err != nil {
		t.Fatalf("newModel: %v", err)
	}
	enforcer, err := casbinlib.NewEnforcer(model)
	if err != nil {
		t.Fatalf("NewEnforcer: %v", err)
	}
	roleID := uuid.MustParse("018f0000-0000-7000-8000-000000000002")
	if _, err := enforcer.AddPolicy(roleSubject(roleID), policyWildcard, policyWildcard); err != nil {
		t.Fatalf("AddPolicy: %v", err)
	}
	allowed, err := enforcer.Enforce(roleSubject(roleID), "/api/v1/users/:id", "DELETE")
	if err != nil {
		t.Fatalf("Enforce: %v", err)
	}
	if !allowed {
		t.Fatal("wildcard policy denied request")
	}
}
