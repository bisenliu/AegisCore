package postgres

import (
	"context"
	"fmt"

	"github.com/aegiscore/user-services/ent"
	entuser "github.com/aegiscore/user-services/ent/user"
	"github.com/aegiscore/user-services/internal/auth"
	"github.com/aegiscore/user-services/internal/user"
	"github.com/google/uuid"
	"go.uber.org/fx"
)

type userStore struct {
	client *ent.Client
}

var _ user.UserProfileStore = (*userStore)(nil)
var _ auth.UserCredentialStore = (*userStore)(nil)
var _ auth.UserTokenVersionStore = (*userStore)(nil)

// UserStoreParams 包含 PostgreSQL-backed 用户 store 所需的 Fx 输入。
type UserStoreParams struct {
	fx.In

	Client *ent.Client `name:"user_db"`
}

// NewUserStore 构造基于 Ent 的用户 store。
func NewUserStore(params UserStoreParams) *userStore {
	return &userStore{client: params.Client}
}

// Create 插入用户记录，并将唯一约束冲突映射为 ErrUserAlreadyExists。
func (s *userStore) Create(ctx context.Context, input user.CreateUserInput) (*user.User, error) {
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
		return nil, user.ErrUserAlreadyExists
	}
	return nil, fmt.Errorf("create user username %s: %w", input.Username, err)
}

// GetByUserID 按外部 UUID 返回未软删除用户。
func (s *userStore) GetByUserID(ctx context.Context, userID uuid.UUID) (*user.User, error) {
	found, err := s.client.User.Query().Where(entuser.UserIDEQ(userID), entuser.DeletedAtIsNil()).Only(ctx)
	if err == nil {
		return toModel(found), nil
	}
	if ent.IsNotFound(err) {
		return nil, user.ErrUserNotFound
	}
	return nil, fmt.Errorf("query user by user_id %s: %w", userID.String(), err)
}

// GetByUsername 按规范化 username 返回未软删除用户。
func (s *userStore) GetByUsername(ctx context.Context, username string) (*auth.UserCredential, error) {
	found, err := s.client.User.Query().Where(entuser.UsernameEQ(username), entuser.DeletedAtIsNil()).Only(ctx)
	if err == nil {
		return toCredential(found), nil
	}
	if ent.IsNotFound(err) {
		return nil, user.ErrUserNotFound
	}
	return nil, fmt.Errorf("query user by username %s: %w", username, err)
}

// GetCredentialByUserID 按外部 UUID 返回认证能力需要的最小用户凭据。
func (s *userStore) GetCredentialByUserID(ctx context.Context, userID uuid.UUID) (*auth.UserCredential, error) {
	found, err := s.client.User.Query().Where(entuser.UserIDEQ(userID), entuser.DeletedAtIsNil()).Only(ctx)
	if err == nil {
		return toCredential(found), nil
	}
	if ent.IsNotFound(err) {
		return nil, user.ErrUserNotFound
	}
	return nil, fmt.Errorf("query user credential by user_id %s: %w", userID.String(), err)
}

// GetTokenVersion 返回未软删除用户的当前 token version。
func (s *userStore) GetTokenVersion(ctx context.Context, userID uuid.UUID) (int64, error) {
	found, err := s.client.User.Query().Where(entuser.UserIDEQ(userID), entuser.DeletedAtIsNil()).Only(ctx)
	if err == nil {
		return found.TokenVersion, nil
	}
	if ent.IsNotFound(err) {
		return 0, user.ErrUserNotFound
	}
	return 0, fmt.Errorf("query user token version by user_id %s: %w", userID.String(), err)
}

// IncrementTokenVersion 递增用户 token version 并返回新值。
func (s *userStore) IncrementTokenVersion(ctx context.Context, userID uuid.UUID) (int64, error) {
	updated, err := s.client.User.Update().Where(entuser.UserIDEQ(userID), entuser.DeletedAtIsNil()).AddTokenVersion(1).Save(ctx)
	if err == nil {
		if updated == 0 {
			// Ent Update().Save 返回受影响行数，0 表示过滤条件未匹配到用户。
			return 0, user.ErrUserNotFound
		}
		found, err := s.GetByUserID(ctx, userID)
		if err != nil {
			return 0, err
		}
		return found.TokenVersion, nil
	}
	return 0, fmt.Errorf("increment user token version by user_id %s: %w", userID.String(), err)
}

// UpdateCredentials 替换密码哈希和状态，递增 token version 并返回新版本。
func (s *userStore) UpdateCredentials(ctx context.Context, input auth.UpdateCredentialsInput) (int64, error) {
	updated, err := s.client.User.Update().Where(entuser.UserIDEQ(input.UserID), entuser.DeletedAtIsNil()).SetPasswordHash(input.PasswordHash).SetStatus(int64(input.Status)).AddTokenVersion(1).Save(ctx)
	if err == nil {
		if updated == 0 {
			// Ent Update().Save 返回受影响行数，0 表示过滤条件未匹配到用户。
			return 0, user.ErrUserNotFound
		}
		found, err := s.GetByUserID(ctx, input.UserID)
		if err != nil {
			return 0, err
		}
		return found.TokenVersion, nil
	}
	return 0, fmt.Errorf("update user credentials by user_id %s: %w", input.UserID.String(), err)
}

// ListUsers 返回一页未软删除用户，以及同一过滤条件下的总数。
func (s *userStore) ListUsers(ctx context.Context, input user.ListUsersInput) ([]user.User, int, error) {
	predicates := buildListPredicates(input)
	total, err := s.client.User.Query().Where(predicates...).Count(ctx)
	if err != nil {
		return nil, 0, fmt.Errorf("count users: %w", err)
	}

	users, err := s.client.User.Query().
		Where(predicates...).
		Order(entuser.ByID()).
		Offset(input.Offset).
		Limit(input.Limit).
		All(ctx)
	if err != nil {
		return nil, 0, fmt.Errorf("list users: %w", err)
	}
	return toModels(users), total, nil
}

func toModel(entUser *ent.User) *user.User {
	if entUser == nil {
		return nil
	}
	return &user.User{
		ID:           entUser.ID,
		UserID:       entUser.UserID,
		Nickname:     entUser.Nickname,
		Username:     entUser.Username,
		PasswordHash: entUser.PasswordHash,
		Status:       user.UserStatus(entUser.Status),
		TokenVersion: entUser.TokenVersion,
		CreatedAt:    entUser.CreatedAt,
		UpdatedAt:    entUser.UpdatedAt,
	}
}

func toModels(users []*ent.User) []user.User {
	result := make([]user.User, 0, len(users))
	for _, entUser := range users {
		if mapped := toModel(entUser); mapped != nil {
			result = append(result, *mapped)
		}
	}
	return result
}

func toCredential(entUser *ent.User) *auth.UserCredential {
	if entUser == nil {
		return nil
	}
	return &auth.UserCredential{
		UserID:       entUser.UserID,
		Username:     entUser.Username,
		PasswordHash: entUser.PasswordHash,
		Status:       user.UserStatus(entUser.Status),
		TokenVersion: entUser.TokenVersion,
	}
}
