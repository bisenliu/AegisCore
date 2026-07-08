package query

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	userapplication "github.com/aegiscore/user-service/internal/features/user/application"
	userdomain "github.com/aegiscore/user-service/internal/features/user/domain"
	"github.com/aegiscore/user-service/internal/shared/identity"
)

var testUserID = uuid.MustParse("018f6f3e-7c4d-7b2a-9f8a-4f6b1b2c3d4e")

func TestUserQueryServiceGetUserByID(t *testing.T) {
	createdAt := int64(1780048800000)

	t.Run("success", func(t *testing.T) {
		repo := NewMockUserProfileStore(gomock.NewController(t))
		repo.EXPECT().GetByUserID(gomock.Any(), testUserID).Return(&userdomain.User{ID: 123, UserID: testUserID, Nickname: "Alice", Username: "alice", Status: identity.UserStatusNormal, CreatedAt: createdAt, UpdatedAt: createdAt}, nil)
		svc := NewUserQueryService(repo)

		user, err := svc.GetUserByID(context.Background(), GetUserByIDQuery{UserID: testUserID})

		require.NoError(t, err)
		require.Equal(t, testUserID, user.User.UserID)
		require.Equal(t, "alice", user.User.Username)
		require.Equal(t, createdAt, user.User.CreatedAt)
	})

	t.Run("map domain not found", func(t *testing.T) {
		repo := NewMockUserProfileStore(gomock.NewController(t))
		repo.EXPECT().GetByUserID(gomock.Any(), testUserID).Return(nil, identity.ErrUserNotFound)
		svc := NewUserQueryService(repo)

		_, err := svc.GetUserByID(context.Background(), GetUserByIDQuery{UserID: testUserID})

		require.ErrorIs(t, err, identity.ErrUserNotFound)
	})

	t.Run("map wrapped domain not found", func(t *testing.T) {
		repo := NewMockUserProfileStore(gomock.NewController(t))
		repo.EXPECT().GetByUserID(gomock.Any(), testUserID).Return(nil, fmt.Errorf("repository miss: %w", identity.ErrUserNotFound))
		svc := NewUserQueryService(repo)

		_, err := svc.GetUserByID(context.Background(), GetUserByIDQuery{UserID: testUserID})

		require.ErrorIs(t, err, identity.ErrUserNotFound)
	})

	t.Run("wrap repository error", func(t *testing.T) {
		repoErr := errors.New("database down")
		repo := NewMockUserProfileStore(gomock.NewController(t))
		repo.EXPECT().GetByUserID(gomock.Any(), testUserID).Return(nil, repoErr)
		svc := NewUserQueryService(repo)

		_, err := svc.GetUserByID(context.Background(), GetUserByIDQuery{UserID: testUserID})

		require.ErrorIs(t, err, repoErr)
	})
}

func TestUserQueryServiceListUsers(t *testing.T) {
	createdAt := int64(1780048800000)

	t.Run("normalized default pagination returns empty page", func(t *testing.T) {
		repo := NewMockUserProfileStore(gomock.NewController(t))
		var listInput userapplication.ListUsersInput
		repo.EXPECT().ListUsers(gomock.Any(), gomock.Any()).DoAndReturn(func(_ context.Context, input userapplication.ListUsersInput) ([]userdomain.User, bool, error) {
			listInput = input
			return nil, false, nil
		})
		svc := NewUserQueryService(repo)

		users, err := svc.ListUsers(context.Background(), ListUsersQuery{PageSize: 10, Limit: 10})

		require.NoError(t, err)
		require.Nil(t, listInput.AfterUserID)
		require.Equal(t, 10, listInput.Limit)
		require.Empty(t, users.Items)
		require.Equal(t, 10, users.PageSize)
		require.Empty(t, users.NextCursor)
		require.False(t, users.HasNext)
	})

	t.Run("explicit cursor pagination and filters", func(t *testing.T) {
		status := identity.UserStatusNormal
		repo := NewMockUserProfileStore(gomock.NewController(t))
		var listInput userapplication.ListUsersInput
		repo.EXPECT().ListUsers(gomock.Any(), gomock.Any()).DoAndReturn(func(_ context.Context, input userapplication.ListUsersInput) ([]userdomain.User, bool, error) {
			listInput = input
			return []userdomain.User{{ID: 1, UserID: testUserID, Nickname: "Alice", Username: "alice", Status: identity.UserStatusNormal, CreatedAt: createdAt, UpdatedAt: createdAt}}, true, nil
		})
		svc := NewUserQueryService(repo)
		afterUserID := uuid.MustParse("018f6f3e-7c4d-7b2a-9f8a-4f6b1b2c3d4d")

		users, err := svc.ListUsers(context.Background(), ListUsersQuery{Cursor: &afterUserID, PageSize: 20, Limit: 20, Nickname: "Ali", Username: "alice", Status: &status})

		require.NoError(t, err)
		require.NotNil(t, listInput.AfterUserID)
		require.Equal(t, afterUserID, *listInput.AfterUserID)
		require.Equal(t, 20, listInput.Limit)
		require.Equal(t, "Ali", listInput.Nickname)
		require.Equal(t, "alice", listInput.Username)
		require.NotNil(t, listInput.Status)
		require.Equal(t, identity.UserStatusNormal, *listInput.Status)
		require.Len(t, users.Items, 1)
		require.Equal(t, testUserID, users.Items[0].UserID)
		require.Equal(t, "alice", users.Items[0].Username)
		require.Equal(t, createdAt, users.Items[0].CreatedAt)
		require.Equal(t, 20, users.PageSize)
		require.Equal(t, testUserID.String(), users.NextCursor)
		require.True(t, users.HasNext)
	})

	t.Run("wrap repository error", func(t *testing.T) {
		repoErr := errors.New("database down")
		repo := NewMockUserProfileStore(gomock.NewController(t))
		repo.EXPECT().ListUsers(gomock.Any(), gomock.Any()).Return(nil, false, repoErr)
		svc := NewUserQueryService(repo)

		_, err := svc.ListUsers(context.Background(), ListUsersQuery{PageSize: 10, Limit: 10})

		require.ErrorIs(t, err, repoErr)
	})
}
