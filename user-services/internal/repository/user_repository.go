package repository

import (
	"context"
	"fmt"

	"github.com/aegiscore/common/response"
	"github.com/aegiscore/user-services/ent"
	"go.uber.org/fx"
)

type UserRepository interface {
	GetByID(ctx context.Context, id int64) (*ent.User, error)
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

func (r *userRepository) GetByID(ctx context.Context, id int64) (*ent.User, error) {
	user, err := r.client.User.Get(ctx, id)
	if err == nil {
		return user, nil
	}
	if ent.IsNotFound(err) {
		return nil, response.NotFoundError("user not found")
	}
	return nil, fmt.Errorf("query user by id %d: %w", id, err)
}
