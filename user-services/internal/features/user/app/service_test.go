package app

import (
	"context"
	"errors"
	"testing"

	"github.com/aegiscore/common/security/password"
	userdomain "github.com/aegiscore/user-services/internal/features/user/domain"
	"github.com/google/uuid"
)

var testUserID = uuid.MustParse("018f6f3e-7c4d-7b2a-9f8a-4f6b1b2c3d4e")

func TestUserServiceCreateUser(t *testing.T) {
	createdAt := int64(1780048800000)

	t.Run("success uses normalized fields and defaults status", func(t *testing.T) {
		repo := &stubUserRepository{created: &userdomain.User{ID: 123, UserID: testUserID, Nickname: "Alice", Username: "alice", Status: userdomain.UserStatusNormal, TokenVersion: 1, CreatedAt: createdAt, UpdatedAt: createdAt}}
		svc := NewUserService(repo)

		user, err := svc.CreateUser(context.Background(), CreateUserCommand{Nickname: "Alice", Username: "alice", Password: "secret"})

		if err != nil {
			t.Fatalf("CreateUser: %v", err)
		}
		if repo.createdInput.Nickname != "Alice" || repo.createdInput.Username != "alice" || repo.createdInput.UserID == uuid.Nil || repo.createdInput.Status != userdomain.UserStatusNormal {
			t.Fatalf("createdInput = %#v", repo.createdInput)
		}
		matched, err := password.VerifyContext(context.Background(), "secret", repo.createdInput.PasswordHash)
		if err != nil || !matched {
			t.Fatalf("created password was not hashed correctly: matched=%v err=%v", matched, err)
		}
		if user.User.UserID != testUserID || user.User.Username != "alice" || user.User.CreatedAt != createdAt || user.User.UpdatedAt != createdAt {
			t.Fatalf("user = %#v", user)
		}
	})

	t.Run("map domain create conflict", func(t *testing.T) {
		svc := NewUserService(&stubUserRepository{createErr: userdomain.ErrUserAlreadyExists})

		_, err := svc.CreateUser(context.Background(), CreateUserCommand{Nickname: "Alice", Username: "alice", Password: "secret"})

		if !errors.Is(err, userdomain.ErrUserAlreadyExists) {
			t.Fatalf("err = %v, want ErrUserAlreadyExists", err)
		}
	})

	t.Run("maps uppercase duplicate after normalization", func(t *testing.T) {
		repo := &stubUserRepository{createErr: userdomain.ErrUserAlreadyExists}
		svc := NewUserService(repo)

		_, err := svc.CreateUser(context.Background(), CreateUserCommand{Nickname: "Alice", Username: "alice", Password: "secret"})

		if repo.createdInput.Username != "alice" {
			t.Fatalf("created username = %q", repo.createdInput.Username)
		}
		if !errors.Is(err, userdomain.ErrUserAlreadyExists) {
			t.Fatalf("err = %v, want ErrUserAlreadyExists", err)
		}
	})
}

func TestUserServiceGetUserByID(t *testing.T) {
	createdAt := int64(1780048800000)

	t.Run("success", func(t *testing.T) {
		repo := &stubUserRepository{getByUserIDUser: &userdomain.User{ID: 123, UserID: testUserID, Nickname: "Alice", Username: "alice", Status: userdomain.UserStatusNormal, CreatedAt: createdAt, UpdatedAt: createdAt}}
		svc := NewUserService(repo)

		user, err := svc.GetUserByID(context.Background(), testUserID)

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
		svc := NewUserService(&stubUserRepository{getByUserIDErr: userdomain.ErrUserNotFound})

		_, err := svc.GetUserByID(context.Background(), testUserID)

		if !errors.Is(err, userdomain.ErrUserNotFound) {
			t.Fatalf("err = %v, want ErrUserNotFound", err)
		}
	})

	t.Run("wrap repository error", func(t *testing.T) {
		repoErr := errors.New("database down")
		svc := NewUserService(&stubUserRepository{getByUserIDErr: repoErr})

		_, err := svc.GetUserByID(context.Background(), testUserID)

		if err == nil || !errors.Is(err, repoErr) {
			t.Fatalf("err = %v, want %v", err, repoErr)
		}
	})
}

func TestUserServiceListUsers(t *testing.T) {
	createdAt := int64(1780048800000)

	t.Run("normalized default pagination returns empty page", func(t *testing.T) {
		repo := &stubUserRepository{}
		svc := NewUserService(repo)

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
		status := userdomain.UserStatusNormal
		repo := &stubUserRepository{listUsers: []userdomain.User{{ID: 1, UserID: testUserID, Nickname: "Alice", Username: "alice", Status: userdomain.UserStatusNormal, CreatedAt: createdAt, UpdatedAt: createdAt}}, listHasNext: true}
		svc := NewUserService(repo)
		afterUserID := uuid.MustParse("018f6f3e-7c4d-7b2a-9f8a-4f6b1b2c3d4d")

		users, err := svc.ListUsers(context.Background(), ListUsersQuery{Cursor: &afterUserID, PageSize: 20, Limit: 20, Nickname: "Ali", Username: "alice", Status: &status})

		if err != nil {
			t.Fatalf("ListUsers: %v", err)
		}
		if repo.listInput.AfterUserID == nil || *repo.listInput.AfterUserID != afterUserID || repo.listInput.Limit != 20 || repo.listInput.Nickname != "Ali" || repo.listInput.Username != "alice" {
			t.Fatalf("listInput = %#v", repo.listInput)
		}
		if repo.listInput.Status == nil || *repo.listInput.Status != userdomain.UserStatusNormal {
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
		svc := NewUserService(&stubUserRepository{listErr: repoErr})

		_, err := svc.ListUsers(context.Background(), ListUsersQuery{PageSize: 10, Limit: 10})

		if err == nil || !errors.Is(err, repoErr) {
			t.Fatalf("err = %v, want %v", err, repoErr)
		}
	})
}

type stubUserRepository struct {
	created         *userdomain.User
	createErr       error
	createdInput    CreateUserInput
	listUsers       []userdomain.User
	listHasNext     bool
	listErr         error
	listInput       ListUsersInput
	getByUserID     uuid.UUID
	getByUserIDUser *userdomain.User
	getByUserIDErr  error
}

func (r *stubUserRepository) Create(_ context.Context, input CreateUserInput) (*userdomain.User, error) {
	r.createdInput = input
	if r.createErr != nil {
		return nil, r.createErr
	}
	return r.created, nil
}

func (r *stubUserRepository) GetByUserID(_ context.Context, userID uuid.UUID) (*userdomain.User, error) {
	r.getByUserID = userID
	if r.getByUserIDErr != nil {
		return nil, r.getByUserIDErr
	}
	return r.getByUserIDUser, nil
}

func (r *stubUserRepository) ListUsers(_ context.Context, input ListUsersInput) ([]userdomain.User, bool, error) {
	r.listInput = input
	if r.listErr != nil {
		return nil, false, r.listErr
	}
	return r.listUsers, r.listHasNext, nil
}
