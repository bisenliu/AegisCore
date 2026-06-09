package user

import (
	"context"
	"errors"
	"testing"

	"github.com/aegiscore/common/contract/response"
	"github.com/aegiscore/common/security/password"
	"github.com/aegiscore/user-services/internal/messages"
	"github.com/google/uuid"
)

var testUserID = uuid.MustParse("018f6f3e-7c4d-7b2a-9f8a-4f6b1b2c3d4e")

func TestUserServiceCreateUser(t *testing.T) {
	createdAt := int64(1780048800000)

	t.Run("success uses normalized fields and defaults status", func(t *testing.T) {
		repo := &stubUserRepository{created: &User{ID: 123, UserID: testUserID, Nickname: "Alice", Username: "alice", Status: UserStatusNormal, TokenVersion: 1, CreatedAt: createdAt, UpdatedAt: createdAt}}
		svc := NewUserService(repo)

		user, err := svc.CreateUser(context.Background(), CreateUserCommand{Nickname: "Alice", Username: "alice", Password: "secret"})

		if err != nil {
			t.Fatalf("CreateUser: %v", err)
		}
		if repo.createdInput.Nickname != "Alice" || repo.createdInput.Username != "alice" || repo.createdInput.UserID == uuid.Nil || repo.createdInput.Status != UserStatusNormal {
			t.Fatalf("createdInput = %#v", repo.createdInput)
		}
		matched, err := password.Verify("secret", repo.createdInput.PasswordHash)
		if err != nil || !matched {
			t.Fatalf("created password was not hashed correctly: matched=%v err=%v", matched, err)
		}
		if user.UserID != testUserID.String() || user.Username != "alice" || user.CreatedAt != createdAt || user.UpdatedAt != createdAt {
			t.Fatalf("user = %#v", user)
		}
	})

	t.Run("preserve create conflict", func(t *testing.T) {
		svc := NewUserService(&stubUserRepository{createErr: response.ConflictError(messages.UserAlreadyExists)})

		_, err := svc.CreateUser(context.Background(), CreateUserCommand{Nickname: "Alice", Username: "alice", Password: "secret"})

		appErr := response.FromError(err)
		if appErr.Code != response.CodeConflict || appErr.Message != messages.UserAlreadyExists {
			t.Fatalf("err = %#v", appErr)
		}
	})

	t.Run("map domain create conflict", func(t *testing.T) {
		svc := NewUserService(&stubUserRepository{createErr: ErrUserAlreadyExists})

		_, err := svc.CreateUser(context.Background(), CreateUserCommand{Nickname: "Alice", Username: "alice", Password: "secret"})

		appErr := response.FromError(err)
		if appErr.Code != response.CodeConflict || appErr.Message != messages.UserAlreadyExists {
			t.Fatalf("err = %#v", appErr)
		}
	})

	t.Run("maps uppercase duplicate after normalization", func(t *testing.T) {
		repo := &stubUserRepository{createErr: ErrUserAlreadyExists}
		svc := NewUserService(repo)

		_, err := svc.CreateUser(context.Background(), CreateUserCommand{Nickname: "Alice", Username: "alice", Password: "secret"})

		if repo.createdInput.Username != "alice" {
			t.Fatalf("created username = %q", repo.createdInput.Username)
		}
		appErr := response.FromError(err)
		if appErr.Code != response.CodeConflict || appErr.Message != messages.UserAlreadyExists {
			t.Fatalf("err = %#v", appErr)
		}
	})
}

func TestUserServiceGetUserByID(t *testing.T) {
	createdAt := int64(1780048800000)

	t.Run("success", func(t *testing.T) {
		repo := &stubUserRepository{getByUserIDUser: &User{ID: 123, UserID: testUserID, Nickname: "Alice", Username: "alice", Status: UserStatusNormal, CreatedAt: createdAt, UpdatedAt: createdAt}}
		svc := NewUserService(repo)

		user, err := svc.GetUserByID(context.Background(), testUserID)

		if err != nil {
			t.Fatalf("GetUserByID: %v", err)
		}
		if repo.getByUserID != testUserID {
			t.Fatalf("getByUserID = %s", repo.getByUserID)
		}
		if user.UserID != testUserID.String() || user.Username != "alice" || user.CreatedAt != createdAt {
			t.Fatalf("user = %#v", user)
		}
	})

	t.Run("map domain not found", func(t *testing.T) {
		svc := NewUserService(&stubUserRepository{getByUserIDErr: ErrUserNotFound})

		_, err := svc.GetUserByID(context.Background(), testUserID)

		appErr := response.FromError(err)
		if appErr.Code != response.CodeNotFound || appErr.Message != messages.UserNotFound {
			t.Fatalf("err = %#v", appErr)
		}
	})

	t.Run("wrap repository error", func(t *testing.T) {
		svc := NewUserService(&stubUserRepository{getByUserIDErr: errors.New("database down")})

		_, err := svc.GetUserByID(context.Background(), testUserID)

		appErr := response.FromError(err)
		if appErr.Code != response.CodeInternalError || appErr.Message != response.MessageInternalError {
			t.Fatalf("err = %#v", appErr)
		}
	})
}

