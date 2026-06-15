package casbin

import (
	"context"
	"errors"
	"testing"
)

var errFakeEnforcerUnavailable = errors.New("engine unavailable")

func TestAuthorizerAuthorize(t *testing.T) {
	tests := []struct {
		name     string
		enforcer *fakeEnforcer
		wantErr  error
	}{
		{name: "allowed", enforcer: &fakeEnforcer{allowed: true}},
		{name: "denied", enforcer: &fakeEnforcer{allowed: false}, wantErr: ErrDenied},
		{name: "error", enforcer: &fakeEnforcer{err: errFakeEnforcerUnavailable}, wantErr: errFakeEnforcerUnavailable},
		{name: "missing enforcer", wantErr: ErrNotConfigured},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := NewAuthorizer(tt.enforcer).Authorize(context.Background(), Request{Subject: "user:1", Object: "/api/v1/users/:id", Action: "GET"})
			if tt.wantErr == nil && err != nil {
				t.Fatalf("Authorize error = %v, want nil", err)
			}
			if tt.wantErr != nil && !errors.Is(err, tt.wantErr) {
				t.Fatalf("Authorize error = %v, want %v", err, tt.wantErr)
			}
			if tt.enforcer != nil && (tt.enforcer.subject != "user:1" || tt.enforcer.object != "/api/v1/users/:id" || tt.enforcer.action != "GET") {
				t.Fatalf("enforcer call = %#v", tt.enforcer)
			}
		})
	}
}

func TestEnforceHonorsContext(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	allowed, err := Enforce(ctx, &fakeEnforcer{allowed: true}, Request{Subject: "user:1", Object: "/users", Action: "GET"})
	if allowed || !errors.Is(err, context.Canceled) {
		t.Fatalf("Enforce = (%v, %v), want false, context.Canceled", allowed, err)
	}
}

type fakeEnforcer struct {
	allowed bool
	err     error
	subject string
	object  string
	action  string
}

func (e *fakeEnforcer) Enforce(args ...interface{}) (bool, error) {
	if len(args) != 3 {
		return false, errors.New("unexpected argument count")
	}
	e.subject, _ = args[0].(string)
	e.object, _ = args[1].(string)
	e.action, _ = args[2].(string)
	return e.allowed, e.err
}
