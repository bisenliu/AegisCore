package service

import (
	"context"
	"errors"
	"testing"

	"github.com/aegiscore/common/password"
	"github.com/aegiscore/common/response"
	"github.com/aegiscore/user-services/ent"
	"github.com/aegiscore/user-services/internal/domain"
	"github.com/aegiscore/user-services/internal/dto"
	"github.com/aegiscore/user-services/internal/errmsg"
	"github.com/aegiscore/user-services/internal/repository"
	"github.com/google/uuid"
)

var testUserID = uuid.MustParse("018f6f3e-7c4d-7b2a-9f8a-4f6b1b2c3d4e")

func TestUserServiceCreateUser(t *testing.T) {
	createdAt := int64(1780048800000)

	t.Run("success normalizes fields and defaults status", func(t *testing.T) {
		repo := &stubUserRepository{created: &ent.User{ID: 123, UserID: testUserID, Nickname: "Alice", Username: "alice", Status: int64(domain.UserStatusNormal), TokenVersion: 1, CreatedAt: createdAt, UpdatedAt: createdAt}}
		svc := NewUserService(repo)

		user, err := svc.CreateUser(context.Background(), dto.CreateUserRequest{Nickname: " Alice ", Username: " alice ", Password: " secret "})

		if err != nil {
			t.Fatalf("CreateUser: %v", err)
		}
		if repo.checkedUsername != "alice" {
			t.Fatalf("checkedUsername = %q", repo.checkedUsername)
		}
		if repo.createdInput.Nickname != "Alice" || repo.createdInput.Username != "alice" || repo.createdInput.UserID == uuid.Nil || repo.createdInput.Status != domain.UserStatusNormal {
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

	t.Run("reject blank trimmed name", func(t *testing.T) {
		svc := NewUserService(&stubUserRepository{})

		_, err := svc.CreateUser(context.Background(), dto.CreateUserRequest{Nickname: "   ", Username: "alice", Password: "secret"})

		appErr := response.FromError(err)
		if appErr.Code != response.CodeValidationFailed || appErr.Message != errmsg.MsgInvalidUserName {
			t.Fatalf("err = %#v", appErr)
		}
	})

	t.Run("reject blank trimmed password", func(t *testing.T) {
		svc := NewUserService(&stubUserRepository{})

		_, err := svc.CreateUser(context.Background(), dto.CreateUserRequest{Nickname: "Alice", Username: "alice", Password: "   "})

		appErr := response.FromError(err)
		if appErr.Code != response.CodeValidationFailed || appErr.Message != errmsg.MsgInvalidPassword {
			t.Fatalf("err = %#v", appErr)
		}
	})

	t.Run("reject existing username", func(t *testing.T) {
		svc := NewUserService(&stubUserRepository{exists: true})

		_, err := svc.CreateUser(context.Background(), dto.CreateUserRequest{Nickname: "Alice", Username: "alice", Password: "secret"})

		appErr := response.FromError(err)
		if appErr.Code != response.CodeConflict || appErr.Message != errmsg.MsgUserAlreadyExists {
			t.Fatalf("err = %#v", appErr)
		}
	})

	t.Run("wrap existence check error", func(t *testing.T) {
		svc := NewUserService(&stubUserRepository{existsErr: errors.New("database down")})

		_, err := svc.CreateUser(context.Background(), dto.CreateUserRequest{Nickname: "Alice", Username: "alice", Password: "secret"})

		appErr := response.FromError(err)
		if appErr.Code != response.CodeInternalError || appErr.Message != "internal server error" {
			t.Fatalf("err = %#v", appErr)
		}
	})

	t.Run("preserve create conflict", func(t *testing.T) {
		svc := NewUserService(&stubUserRepository{createErr: response.ConflictError(errmsg.MsgUserAlreadyExists)})

		_, err := svc.CreateUser(context.Background(), dto.CreateUserRequest{Nickname: "Alice", Username: "alice", Password: "secret"})

		appErr := response.FromError(err)
		if appErr.Code != response.CodeConflict || appErr.Message != errmsg.MsgUserAlreadyExists {
			t.Fatalf("err = %#v", appErr)
		}
	})

	t.Run("map domain create conflict", func(t *testing.T) {
		svc := NewUserService(&stubUserRepository{createErr: domain.ErrUserAlreadyExists})

		_, err := svc.CreateUser(context.Background(), dto.CreateUserRequest{Nickname: "Alice", Username: "alice", Password: "secret"})

		appErr := response.FromError(err)
		if appErr.Code != response.CodeConflict || appErr.Message != errmsg.MsgUserAlreadyExists {
			t.Fatalf("err = %#v", appErr)
		}
	})
}

func TestUserServiceGetUserByID(t *testing.T) {
	createdAt := int64(1780048800000)

	t.Run("success", func(t *testing.T) {
		repo := &stubUserRepository{getByUserIDUser: &ent.User{ID: 123, UserID: testUserID, Nickname: "Alice", Username: "alice", Status: int64(domain.UserStatusNormal), CreatedAt: createdAt, UpdatedAt: createdAt}}
		svc := NewUserService(repo)

		user, err := svc.GetUserByID(context.Background(), testUserID.String())

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
		svc := NewUserService(&stubUserRepository{getByUserIDErr: domain.ErrUserNotFound})

		_, err := svc.GetUserByID(context.Background(), testUserID.String())

		appErr := response.FromError(err)
		if appErr.Code != response.CodeNotFound || appErr.Message != errmsg.MsgUserNotFound {
			t.Fatalf("err = %#v", appErr)
		}
	})

	t.Run("wrap repository error", func(t *testing.T) {
		svc := NewUserService(&stubUserRepository{getByUserIDErr: errors.New("database down")})

		_, err := svc.GetUserByID(context.Background(), testUserID.String())

		appErr := response.FromError(err)
		if appErr.Code != response.CodeInternalError || appErr.Message != response.MessageInternalError {
			t.Fatalf("err = %#v", appErr)
		}
	})
}

func TestUserServiceListUsers(t *testing.T) {
	createdAt := int64(1780048800000)

	t.Run("default pagination returns empty page", func(t *testing.T) {
		repo := &stubUserRepository{}
		svc := NewUserService(repo)

		users, err := svc.ListUsers(context.Background(), dto.ListUsersRequest{})

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
		status := domain.UserStatusNormal
		repo := &stubUserRepository{listUsers: []*ent.User{{ID: 1, UserID: testUserID, Nickname: "Alice", Username: "alice", Status: int64(domain.UserStatusNormal), CreatedAt: createdAt, UpdatedAt: createdAt}}, listTotal: 128}
		svc := NewUserService(repo)

		users, err := svc.ListUsers(context.Background(), dto.ListUsersRequest{Page: 2, PageSize: 20, Nickname: " Ali ", Username: " alice ", Status: &status})

		if err != nil {
			t.Fatalf("ListUsers: %v", err)
		}
		if repo.listInput.Offset != 20 || repo.listInput.Limit != 20 || repo.listInput.Nickname != "Ali" || repo.listInput.Username != "alice" {
			t.Fatalf("listInput = %#v", repo.listInput)
		}
		if repo.listInput.Status == nil || *repo.listInput.Status != domain.UserStatusNormal {
			t.Fatalf("status = %#v", repo.listInput.Status)
		}
		if len(users.Items) != 1 || users.Items[0].UserID != testUserID.String() || users.Items[0].Username != "alice" || users.Items[0].CreatedAt != createdAt {
			t.Fatalf("items = %#v", users.Items)
		}
		if users.Pagination.Page != 2 || users.Pagination.PageSize != 20 || users.Pagination.Total != 128 || users.Pagination.TotalPages != 7 {
			t.Fatalf("pagination = %#v", users.Pagination)
		}
	})

	t.Run("invalid pagination boundaries use defaults", func(t *testing.T) {
		repo := &stubUserRepository{}
		svc := NewUserService(repo)

		_, err := svc.ListUsers(context.Background(), dto.ListUsersRequest{Page: -1, PageSize: 0})

		if err != nil {
			t.Fatalf("ListUsers: %v", err)
		}
		if repo.listInput.Offset != 0 || repo.listInput.Limit != 10 {
			t.Fatalf("listInput = %#v", repo.listInput)
		}
	})

	t.Run("wrap repository error", func(t *testing.T) {
		svc := NewUserService(&stubUserRepository{listErr: errors.New("database down")})

		_, err := svc.ListUsers(context.Background(), dto.ListUsersRequest{})

		appErr := response.FromError(err)
		if appErr.Code != response.CodeInternalError || appErr.Message != response.MessageInternalError {
			t.Fatalf("err = %#v", appErr)
		}
	})
}

type stubUserRepository struct {
	created         *ent.User
	createErr       error
	createdInput    repository.CreateUserInput
	exists          bool
	existsErr       error
	checkedUsername string
	listUsers       []*ent.User
	listTotal       int
	listErr         error
	listInput       repository.ListUsersInput
	getByUserID     uuid.UUID
	getByUserIDUser *ent.User
	getByUserIDErr  error
}

func (r *stubUserRepository) Create(_ context.Context, input repository.CreateUserInput) (*ent.User, error) {
	r.createdInput = input
	if r.createErr != nil {
		return nil, r.createErr
	}
	return r.created, nil
}

func (r *stubUserRepository) ExistsByUsername(_ context.Context, username string) (bool, error) {
	r.checkedUsername = username
	if r.existsErr != nil {
		return false, r.existsErr
	}
	return r.exists, nil
}

func (r *stubUserRepository) GetByUserID(_ context.Context, userID uuid.UUID) (*ent.User, error) {
	r.getByUserID = userID
	if r.getByUserIDErr != nil {
		return nil, r.getByUserIDErr
	}
	return r.getByUserIDUser, nil
}

func (r *stubUserRepository) GetByUsername(context.Context, string) (*ent.User, error) {
	return nil, nil
}

func (r *stubUserRepository) GetTokenVersion(context.Context, uuid.UUID) (int64, error) {
	return 0, nil
}

func (r *stubUserRepository) IncrementTokenVersion(context.Context, uuid.UUID) (int64, error) {
	return 0, nil
}

func (r *stubUserRepository) UpdateCredentials(context.Context, repository.UpdateCredentialsInput) (int64, error) {
	return 0, nil
}

func (r *stubUserRepository) ListUsers(_ context.Context, input repository.ListUsersInput) ([]*ent.User, int, error) {
	r.listInput = input
	if r.listErr != nil {
		return nil, 0, r.listErr
	}
	return r.listUsers, r.listTotal, nil
}
