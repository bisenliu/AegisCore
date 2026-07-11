package domain

import (
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	contracterrors "github.com/aegiscore/common/contract/errors"
	"github.com/aegiscore/user-service/internal/messages"
)

func TestPermissionErrorsAreApplicationErrors(t *testing.T) {
	tests := []struct {
		name    string
		err     error
		kind    contracterrors.Kind
		reason  contracterrors.Reason
		code    contracterrors.Code
		message string
	}{
		{
			name:    "permission not found",
			err:     ErrPermissionNotFound,
			kind:    contracterrors.KindNotFound,
			reason:  reasonPermissionNotFound,
			code:    contracterrors.CodeNotFound,
			message: messages.PermissionNotFound,
		},
		{
			name:    "permission already exists",
			err:     ErrPermissionAlreadyExists,
			kind:    contracterrors.KindConflict,
			reason:  reasonPermissionAlreadyExists,
			code:    contracterrors.CodeConflict,
			message: messages.PermissionAlreadyExists,
		},
		{
			name:    "permission invalid",
			err:     ErrPermissionInvalid,
			kind:    contracterrors.KindValidation,
			reason:  reasonPermissionInvalid,
			code:    contracterrors.CodeValidationFailed,
			message: messages.InvalidPermission,
		},
		{
			name:    "system permission protected",
			err:     ErrSystemPermissionProtected,
			kind:    contracterrors.KindConflict,
			reason:  reasonSystemPermissionProtected,
			code:    contracterrors.CodeConflict,
			message: messages.SystemPermissionProtected,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var appErr *contracterrors.Error
			require.ErrorAs(t, tt.err, &appErr)
			require.Equal(t, tt.kind, appErr.Kind)
			require.Equal(t, tt.reason, appErr.Reason)
			require.Equal(t, tt.code, appErr.Code)
			require.Equal(t, tt.message, appErr.Message)
		})
	}
}

func TestPermissionErrorsSupportErrorsIs(t *testing.T) {
	tests := []struct {
		name string
		err  error
	}{
		{name: "permission not found", err: ErrPermissionNotFound},
		{name: "permission already exists", err: ErrPermissionAlreadyExists},
		{name: "permission invalid", err: ErrPermissionInvalid},
		{name: "system permission protected", err: ErrSystemPermissionProtected},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require.ErrorIs(t, tt.err, tt.err)
			require.ErrorIs(t, fmt.Errorf("permission operation failed: %w", tt.err), tt.err)
		})
	}

	for _, current := range tests {
		for _, target := range tests {
			if current.name == target.name {
				continue
			}
			require.False(t, errors.Is(current.err, target.err), "%s should not match %s", current.name, target.name)
		}
	}
}

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

func TestNormalizeHTTPMethodAllowlist(t *testing.T) {
	for _, method := range []string{"GET", "POST", "PUT", "PATCH", "DELETE"} {
		t.Run(method, func(t *testing.T) {
			got, err := NormalizeHTTPMethod(" " + strings.ToLower(method) + " ")
			require.NoError(t, err)
			require.Equal(t, method, got)
		})
	}

	for _, method := range []string{"", "HEAD", "OPTIONS", "TRACE", "CONNECT"} {
		t.Run("reject_"+method, func(t *testing.T) {
			_, err := NormalizeHTTPMethod(method)
			require.ErrorIs(t, err, ErrPermissionInvalid)
			require.ErrorContains(t, err, "unsupported http method")
		})
	}
}

func TestSystemPermissionProtection(t *testing.T) {
	permission := Permission{HTTPMethod: "GET", PathTemplate: "/api/v1/users", IsSystem: true}

	require.NoError(t, permission.ProtectSystemIdentity(RouteIdentity{Method: "GET", PathTemplate: "/api/v1/users"}))

	err := permission.ProtectSystemIdentity(RouteIdentity{Method: "POST", PathTemplate: "/api/v1/users"})
	require.ErrorIs(t, err, ErrSystemPermissionProtected)

	nonSystem := Permission{HTTPMethod: "GET", PathTemplate: "/api/v1/users"}
	require.NoError(t, nonSystem.ProtectSystemIdentity(RouteIdentity{Method: "POST", PathTemplate: "/api/v1/users"}))
}
