package identity

import (
	"errors"
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"

	contracterrors "github.com/aegiscore/common/contract/errors"
	"github.com/aegiscore/user-service/internal/messages"
)

func TestIdentityErrorsAreApplicationErrors(t *testing.T) {
	tests := []struct {
		name    string
		err     error
		kind    contracterrors.Kind
		reason  contracterrors.Reason
		code    contracterrors.Code
		message string
	}{
		{
			name:    "user not found",
			err:     ErrUserNotFound,
			kind:    contracterrors.KindNotFound,
			reason:  reasonUserNotFound,
			code:    contracterrors.CodeNotFound,
			message: messages.UserNotFound,
		},
		{
			name:    "user already exists",
			err:     ErrUserAlreadyExists,
			kind:    contracterrors.KindConflict,
			reason:  reasonUserAlreadyExists,
			code:    contracterrors.CodeConflict,
			message: messages.UserAlreadyExists,
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

func TestIdentityErrorsSupportErrorsIs(t *testing.T) {
	require.ErrorIs(t, ErrUserNotFound, ErrUserNotFound)
	require.ErrorIs(t, fmt.Errorf("query user: %w", ErrUserNotFound), ErrUserNotFound)
	require.ErrorIs(t, ErrUserAlreadyExists, ErrUserAlreadyExists)
	require.ErrorIs(t, fmt.Errorf("create user: %w", ErrUserAlreadyExists), ErrUserAlreadyExists)

	require.False(t, errors.Is(ErrUserAlreadyExists, ErrUserNotFound))
	require.False(t, errors.Is(ErrUserNotFound, ErrUserAlreadyExists))
}
