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
		authz    *Authorizer
		wantErr  error
		wantCall bool
	}{
		{name: "allowed", authz: NewAuthorizer(&fakeEnforcer{allowed: true}), wantCall: true},
		{name: "denied", authz: NewAuthorizer(&fakeEnforcer{allowed: false}), wantErr: ErrDenied, wantCall: true},
		{name: "error", authz: NewAuthorizer(&fakeEnforcer{err: errFakeEnforcerUnavailable}), wantErr: errFakeEnforcerUnavailable, wantCall: true},
		{name: "missing authorizer", wantErr: ErrNotConfigured},
		{name: "missing enforcer", authz: NewAuthorizer(nil), wantErr: ErrNotConfigured},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.authz.Authorize(context.Background(), Request{Subject: "user:1", Object: "/api/v1/users/:id", Action: "GET"})
			if tt.wantErr == nil && err != nil {
				t.Fatalf("Authorize error = %v, want nil", err)
			}
			if tt.wantErr != nil && !errors.Is(err, tt.wantErr) {
				t.Fatalf("Authorize error = %v, want %v", err, tt.wantErr)
			}
			if !tt.wantCall {
				return
			}
			enforcer, ok := tt.authz.enforcer.(*fakeEnforcer)
			if !ok {
				t.Fatalf("enforcer = %T, want *fakeEnforcer", tt.authz.enforcer)
			}
			if enforcer.subject != "user:1" || enforcer.object != "/api/v1/users/:id" || enforcer.action != "GET" {
				t.Fatalf("enforcer call = %#v", enforcer)
			}
		})
	}
}

func TestEnforce(t *testing.T) {
	tests := []struct {
		name        string
		enforcer    Enforcer
		wantAllowed bool
		wantErr     error
	}{
		{name: "allowed", enforcer: &fakeEnforcer{allowed: true}, wantAllowed: true},
		{name: "denied", enforcer: &fakeEnforcer{allowed: false}},
		{name: "error", enforcer: &fakeEnforcer{err: errFakeEnforcerUnavailable}, wantErr: errFakeEnforcerUnavailable},
		{name: "missing enforcer", wantErr: ErrNotConfigured},
		{name: "typed nil enforcer", enforcer: (*fakeEnforcer)(nil), wantErr: ErrNotConfigured},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			allowed, err := Enforce(context.Background(), tt.enforcer, Request{Subject: "user:1", Object: "/users", Action: "GET"})
			if allowed != tt.wantAllowed {
				t.Fatalf("Enforce allowed = %v, want %v", allowed, tt.wantAllowed)
			}
			if tt.wantErr == nil && err != nil {
				t.Fatalf("Enforce error = %v, want nil", err)
			}
			if tt.wantErr != nil && !errors.Is(err, tt.wantErr) {
				t.Fatalf("Enforce error = %v, want %v", err, tt.wantErr)
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
	if e == nil {
		return false, ErrNotConfigured
	}
	if len(args) != 3 {
		return false, errors.New("unexpected argument count")
	}
	e.subject, _ = args[0].(string)
	e.object, _ = args[1].(string)
	e.action, _ = args[2].(string)
	return e.allowed, e.err
}