func TestUserServiceListUsers(t *testing.T) {
	createdAt := int64(1780048800000)

	t.Run("normalized default pagination returns empty page", func(t *testing.T) {
		repo := &stubUserRepository{}
		svc := NewUserService(repo)

		users, err := svc.ListUsers(context.Background(), ListUsersQuery{Page: 1, PageSize: 10, Limit: 10})

		if err != nil {
			t.Fatalf("ListUsers: %v", err)
		}
		if repo.listInput.Offset != 0 || repo.listInput.Limit != 10 {
			t.Fatalf("listInput = %#v", repo.listInput)
		}
		if users.Items == nil || len(users.Items) != 0 {
			t.Fatalf("items = %#v", users.Items)
		}
		if users.Pagination.Page != 1 || users.Pagination.PageSize != 10 || users.Pagination.Total != 0 || users.Pagination.TotalPages != 0 {
			t.Fatalf("pagination = %#v", users.Pagination)
		}
	})

	t.Run("explicit pagination and filters", func(t *testing.T) {
		status := UserStatusNormal
		repo := &stubUserRepository{listUsers: []User{{ID: 1, UserID: testUserID, Nickname: "Alice", Username: "alice", Status: UserStatusNormal, CreatedAt: createdAt, UpdatedAt: createdAt}}, listTotal: 128}
		svc := NewUserService(repo)

		users, err := svc.ListUsers(context.Background(), ListUsersQuery{Page: 2, PageSize: 20, Offset: 20, Limit: 20, Nickname: "Ali", Username: "alice", Status: &status})

		if err != nil {
			t.Fatalf("ListUsers: %v", err)
		}
		if repo.listInput.Offset != 20 || repo.listInput.Limit != 20 || repo.listInput.Nickname != "Ali" || repo.listInput.Username != "alice" {
			t.Fatalf("listInput = %#v", repo.listInput)
		}
		if repo.listInput.Status == nil || *repo.listInput.Status != UserStatusNormal {
			t.Fatalf("status = %#v", repo.listInput.Status)
		}
		if len(users.Items) != 1 || users.Items[0].UserID != testUserID.String() || users.Items[0].Username != "alice" || users.Items[0].CreatedAt != createdAt {
			t.Fatalf("items = %#v", users.Items)
		}
		if users.Pagination.Page != 2 || users.Pagination.PageSize != 20 || users.Pagination.Total != 128 || users.Pagination.TotalPages != 7 {
			t.Fatalf("pagination = %#v", users.Pagination)
		}
	})

	t.Run("wrap repository error", func(t *testing.T) {
		svc := NewUserService(&stubUserRepository{listErr: errors.New("database down")})

		_, err := svc.ListUsers(context.Background(), ListUsersQuery{Page: 1, PageSize: 10, Limit: 10})

		appErr := response.FromError(err)
		if appErr.Code != response.CodeInternalError || appErr.Message != response.MessageInternalError {
			t.Fatalf("err = %#v", appErr)
		}
	})
}

type stubUserRepository struct {
	created         *User
	createErr       error
	createdInput    CreateUserInput
	listUsers       []User
	listTotal       int
	listErr         error
	listInput       ListUsersInput
	getByUserID     uuid.UUID
	getByUserIDUser *User
	getByUserIDErr  error
}

func (r *stubUserRepository) Create(_ context.Context, input CreateUserInput) (*User, error) {
	r.createdInput = input
	if r.createErr != nil {
		return nil, r.createErr
	}
	return r.created, nil
}

func (r *stubUserRepository) GetByUserID(_ context.Context, userID uuid.UUID) (*User, error) {
	r.getByUserID = userID
	if r.getByUserIDErr != nil {
		return nil, r.getByUserIDErr
	}
	return r.getByUserIDUser, nil
}

func (r *stubUserRepository) ListUsers(_ context.Context, input ListUsersInput) ([]User, int, error) {
	r.listInput = input
	if r.listErr != nil {
		return nil, 0, r.listErr
	}
	return r.listUsers, r.listTotal, nil
}
