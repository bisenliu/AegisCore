package userhttp

import (
	"testing"

	"github.com/stretchr/testify/require"

	contracterrors "github.com/aegiscore/common/contract/errors"
	"github.com/aegiscore/user-service/internal/messages"
	"github.com/aegiscore/user-service/internal/shared/identity"
)

func TestPrepareListUsersQuery(t *testing.T) {
	status := identity.UserStatusNormal
	query, err := prepareListUsersQuery(ListUsersRequest{
		Cursor:   " 018f6f3e-7c4d-7b2a-9f8a-4f6b1b2c3d4e ",
		PageSize: 101,
		Nickname: " Alice ",
		Username: " alice ",
		Status:   &status,
	})
	require.NoError(t, err)
	require.NotNil(t, query.Cursor)
	require.Equal(t, "018f6f3e-7c4d-7b2a-9f8a-4f6b1b2c3d4e", query.Cursor.String())
	require.Equal(t, 100, query.PageSize)
	require.Equal(t, 100, query.Limit)
	require.Equal(t, "Alice", query.Nickname)
	require.Equal(t, "alice", query.Username)
	require.NotNil(t, query.Status)
	require.Equal(t, status, *query.Status)

	_, err = prepareListUsersQuery(ListUsersRequest{Cursor: "bad"})
	appErr := contracterrors.FromError(err)
	require.Equal(t, contracterrors.CodeBadRequest, appErr.Code)
	require.Equal(t, messages.InvalidUserID, appErr.Message)
}

func TestPrepareCreateUserCommand(t *testing.T) {
	cmd, err := prepareCreateUserCommand(CreateUserRequest{Nickname: " Alice ", Username: " ALICE ", Password: " secret "})
	require.NoError(t, err)
	require.Equal(t, "Alice", cmd.Nickname)
	require.Equal(t, "alice", cmd.Username)
	require.Equal(t, "secret", cmd.Password)

	_, err = prepareCreateUserCommand(CreateUserRequest{Nickname: "Alice", Username: "alice", Password: " "})
	appErr := contracterrors.FromError(err)
	require.Equal(t, contracterrors.CodeValidationFailed, appErr.Code)
	require.Equal(t, messages.InvalidPassword, appErr.Message)
}
