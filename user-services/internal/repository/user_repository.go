package repository

import (
	"context"
	"fmt"

	"github.com/aegiscore/common/response"
	"github.com/aegiscore/user-services/ent"
	"github.com/aegiscore/user-services/ent/predicate"
	"github.com/aegiscore/user-services/ent/user"
	"github.com/aegiscore/user-services/internal/apperror"
	"github.com/aegiscore/user-services/internal/domain"
	"github.com/google/uuid"
	"go.uber.org/fx"
)

type UserRepository interface {
	Create(ctx context.Context, input CreateUserInput) (*ent.User, error)
	ExistsByUsername(ctx context.Context, username string) (bool, error)
	GetByUsername(ctx context.Context, username string) (*ent.User, error)
	GetByUserID(ctx context.Context, userID uuid.UUID) (*ent.User, error)
	GetTokenVersion(ctx context.Context, userID uuid.UUID) (int64, error)
	IncrementTokenVersion(ctx context.Context, userID uuid.UUID) (int64, error)
	UpdatePasswordHashAndStatus(ctx context.Context, userID uuid.UUID, passwordHash string, status domain.UserStatus) (int64, error)
	ListUsers(ctx context.Context, input ListUsersInput) ([]*ent.User, int, error)
}

type CreateUserInput struct {
	Nickname     string
	UserID       uuid.UUID
	Username     string
	PasswordHash string
	Status       domain.UserStatus
}

type ListUsersInput struct {
	Offset   int
	Limit    int
	Nickname string
	Username string
	Status   *domain.UserStatus
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
		SetUserID(input.UserID).
		SetNickname(input.Nickname).
		SetUsername(input.Username).
		SetPasswordHash(input.PasswordHash).
		SetStatus(int64(input.Status)).
		Save(ctx)
	if err == nil {
		return created, nil
	}
	if ent.IsConstraintError(err) {
		return nil, response.ConflictError(apperror.MsgUserAlreadyExists)
	}
	return nil, fmt.Errorf("create user username %s: %w", input.Username, err)
}

func (r *userRepository) ExistsByUsername(ctx context.Context, username string) (bool, error) {
	exists, err := r.client.User.Query().Where(user.UsernameEQ(username), user.DeletedAtIsNil()).Exist(ctx)
	if err != nil {
		return false, fmt.Errorf("check username %s exists: %w", username, err)
	}
	return exists, nil
}

func (r *userRepository) GetByUserID(ctx context.Context, userID uuid.UUID) (*ent.User, error) {
	user, err := r.client.User.Query().Where(user.UserIDEQ(userID), user.DeletedAtIsNil()).Only(ctx)
	if err == nil {
		return user, nil
	}
	if ent.IsNotFound(err) {
		return nil, response.NotFoundError(apperror.MsgUserNotFound)
	}
	return nil, fmt.Errorf("query user by user_id %s: %w", userID.String(), err)
}

func (r *userRepository) GetByUsername(ctx context.Context, username string) (*ent.User, error) {
	user, err := r.client.User.Query().Where(user.UsernameEQ(username), user.DeletedAtIsNil()).Only(ctx)
	if err == nil {
		return user, nil
	}
	if ent.IsNotFound(err) {
		return nil, response.NotFoundError(apperror.MsgUserNotFound)
	}
	return nil, fmt.Errorf("query user by username %s: %w", username, err)
}

func (r *userRepository) GetTokenVersion(ctx context.Context, userID uuid.UUID) (int64, error) {
	user, err := r.client.User.Query().Where(user.UserIDEQ(userID), user.DeletedAtIsNil()).Only(ctx)
	if err == nil {
		return user.TokenVersion, nil
	}
	if ent.IsNotFound(err) {
		return 0, response.NotFoundError(apperror.MsgUserNotFound)
	}
	return 0, fmt.Errorf("query user token version by user_id %s: %w", userID.String(), err)
}

func (r *userRepository) IncrementTokenVersion(ctx context.Context, userID uuid.UUID) (int64, error) {
	updated, err := r.client.User.Update().Where(user.UserIDEQ(userID), user.DeletedAtIsNil()).AddTokenVersion(1).Save(ctx)
	if err == nil {
		if updated == 0 {
			return 0, response.NotFoundError(apperror.MsgUserNotFound)
		}
		user, err := r.GetByUserID(ctx, userID)
		if err != nil {
			return 0, err
		}
		return user.TokenVersion, nil
	}
	return 0, fmt.Errorf("increment user token version by user_id %s: %w", userID.String(), err)
}

func (r *userRepository) UpdatePasswordHashAndStatus(ctx context.Context, userID uuid.UUID, passwordHash string, status domain.UserStatus) (int64, error) {
	updated, err := r.client.User.Update().Where(user.UserIDEQ(userID), user.DeletedAtIsNil()).SetPasswordHash(passwordHash).SetStatus(int64(status)).AddTokenVersion(1).Save(ctx)
	if err == nil {
		if updated == 0 {
			return 0, response.NotFoundError(apperror.MsgUserNotFound)
		}
		user, err := r.GetByUserID(ctx, userID)
		if err != nil {
			return 0, err
		}
		return user.TokenVersion, nil
	}
	return 0, fmt.Errorf("update user password by user_id %s: %w", userID.String(), err)
}

func (r *userRepository) ListUsers(ctx context.Context, input ListUsersInput) ([]*ent.User, int, error) {
	predicates := userListPredicates(input)
	total, err := r.client.User.Query().Where(predicates...).Count(ctx)
	if err != nil {
		return nil, 0, fmt.Errorf("count users: %w", err)
	}

	users, err := r.client.User.Query().
		Where(predicates...).
		Order(user.ByID()).
		Offset(input.Offset).
		Limit(input.Limit).
		All(ctx)
	if err != nil {
		return nil, 0, fmt.Errorf("list users: %w", err)
	}
	return users, total, nil
}

func userListPredicates(input ListUsersInput) []predicate.User {
	predicates := []predicate.User{user.DeletedAtIsNil()}
	if input.Nickname != "" {
		predicates = append(predicates, user.NicknameContains(input.Nickname))
	}
	if input.Username != "" {
		predicates = append(predicates, user.UsernameEQ(input.Username))
	}
	if input.Status != nil {
		predicates = append(predicates, user.StatusEQ(int64(*input.Status)))
	}
	return predicates
}
