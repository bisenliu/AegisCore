package authorization

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"
)

func TestAuthorizerEnforceDelegatesValidUserID(t *testing.T) {
	engine := &fakeEngine{allowed: true}
	authz := NewAuthorizer(engine)
	userID := "018f0000-0000-7000-8000-000000000701"
	allowed, err := authz.Enforce(context.Background(), userID, "/api/v1/users/:user_id", "GET")
	if err != nil {
		t.Fatalf("Enforce: %v", err)
	}
	if !allowed {
		t.Fatal("allowed = false, want true")
	}
	if engine.calls != 1 || engine.userID.String() != userID || engine.pathTemplate != "/api/v1/users/:user_id" || engine.method != "GET" {
		t.Fatalf("engine call = %#v", engine)
	}
}

func TestAuthorizerEnforceInvalidUserIDFailsClosed(t *testing.T) {
	engine := &fakeEngine{allowed: true}
	authz := NewAuthorizer(engine)
	allowed, err := authz.Enforce(context.Background(), "not-a-uuid", "/api/v1/users", "GET")
	if !errors.Is(err, ErrInvalidSubjectUserID) {
		t.Fatalf("Enforce err = %v, want %v", err, ErrInvalidSubjectUserID)
	}
	if allowed {
		t.Fatal("invalid user allowed")
	}
	if engine.calls != 0 {
		t.Fatalf("engine calls = %d, want 0", engine.calls)
	}
}

type fakeEngine struct {
	allowed      bool
	calls        int
	userID       uuid.UUID
	pathTemplate string
	method       string
}

func (e *fakeEngine) Enforce(_ context.Context, userID uuid.UUID, pathTemplate string, method string) (bool, error) {
	e.calls++
	e.userID = userID
	e.pathTemplate = pathTemplate
	e.method = method
	return e.allowed, nil
}
