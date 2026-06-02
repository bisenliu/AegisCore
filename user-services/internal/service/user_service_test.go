package service

import (
	"context"
	"errors"
	"testing"

	"github.com/aegiscore/common/response"
	"github.com/aegiscore/user-services/ent"
	"github.com/aegiscore/user-services/internal/apperror"
	"github.com/aegiscore/user-services/internal/dto"
	"github.com/aegiscore/user-services/internal/repository"
)

func TestUserServiceCreateUser(t *testing.T) {
	createdAt := int64(1780048800000)

	t.Run("success normalizes fields and defaults active", func(t *testing.T) {
		repo := &stubUserRepository{created: &ent.User{ID: 123, Name: "Alice", Email: "alice@example.com", Active: true, CreatedAt: createdAt, UpdatedAt: createdAt}}
		svc := NewUserService(repo)

		user, err := svc.CreateUser(context.Background(), dto.CreateUserRequest{Name: " Alice ", Email: "ALICE@EXAMPLE.COM", Password: " secret "})

		if err != nil {
			t.Fatalf("CreateUser: %v", err)
		}
		if repo.checkedEmail != "alice@example.com" {
			t.Fatalf("checkedEmail = %q", repo.checkedEmail)
		}
		if repo.createdInput.Name != "Alice" || repo.createdInput.Email != "alice@example.com" || repo.createdInput.Password != "secret" || !repo.createdInput.Active {
			t.Fatalf("createdInput = %#v", repo.createdInput)
		}
		if user.ID != 123 || user.Email != "alice@example.com" || user.CreatedAt != createdAt || user.UpdatedAt != createdAt {
			t.Fatalf("user = %#v", user)
		}
	})

	t.Run("reject blank trimmed name", func(t *testing.T) {
		svc := NewUserService(&stubUserRepository{})

		_, err := svc.CreateUser(context.Background(), dto.CreateUserRequest{Name: "   ", Email: "alice@example.com", Password: "secret"})

		appErr := response.FromError(err)
		if appErr.Code != response.CodeValidationFailed || appErr.Message != apperror.MsgInvalidUserName {
			t.Fatalf("err = %#v", appErr)
		}
	})

	t.Run("reject blank trimmed password", func(t *testing.T) {
		svc := NewUserService(&stubUserRepository{})

		_, err := svc.CreateUser(context.Background(), dto.CreateUserRequest{Name: "Alice", Email: "alice@example.com", Password: "   "})

		appErr := response.FromError(err)
		if appErr.Code != response.CodeValidationFailed || appErr.Message != apperror.MsgInvalidPassword {
			t.Fatalf("err = %#v", appErr)
		}
	})

	t.Run("reject existing email", func(t *testing.T) {
		svc := NewUserService(&stubUserRepository{exists: true})

		_, err := svc.CreateUser(context.Background(), dto.CreateUserRequest{Name: "Alice", Email: "alice@example.com", Password: "secret"})

		appErr := response.FromError(err)
		if appErr.Code != response.CodeConflict || appErr.Message != apperror.MsgUserAlreadyExists {
			t.Fatalf("err = %#v", appErr)
		}
	})

	t.Run("wrap existence check error", func(t *testing.T) {
		svc := NewUserService(&stubUserRepository{existsErr: errors.New("database down")})

		_, err := svc.CreateUser(context.Background(), dto.CreateUserRequest{Name: "Alice", Email: "alice@example.com", Password: "secret"})

		appErr := response.FromError(err)
		if appErr.Code != response.CodeInternalError || appErr.Message != "internal server error" {
			t.Fatalf("err = %#v", appErr)
		}
	})

	t.Run("preserve create conflict", func(t *testing.T) {
		svc := NewUserService(&stubUserRepository{createErr: response.ConflictError(apperror.MsgUserAlreadyExists)})

		_, err := svc.CreateUser(context.Background(), dto.CreateUserRequest{Name: "Alice", Email: "alice@example.com", Password: "secret"})

		appErr := response.FromError(err)
		if appErr.Code != response.CodeConflict || appErr.Message != apperror.MsgUserAlreadyExists {
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
		active := true
		repo := &stubUserRepository{listUsers: []*ent.User{{ID: 1, Name: "Alice", Email: "alice@example.com", Active: true, CreatedAt: createdAt, UpdatedAt: createdAt}}, listTotal: 128}
		svc := NewUserService(repo)

		users, err := svc.ListUsers(context.Background(), dto.ListUsersRequest{Page: 2, PageSize: 20, Name: " Ali ", Email: " ALICE@EXAMPLE.COM ", Active: &active})

		if err != nil {
			t.Fatalf("ListUsers: %v", err)
		}
		if repo.listInput.Offset != 20 || repo.listInput.Limit != 20 || repo.listInput.Name != "Ali" || repo.listInput.Email != "alice@example.com" {
			t.Fatalf("listInput = %#v", repo.listInput)
		}
		if repo.listInput.Active == nil || !*repo.listInput.Active {
			t.Fatalf("active = %#v", repo.listInput.Active)
		}
		if len(users.Items) != 1 || users.Items[0].ID != 1 || users.Items[0].Email != "alice@example.com" || users.Items[0].CreatedAt != createdAt {
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
	created      *ent.User
	createErr    error
	createdInput repository.CreateUserInput
	exists       bool
	existsErr    error
	checkedEmail string
	listUsers    []*ent.User
	listTotal    int
	listErr      error
	listInput    repository.ListUsersInput
}

func (r *stubUserRepository) Create(_ context.Context, input repository.CreateUserInput) (*ent.User, error) {
	r.createdInput = input
	if r.createErr != nil {
		return nil, r.createErr
	}
	return r.created, nil
}

func (r *stubUserRepository) ExistsByEmail(_ context.Context, email string) (bool, error) {
	r.checkedEmail = email
	if r.existsErr != nil {
		return false, r.existsErr
	}
	return r.exists, nil
}

func (r *stubUserRepository) GetByID(context.Context, int64) (*ent.User, error) {
	return nil, nil
}

func (r *stubUserRepository) ListUsers(_ context.Context, input repository.ListUsersInput) ([]*ent.User, int, error) {
	r.listInput = input
	if r.listErr != nil {
		return nil, 0, r.listErr
	}
	return r.listUsers, r.listTotal, nil
}
