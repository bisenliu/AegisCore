package domain

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/require"

	contracterrors "github.com/aegiscore/common/contract/errors"
	"github.com/aegiscore/user-service/internal/messages"
)

func TestRoleDomainErrorsAreApplicationErrors(t *testing.T) {
	tests := []struct {
		name       string
		err        *contracterrors.Error
		wantKind   contracterrors.Kind
		wantReason contracterrors.Reason
		wantCode   contracterrors.Code
		wantMsg    string
	}{
		{name: "role not found", err: ErrRoleNotFound, wantKind: contracterrors.KindNotFound, wantReason: reasonRoleNotFound, wantCode: contracterrors.CodeNotFound, wantMsg: messages.RoleNotFound},
		{name: "role already exists", err: ErrRoleAlreadyExists, wantKind: contracterrors.KindConflict, wantReason: reasonRoleAlreadyExists, wantCode: contracterrors.CodeConflict, wantMsg: messages.RoleAlreadyExists},
		{name: "role invalid", err: ErrRoleInvalid, wantKind: contracterrors.KindValidation, wantReason: reasonRoleInvalid, wantCode: contracterrors.CodeValidationFailed, wantMsg: messages.InvalidRole},
		{name: "system role protected", err: ErrSystemRoleProtected, wantKind: contracterrors.KindConflict, wantReason: reasonSystemRoleProtected, wantCode: contracterrors.CodeConflict, wantMsg: messages.SystemRoleProtected},
		{name: "role inactive", err: ErrRoleInactive, wantKind: contracterrors.KindConflict, wantReason: reasonRoleInactive, wantCode: contracterrors.CodeConflict, wantMsg: messages.RoleInactive},
		{name: "user role already exists", err: ErrUserRoleAlreadyExists, wantKind: contracterrors.KindConflict, wantReason: reasonUserRoleAlreadyExists, wantCode: contracterrors.CodeConflict, wantMsg: messages.UserRoleAlreadyExists},
		{name: "user role not found", err: ErrUserRoleNotFound, wantKind: contracterrors.KindNotFound, wantReason: reasonUserRoleNotFound, wantCode: contracterrors.CodeNotFound, wantMsg: messages.UserRoleNotFound},
		{name: "role permission already exists", err: ErrRolePermissionAlreadyExists, wantKind: contracterrors.KindConflict, wantReason: reasonRolePermissionAlreadyExists, wantCode: contracterrors.CodeConflict, wantMsg: messages.RolePermissionAlreadyExists},
		{name: "role permission not found", err: ErrRolePermissionNotFound, wantKind: contracterrors.KindNotFound, wantReason: reasonRolePermissionNotFound, wantCode: contracterrors.CodeNotFound, wantMsg: messages.RolePermissionNotFound},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require.Equal(t, tt.wantKind, tt.err.Kind)
			require.Equal(t, tt.wantReason, tt.err.Reason)
			require.Equal(t, tt.wantCode, tt.err.Code)
			require.Equal(t, tt.wantMsg, tt.err.Message)
			require.Same(t, tt.err, contracterrors.FromError(tt.err))

			wrapped := errors.Join(errors.New("context"), tt.err)
			require.ErrorIs(t, wrapped, tt.err)
			require.Same(t, tt.err, contracterrors.FromError(wrapped))
		})
	}
}

func TestRoleDomainErrorReasonsDoNotCrossMatchSameKind(t *testing.T) {
	require.NotErrorIs(t, ErrRoleNotFound, ErrUserRoleNotFound)
	require.NotErrorIs(t, ErrUserRoleNotFound, ErrRolePermissionNotFound)
	require.NotErrorIs(t, ErrRoleAlreadyExists, ErrUserRoleAlreadyExists)
	require.NotErrorIs(t, ErrUserRoleAlreadyExists, ErrRolePermissionAlreadyExists)
}
