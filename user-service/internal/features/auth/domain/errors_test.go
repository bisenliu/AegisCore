package domain

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/require"

	contracterrors "github.com/aegiscore/common/contract/errors"
	"github.com/aegiscore/user-service/internal/messages"
)

func TestAuthDomainErrorsCarryApplicationContract(t *testing.T) {
	tests := []struct {
		name    string
		err     error
		kind    contracterrors.Kind
		reason  contracterrors.Reason
		code    contracterrors.Code
		message string
	}{
		{
			name:    "invalid credentials",
			err:     ErrInvalidCredentials,
			kind:    contracterrors.KindUnauthenticated,
			reason:  reasonInvalidCredentials,
			code:    contracterrors.CodeUnauthenticated,
			message: messages.InvalidCredentials,
		},
		{
			name:    "user status rejected",
			err:     ErrUserStatusRejected,
			kind:    contracterrors.KindUnauthenticated,
			reason:  reasonUserStatusRejected,
			code:    contracterrors.CodeUnauthenticated,
			message: messages.InvalidCredentials,
		},
		{
			name:    "missing session",
			err:     ErrMissingSession,
			kind:    contracterrors.KindUnauthenticated,
			reason:  reasonMissingSession,
			code:    contracterrors.CodeUnauthenticated,
			message: messages.MissingSession,
		},
		{
			name:    "token invalid",
			err:     ErrTokenInvalid,
			kind:    contracterrors.KindUnauthenticated,
			reason:  reasonAuthTokenInvalid,
			code:    contracterrors.CodeTokenInvalid,
			message: messages.MissingSession,
		},
		{
			name:    "auth session not found",
			err:     ErrAuthSessionNotFound,
			kind:    contracterrors.KindUnauthenticated,
			reason:  reasonAuthSessionNotFound,
			code:    contracterrors.CodeTokenInvalid,
			message: messages.MissingSession,
		},
		{
			name:    "auth session mismatch",
			err:     ErrAuthSessionMismatch,
			kind:    contracterrors.KindUnauthenticated,
			reason:  reasonAuthSessionMismatch,
			code:    contracterrors.CodeTokenInvalid,
			message: messages.MissingSession,
		},
		{
			name:    "password change session not found",
			err:     ErrPasswordChangeSessionNotFound,
			kind:    contracterrors.KindUnauthenticated,
			reason:  reasonPasswordChangeSessionNotFound,
			code:    contracterrors.CodeTokenInvalid,
			message: messages.MissingSession,
		},
		{
			name:    "password change session mismatch",
			err:     ErrPasswordChangeSessionMismatch,
			kind:    contracterrors.KindUnauthenticated,
			reason:  reasonPasswordChangeSessionMismatch,
			code:    contracterrors.CodeTokenInvalid,
			message: messages.MissingSession,
		},
		{
			name:    "session revocation incomplete",
			err:     ErrSessionRevocationIncomplete,
			kind:    contracterrors.KindServiceUnavailable,
			reason:  reasonSessionRevocationIncomplete,
			code:    contracterrors.CodeServiceUnavailable,
			message: messages.AuthRevocationIncomplete,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			wrapped := errors.Join(errors.New("outer"), tt.err)
			require.ErrorIs(t, wrapped, tt.err)

			var appErr *contracterrors.Error
			require.ErrorAs(t, tt.err, &appErr)
			require.Equal(t, tt.kind, appErr.Kind)
			require.Equal(t, tt.reason, appErr.Reason)
			require.Equal(t, tt.code, appErr.Code)
			require.Equal(t, tt.message, appErr.Message)
		})
	}
}
