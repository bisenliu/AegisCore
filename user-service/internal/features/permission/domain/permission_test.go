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
			name:    "permission invalid",
			err:     ErrPermissionInvalid,
			kind:    contracterrors.KindValidation,
			reason:  reasonPermissionInvalid,
			code:    contracterrors.CodeValidationFailed,
			message: messages.InvalidPermission,
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
		{name: "permission invalid", err: ErrPermissionInvalid},
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
