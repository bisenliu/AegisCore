package casbin

import (
	"context"
	"errors"
	"testing"

	"go.uber.org/mock/gomock"
)

var errMockEnforcerUnavailable = errors.New("engine unavailable")

func TestAuthorizerAuthorize(t *testing.T) {
	tests := []struct {
		name    string
		setup   func(ctrl *gomock.Controller) *Authorizer
		wantErr error
	}{
		{name: "allowed", setup: func(ctrl *gomock.Controller) *Authorizer {
			enforcer := NewMockEnforcer(ctrl)
			enforcer.EXPECT().Enforce("user:1", "/api/v1/users/:id", "GET").Return(true, nil)
			return NewAuthorizer(enforcer)
		}},
		{name: "denied", setup: func(ctrl *gomock.Controller) *Authorizer {
			enforcer := NewMockEnforcer(ctrl)
			enforcer.EXPECT().Enforce("user:1", "/api/v1/users/:id", "GET").Return(false, nil)
			return NewAuthorizer(enforcer)
		}, wantErr: ErrDenied},
		{name: "error", setup: func(ctrl *gomock.Controller) *Authorizer {
			enforcer := NewMockEnforcer(ctrl)
			enforcer.EXPECT().Enforce("user:1", "/api/v1/users/:id", "GET").Return(false, errMockEnforcerUnavailable)
			return NewAuthorizer(enforcer)
		}, wantErr: errMockEnforcerUnavailable},
		{name: "missing authorizer", wantErr: ErrNotConfigured},
		{name: "missing enforcer", setup: func(*gomock.Controller) *Authorizer {
			return NewAuthorizer(nil)
		}, wantErr: ErrNotConfigured},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			var authz *Authorizer
			if tt.setup != nil {
				authz = tt.setup(ctrl)
			}
			err := authz.Authorize(context.Background(), Request{Subject: "user:1", Object: "/api/v1/users/:id", Action: "GET"})
			if tt.wantErr == nil && err != nil {
				t.Fatalf("Authorize error = %v, want nil", err)
			}
			if tt.wantErr != nil && !errors.Is(err, tt.wantErr) {
				t.Fatalf("Authorize error = %v, want %v", err, tt.wantErr)
			}
		})
	}
}

func TestEnforce(t *testing.T) {
	tests := []struct {
		name        string
		setup       func(ctrl *gomock.Controller) Enforcer
		wantAllowed bool
		wantErr     error
	}{
		{name: "allowed", setup: func(ctrl *gomock.Controller) Enforcer {
			enforcer := NewMockEnforcer(ctrl)
			enforcer.EXPECT().Enforce("user:1", "/users", "GET").Return(true, nil)
			return enforcer
		}, wantAllowed: true},
		{name: "denied", setup: func(ctrl *gomock.Controller) Enforcer {
			enforcer := NewMockEnforcer(ctrl)
			enforcer.EXPECT().Enforce("user:1", "/users", "GET").Return(false, nil)
			return enforcer
		}},
		{name: "error", setup: func(ctrl *gomock.Controller) Enforcer {
			enforcer := NewMockEnforcer(ctrl)
			enforcer.EXPECT().Enforce("user:1", "/users", "GET").Return(false, errMockEnforcerUnavailable)
			return enforcer
		}, wantErr: errMockEnforcerUnavailable},
		{name: "missing enforcer", wantErr: ErrNotConfigured},
		{name: "typed nil enforcer", setup: func(*gomock.Controller) Enforcer {
			var enforcer *MockEnforcer
			return enforcer
		}, wantErr: ErrNotConfigured},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			var enforcer Enforcer
			if tt.setup != nil {
				enforcer = tt.setup(ctrl)
			}
			allowed, err := Enforce(context.Background(), enforcer, Request{Subject: "user:1", Object: "/users", Action: "GET"})
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
	enforcer := NewMockEnforcer(gomock.NewController(t))
	allowed, err := Enforce(ctx, enforcer, Request{Subject: "user:1", Object: "/users", Action: "GET"})
	if allowed || !errors.Is(err, context.Canceled) {
		t.Fatalf("Enforce = (%v, %v), want false, context.Canceled", allowed, err)
	}
}
