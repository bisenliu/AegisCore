package query

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"
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

		if err != nil {
			t.Fatalf("GetUserByID: %v", err)
		}
		if user.User.UserID != testUserID || user.User.Username != "alice" || user.User.CreatedAt != createdAt {
			t.Fatalf("user = %#v", user)
		}
	})

	t.Run("map domain not found", func(t *testing.T) {
		repo := NewMockUserProfileStore(gomock.NewController(t))
		repo.EXPECT().GetByUserID(gomock.Any(), testUserID).Return(nil, identity.ErrUserNotFound)
		svc := NewUserQueryService(repo)

		_, err := svc.GetUserByID(context.Background(), GetUserByIDQuery{UserID: testUserID})

		if !errors.Is(err, identity.ErrUserNotFound) {
			t.Fatalf("err = %v, want ErrUserNotFound", err)
		}
	})

	t.Run("wrap repository error", func(t *testing.T) {
		repoErr := errors.New("database down")
		repo := NewMockUserProfileStore(gomock.NewController(t))
		repo.EXPECT().GetByUserID(gomock.Any(), testUserID).Return(nil, repoErr)
		svc := NewUserQueryService(repo)

		_, err := svc.GetUserByID(context.Background(), GetUserByIDQuery{UserID: testUserID})

		if err == nil || !errors.Is(err, repoErr) {
			t.Fatalf("err = %v, want %v", err, repoErr)
		}
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

		if err != nil {
			t.Fatalf("ListUsers: %v", err)
		}
		if listInput.AfterUserID != nil || listInput.Limit != 10 {
			t.Fatalf("listInput = %#v", listInput)
		}
		if len(users.Items) != 0 {
			t.Fatalf("items = %#v", users.Items)
		}
		if users.PageSize != 10 || users.NextCursor != "" || users.HasNext {
			t.Fatalf("pagination result = %#v", users)
		}
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

		if err != nil {
			t.Fatalf("ListUsers: %v", err)
		}
		if listInput.AfterUserID == nil || *listInput.AfterUserID != afterUserID || listInput.Limit != 20 || listInput.Nickname != "Ali" || listInput.Username != "alice" {
			t.Fatalf("listInput = %#v", listInput)
		}
		if listInput.Status == nil || *listInput.Status != identity.UserStatusNormal {
			t.Fatalf("status = %#v", listInput.Status)
		}
		if len(users.Items) != 1 || users.Items[0].UserID != testUserID || users.Items[0].Username != "alice" || users.Items[0].CreatedAt != createdAt {
			t.Fatalf("items = %#v", users.Items)
		}
		if users.PageSize != 20 || users.NextCursor != testUserID.String() || !users.HasNext {
			t.Fatalf("pagination result = %#v", users)
		}
	})

	t.Run("wrap repository error", func(t *testing.T) {
		repoErr := errors.New("database down")
		repo := NewMockUserProfileStore(gomock.NewController(t))
		repo.EXPECT().ListUsers(gomock.Any(), gomock.Any()).Return(nil, false, repoErr)
		svc := NewUserQueryService(repo)

		_, err := svc.ListUsers(context.Background(), ListUsersQuery{PageSize: 10, Limit: 10})

		if err == nil || !errors.Is(err, repoErr) {
			t.Fatalf("err = %v, want %v", err, repoErr)
		}
	})
}
