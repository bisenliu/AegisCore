package domain

import (
	"errors"
	"testing"
)

func TestRouteIdentityValidation(t *testing.T) {
	t.Run("normalizes http method", func(t *testing.T) {
		identity, err := NewRouteIdentity(" get ", "/api/v1/users")
		if err != nil {
			t.Fatalf("NewRouteIdentity: %v", err)
		}
		if identity.Method != "GET" || identity.PathTemplate != "/api/v1/users" {
			t.Fatalf("identity = %#v", identity)
		}
	})

	t.Run("rejects unsupported method", func(t *testing.T) {
		_, err := NewRouteIdentity("TRACE", "/api/v1/users")
		if !errors.Is(err, ErrPermissionInvalid) {
			t.Fatalf("err = %v, want ErrPermissionInvalid", err)
		}
	})

	t.Run("rejects invalid path", func(t *testing.T) {
		_, err := NewRouteIdentity("GET", "/livez")
		if !errors.Is(err, ErrPermissionInvalid) {
			t.Fatalf("err = %v, want ErrPermissionInvalid", err)
		}
	})
}

func TestSystemPermissionProtection(t *testing.T) {
	permission := Permission{HTTPMethod: "GET", PathTemplate: "/api/v1/users", IsSystem: true}

	if err := permission.ProtectSystemIdentity(RouteIdentity{Method: "GET", PathTemplate: "/api/v1/users"}); err != nil {
		t.Fatalf("ProtectSystemIdentity matching identity: %v", err)
	}

	err := permission.ProtectSystemIdentity(RouteIdentity{Method: "POST", PathTemplate: "/api/v1/users"})
	if !errors.Is(err, ErrSystemPermissionProtected) {
		t.Fatalf("err = %v, want ErrSystemPermissionProtected", err)
	}

	nonSystem := Permission{HTTPMethod: "GET", PathTemplate: "/api/v1/users"}
	if err := nonSystem.ProtectSystemIdentity(RouteIdentity{Method: "POST", PathTemplate: "/api/v1/users"}); err != nil {
		t.Fatalf("non-system ProtectSystemIdentity: %v", err)
	}
}
