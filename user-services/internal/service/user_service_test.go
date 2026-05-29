package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/aegiscore/common/response"
	"github.com/aegiscore/user-services/ent"
	"github.com/aegiscore/user-services/internal/apperror"
	"github.com/aegiscore/user-services/internal/dto"
	"github.com/aegiscore/user-services/internal/repository"
)

func TestUserServiceCreateUser(t *testing.T) {
	createdAt := time.Date(2026, 5, 29, 10, 0, 0, 0, time.UTC)

	t.Run("success normalizes fields and defaults active", func(t *testing.T) {
		repo := &stubUserRepository{created: &ent.User{ID: 123, Name: "Alice", Email: "alice@example.com", Active: true, CreatedAt: createdAt, UpdatedAt: createdAt}}
		svc := NewUserService(repo)

		user, err := svc.CreateUser(context.Background(), dto.CreateUserRequest{Name: " Alice ", Email: "ALICE@EXAMPLE.COM"})

		if err != nil {
			t.Fatalf("CreateUser: %v", err)
		}
		if repo.checkedEmail != "alice@example.com" {
			t.Fatalf("checkedEmail = %q", repo.checkedEmail)
		}
		if repo.createdInput.Name != "Alice" || repo.createdInput.Email != "alice@example.com" || !repo.createdInput.Active {
			t.Fatalf("createdInput = %#v", repo.createdInput)
		}
		if user.ID != 123 || user.Email != "alice@example.com" {
			t.Fatalf("user = %#v", user)
		}
	})

	t.Run("reject blank trimmed name", func(t *testing.T) {
		svc := NewUserService(&stubUserRepository{})

		_, err := svc.CreateUser(context.Background(), dto.CreateUserRequest{Name: "   ", Email: "alice@example.com"})

		appErr := response.FromError(err)
		if appErr.Code != response.CodeValidationFailed || appErr.Message != apperror.MsgInvalidUserName {
			t.Fatalf("err = %#v", appErr)
		}
	})

	t.Run("reject existing email", func(t *testing.T) {
		svc := NewUserService(&stubUserRepository{exists: true})

		_, err := svc.CreateUser(context.Background(), dto.CreateUserRequest{Name: "Alice", Email: "alice@example.com"})

		appErr := response.FromError(err)
		if appErr.Code != response.CodeConflict || appErr.Message != apperror.MsgUserAlreadyExists {
			t.Fatalf("err = %#v", appErr)
		}
	})

	t.Run("wrap existence check error", func(t *testing.T) {
		svc := NewUserService(&stubUserRepository{existsErr: errors.New("database down")})

		_, err := svc.CreateUser(context.Background(), dto.CreateUserRequest{Name: "Alice", Email: "alice@example.com"})

		appErr := response.FromError(err)
		if appErr.Code != response.CodeInternalError || appErr.Message != "internal server error" {
			t.Fatalf("err = %#v", appErr)
		}
	})

	t.Run("preserve create conflict", func(t *testing.T) {
		svc := NewUserService(&stubUserRepository{createErr: response.ConflictError(apperror.MsgUserAlreadyExists)})

		_, err := svc.CreateUser(context.Background(), dto.CreateUserRequest{Name: "Alice", Email: "alice@example.com"})

		appErr := response.FromError(err)
		if appErr.Code != response.CodeConflict || appErr.Message != apperror.MsgUserAlreadyExists {
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
