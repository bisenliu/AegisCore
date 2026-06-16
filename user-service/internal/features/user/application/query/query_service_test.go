package query

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"

	userapplication "github.com/aegiscore/user-service/internal/features/user/application"
	userdomain "github.com/aegiscore/user-service/internal/features/user/domain"
	"github.com/aegiscore/user-service/internal/shared/identity"
)

var testUserID = uuid.MustParse("018f6f3e-7c4d-7b2a-9f8a-4f6b1b2c3d4e")

func TestUserQueryServiceGetUserByID(t *testing.T) {
	createdAt := int64(1780048800000)

	t.Run("success", func(t *testing.T) {
		repo := &stubUserRepository{getByUserIDUser: &userdomain.User{ID: 123, UserID: testUserID, Nickname: "Alice", Username: "alice", Status: identity.UserStatusNormal, CreatedAt: createdAt, UpdatedAt: createdAt}}
		svc := NewUserQueryService(repo)

		user, err := svc.GetUserByID(context.Background(), GetUserByIDQuery{UserID: testUserID})

		if err != nil {
			t.Fatalf("GetUserByID: %v", err)
		}
		if repo.getByUserID != testUserID {
			t.Fatalf("getByUserID = %s", repo.getByUserID)
		}
		if user.User.UserID != testUserID || user.User.Username != "alice" || user.User.CreatedAt != createdAt {
			t.Fatalf("user = %#v", user)
		}
	})

	t.Run("map domain not found", func(t *testing.T) {
		svc := NewUserQueryService(&stubUserRepository{getByUserIDErr: identity.ErrUserNotFound})

		_, err := svc.GetUserByID(context.Background(), GetUserByIDQuery{UserID: testUserID})

		if !errors.Is(err, identity.ErrUserNotFound) {
			t.Fatalf("err = %v, want ErrUserNotFound", err)
		}
	})

	t.Run("wrap repository error", func(t *testing.T) {
		repoErr := errors.New("database down")
		svc := NewUserQueryService(&stubUserRepository{getByUserIDErr: repoErr})

		_, err := svc.GetUserByID(context.Background(), GetUserByIDQuery{UserID: testUserID})

		if err == nil || !errors.Is(err, repoErr) {
			t.Fatalf("err = %v, want %v", err, repoErr)
		}
	})
}

func TestUserQueryServiceListUsers(t *testing.T) {
	createdAt := int64(1780048800000)

	t.Run("normalized default pagination returns empty page", func(t *testing.T) {
		repo := &stubUserRepository{}
		svc := NewUserQueryService(repo)

		users, err := svc.ListUsers(context.Background(), ListUsersQuery{PageSize: 10, Limit: 10})

		if err != nil {
			t.Fatalf("ListUsers: %v", err)
		}
		if repo.listInput.AfterUserID != nil || repo.listInput.Limit != 10 {
			t.Fatalf("listInput = %#v", repo.listInput)
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
		repo := &stubUserRepository{listUsers: []userdomain.User{{ID: 1, UserID: testUserID, Nickname: "Alice", Username: "alice", Status: identity.UserStatusNormal, CreatedAt: createdAt, UpdatedAt: createdAt}}, listHasNext: true}
		svc := NewUserQueryService(repo)
		afterUserID := uuid.MustParse("018f6f3e-7c4d-7b2a-9f8a-4f6b1b2c3d4d")

		users, err := svc.ListUsers(context.Background(), ListUsersQuery{Cursor: &afterUserID, PageSize: 20, Limit: 20, Nickname: "Ali", Username: "alice", Status: &status})

		if err != nil {
			t.Fatalf("ListUsers: %v", err)
		}
		if repo.listInput.AfterUserID == nil || *repo.listInput.AfterUserID != afterUserID || repo.listInput.Limit != 20 || repo.listInput.Nickname != "Ali" || repo.listInput.Username != "alice" {
			t.Fatalf("listInput = %#v", repo.listInput)
		}
		if repo.listInput.Status == nil || *repo.listInput.Status != identity.UserStatusNormal {
			t.Fatalf("status = %#v", repo.listInput.Status)
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
		svc := NewUserQueryService(&stubUserRepository{listErr: repoErr})

		_, err := svc.ListUsers(context.Background(), ListUsersQuery{PageSize: 10, Limit: 10})

		if err == nil || !errors.Is(err, repoErr) {
			t.Fatalf("err = %v, want %v", err, repoErr)
		}
	})
}

type stubUserRepository struct {
	listUsers       []userdomain.User
	listHasNext     bool
	listErr         error
	listInput       userapplication.ListUsersInput
	getByUserID     uuid.UUID
	getByUserIDUser *userdomain.User
	getByUserIDErr  error
}

func (r *stubUserRepository) Create(context.Context, userapplication.CreateUserInput) (*userdomain.User, error) {
	return nil, nil
}

func (r *stubUserRepository) GetByUserID(_ context.Context, userID uuid.UUID) (*userdomain.User, error) {
	r.getByUserID = userID
	if r.getByUserIDErr != nil {
		return nil, r.getByUserIDErr
	}
	return r.getByUserIDUser, nil
}

func (r *stubUserRepository) ListUsers(_ context.Context, input userapplication.ListUsersInput) ([]userdomain.User, bool, error) {
	r.listInput = input
	if r.listErr != nil {
		return nil, false, r.listErr
	}
	return r.listUsers, r.listHasNext, nil
}
