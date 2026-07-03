package domain

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestRouteIdentityValidation(t *testing.T) {
	t.Run("normalizes http method", func(t *testing.T) {
		identity, err := NewRouteIdentity(" get ", "/api/v1/users")
		require.NoError(t, err)
		require.Equal(t, "GET", identity.Method)
		require.Equal(t, "/api/v1/users", identity.PathTemplate)
	})

	t.Run("rejects unsupported method", func(t *testing.T) {
		_, err := NewRouteIdentity("TRACE", "/api/v1/users")
		require.ErrorIs(t, err, ErrPermissionInvalid)
	})

	t.Run("rejects invalid path", func(t *testing.T) {
		_, err := NewRouteIdentity("GET", "/livez")
		require.ErrorIs(t, err, ErrPermissionInvalid)
	})
}

func TestSystemPermissionProtection(t *testing.T) {
	permission := Permission{HTTPMethod: "GET", PathTemplate: "/api/v1/users", IsSystem: true}

	require.NoError(t, permission.ProtectSystemIdentity(RouteIdentity{Method: "GET", PathTemplate: "/api/v1/users"}))

	err := permission.ProtectSystemIdentity(RouteIdentity{Method: "POST", PathTemplate: "/api/v1/users"})
	require.ErrorIs(t, err, ErrSystemPermissionProtected)

	nonSystem := Permission{HTTPMethod: "GET", PathTemplate: "/api/v1/users"}
	require.NoError(t, nonSystem.ProtectSystemIdentity(RouteIdentity{Method: "POST", PathTemplate: "/api/v1/users"}))
}
