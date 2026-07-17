package postgres

import (
	"context"
	"fmt"

	"github.com/google/uuid"

	"github.com/aegiscore/user-service/ent"
	entuser "github.com/aegiscore/user-service/ent/user"
	userapplication "github.com/aegiscore/user-service/internal/features/user/application"
	userdomain "github.com/aegiscore/user-service/internal/features/user/domain"
	"github.com/aegiscore/user-service/internal/shared/identity"
)

type UserStore struct {
	client *ent.Client
}

var _ userapplication.UserProfileStore = (*UserStore)(nil)

// NewUserStore 构造基于 Ent 的用户 store。
func NewUserStore(client *ent.Client) *UserStore {
	return &UserStore{client: client}
}

// Create 插入用户记录，并将唯一约束冲突映射为 ErrUserAlreadyExists。
func (s *UserStore) Create(ctx context.Context, input userapplication.CreateUserInput) (*userdomain.User, error) {
	created, err := s.client.User.Create().
		SetUserID(input.UserID).
		SetNickname(input.Nickname).
		SetUsername(input.Username).
		SetPasswordHash(input.PasswordHash).
		SetStatus(int64(input.Status)).
		Save(ctx)
	if err == nil {
		return toModel(created), nil
	}
	if ent.IsConstraintError(err) {
		return nil, identity.ErrUserAlreadyExists
	}
	return nil, fmt.Errorf("create user username %s: %w", input.Username, err)
}

// GetByUserID 按外部 UUID 返回未软删除用户。
func (s *UserStore) GetByUserID(ctx context.Context, userID uuid.UUID) (*userdomain.User, error) {
	found, err := s.client.User.Query().Where(entuser.UserIDEQ(userID), entuser.DeletedAtIsNil()).Only(ctx)
	if err == nil {
		return toModel(found), nil
	}
	if ent.IsNotFound(err) {
		return nil, identity.ErrUserNotFound
	}
	return nil, fmt.Errorf("query user by user_id %s: %w", userID.String(), err)
}

// ListUsers 返回一页未软删除用户，以及是否存在下一页。
func (s *UserStore) ListUsers(ctx context.Context, input userapplication.ListUsersInput) ([]userdomain.User, bool, error) {
	predicates := buildListPredicates(input)
	if input.AfterUserID != nil {
		predicates = append(predicates, entuser.UserIDGT(*input.AfterUserID))
	}

	users, err := s.client.User.Query().
		Where(predicates...).
		Order(entuser.ByUserID()).
		Limit(input.Limit + 1).
		All(ctx)
	if err != nil {
		return nil, false, fmt.Errorf("list users: %w", err)
	}
	hasNext := len(users) > input.Limit
	if hasNext {
		users = users[:input.Limit]
	}
	return toModels(users), hasNext, nil
}

func toModel(entUser *ent.User) *userdomain.User {
	if entUser == nil {
		return nil
	}
	return &userdomain.User{
		ID:           entUser.ID,
		UserID:       entUser.UserID,
		Nickname:     entUser.Nickname,
		Username:     entUser.Username,
		PasswordHash: entUser.PasswordHash,
		Status:       identity.UserStatus(entUser.Status),
		TokenVersion: entUser.TokenVersion,
		CreatedAt:    entUser.CreatedAt,
		UpdatedAt:    entUser.UpdatedAt,
	}
}

func toModels(users []*ent.User) []userdomain.User {
	result := make([]userdomain.User, 0, len(users))
	for _, entUser := range users {
		if mapped := toModel(entUser); mapped != nil {
			result = append(result, *mapped)
		}
	}
	return result
}
