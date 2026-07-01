package authorization

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"
	"go.uber.org/mock/gomock"
)

func TestAuthorizerEnforceDelegatesValidUserID(t *testing.T) {
	engine := NewMockEngine(gomock.NewController(t))
	var gotUserID uuid.UUID
	var gotPathTemplate string
	var gotMethod string
	engine.EXPECT().Enforce(gomock.Any(), gomock.Any(), "/api/v1/users/:user_id", "GET").DoAndReturn(func(_ context.Context, userID uuid.UUID, pathTemplate string, method string) (bool, error) {
		gotUserID = userID
		gotPathTemplate = pathTemplate
		gotMethod = method
		return true, nil
	})
	authz := NewAuthorizer(engine)
	userID := "018f0000-0000-7000-8000-000000000701"
	allowed, err := authz.Enforce(context.Background(), userID, "/api/v1/users/:user_id", "GET")
	if err != nil {
		t.Fatalf("Enforce: %v", err)
	}
	if !allowed {
		t.Fatal("allowed = false, want true")
	}
	if gotUserID.String() != userID || gotPathTemplate != "/api/v1/users/:user_id" || gotMethod != "GET" {
		t.Fatalf("engine call userID=%s path=%s method=%s", gotUserID, gotPathTemplate, gotMethod)
	}
}

func TestAuthorizerEnforceInvalidUserIDFailsClosed(t *testing.T) {
	engine := NewMockEngine(gomock.NewController(t))
	authz := NewAuthorizer(engine)
	allowed, err := authz.Enforce(context.Background(), "not-a-uuid", "/api/v1/users", "GET")
	if !errors.Is(err, ErrInvalidSubjectUserID) {
		t.Fatalf("Enforce err = %v, want %v", err, ErrInvalidSubjectUserID)
	}
	if allowed {
		t.Fatal("invalid user allowed")
	}
}
