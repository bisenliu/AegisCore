package repository

import (
	"context"
	"fmt"

	"github.com/aegiscore/common/response"
	"github.com/aegiscore/user-services/ent"
	"github.com/aegiscore/user-services/ent/user"
	"github.com/aegiscore/user-services/internal/apperror"
	"go.uber.org/fx"
)

type UserRepository interface {
	Create(ctx context.Context, input CreateUserInput) (*ent.User, error)
	ExistsByEmail(ctx context.Context, email string) (bool, error)
	GetByID(ctx context.Context, id int64) (*ent.User, error)
}

type CreateUserInput struct {
	Name     string
	Email    string
	Password string
	Active   bool
}

type userRepository struct {
	client *ent.Client
}

type UserRepositoryParams struct {
	fx.In

	Client *ent.Client `name:"user_db"`
}

func NewUserRepository(params UserRepositoryParams) UserRepository {
	return &userRepository{client: params.Client}
}

func (r *userRepository) Create(ctx context.Context, input CreateUserInput) (*ent.User, error) {
	created, err := r.client.User.Create().
		SetName(input.Name).
		SetEmail(input.Email).
		SetPassword(input.Password).
		SetActive(input.Active).
		Save(ctx)
	if err == nil {
		return created, nil
	}
	if ent.IsConstraintError(err) {
		return nil, response.ConflictError(apperror.MsgUserAlreadyExists)
	}
	return nil, fmt.Errorf("create user email %s: %w", input.Email, err)
}

func (r *userRepository) ExistsByEmail(ctx context.Context, email string) (bool, error) {
	exists, err := r.client.User.Query().Where(user.EmailEQ(email)).Exist(ctx)
	if err != nil {
		return false, fmt.Errorf("check user email %s exists: %w", email, err)
	}
	return exists, nil
}

func (r *userRepository) GetByID(ctx context.Context, id int64) (*ent.User, error) {
	user, err := r.client.User.Get(ctx, id)
	if err == nil {
		return user, nil
	}
	if ent.IsNotFound(err) {
		return nil, response.NotFoundError(apperror.MsgUserNotFound)
	}
	return nil, fmt.Errorf("query user by id %d: %w", id, err)
}
