package postgres

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"go.uber.org/fx"

	"github.com/aegiscore/user-service/ent"
	"github.com/aegiscore/user-service/ent/predicate"
	entuser "github.com/aegiscore/user-service/ent/user"
	authapplication "github.com/aegiscore/user-service/internal/features/auth/application"
	authdomain "github.com/aegiscore/user-service/internal/features/auth/domain"
	"github.com/aegiscore/user-service/internal/shared/identity"
)

type CredentialStore struct {
	client *ent.Client
}

var _ authapplication.UserCredentialStore = (*CredentialStore)(nil)
var _ authapplication.UserTokenVersionStore = (*CredentialStore)(nil)

// CredentialStoreParams 包含 PostgreSQL-backed 认证凭据 store 所需的 Fx 输入。
type CredentialStoreParams struct {
	fx.In

	Client *ent.Client `name:"user_db"`
}

// NewCredentialStore 构造基于 Ent 的认证凭据和 token version store。
func NewCredentialStore(params CredentialStoreParams) *CredentialStore {
	return &CredentialStore{client: params.Client}
}

// GetByUsername 按规范化 username 返回未软删除用户的认证凭据。
func (s *CredentialStore) GetByUsername(ctx context.Context, username string) (*authdomain.UserCredential, error) {
	found, err := s.client.User.Query().Where(entuser.UsernameEQ(username), entuser.DeletedAtIsNil()).Only(ctx)
	if err == nil {
		return toCredential(found), nil
	}
	if ent.IsNotFound(err) {
		return nil, identity.ErrUserNotFound
	}
	return nil, fmt.Errorf("query user by username %s: %w", username, err)
}

// GetCredentialByUserID 按外部 UUID 返回认证能力需要的最小用户凭据。
func (s *CredentialStore) GetCredentialByUserID(ctx context.Context, userID uuid.UUID) (*authdomain.UserCredential, error) {
	found, err := s.client.User.Query().Where(entuser.UserIDEQ(userID), entuser.DeletedAtIsNil()).Only(ctx)
	if err == nil {
		return toCredential(found), nil
	}
	if ent.IsNotFound(err) {
		return nil, identity.ErrUserNotFound
	}
	return nil, fmt.Errorf("query user credential by user_id %s: %w", userID.String(), err)
}

// GetTokenVersion 返回未软删除用户的当前 token version。
func (s *CredentialStore) GetTokenVersion(ctx context.Context, userID uuid.UUID) (int64, error) {
	found, err := s.client.User.Query().Where(entuser.UserIDEQ(userID), entuser.DeletedAtIsNil()).Only(ctx)
	if err == nil {
		return found.TokenVersion, nil
	}
	if ent.IsNotFound(err) {
		return 0, identity.ErrUserNotFound
	}
	return 0, fmt.Errorf("query user token version by user_id %s: %w", userID.String(), err)
}

// IncrementTokenVersion 递增用户 token version 并返回新值。
func (s *CredentialStore) IncrementTokenVersion(ctx context.Context, userID uuid.UUID) (int64, error) {
	updated, err := s.client.User.Update().Where(entuser.UserIDEQ(userID), entuser.DeletedAtIsNil()).AddTokenVersion(1).Save(ctx)
	if err == nil {
		if updated == 0 {
			// Ent Update().Save 返回受影响行数，0 表示过滤条件未匹配到用户。
			return 0, identity.ErrUserNotFound
		}
		return s.GetTokenVersion(ctx, userID)
	}
	return 0, fmt.Errorf("increment user token version by user_id %s: %w", userID.String(), err)
}

// UpdateCredentials 替换密码哈希和状态，递增 token version 并返回新版本。
func (s *CredentialStore) UpdateCredentials(ctx context.Context, input authdomain.UpdateCredentialsInput) (int64, error) {
	predicates := []predicate.User{entuser.UserIDEQ(input.UserID), entuser.DeletedAtIsNil()}
	conditional := false
	if input.ExpectedStatus != nil {
		predicates = append(predicates, entuser.StatusEQ(int64(*input.ExpectedStatus)))
		conditional = true
	}
	if input.ExpectedTokenVersion != nil {
		predicates = append(predicates, entuser.TokenVersionEQ(*input.ExpectedTokenVersion))
		conditional = true
	}
	updated, err := s.client.User.Update().
		Where(predicates...).
		SetPasswordHash(input.PasswordHash).
		SetStatus(int64(input.Status)).
		AddTokenVersion(1).
		Save(ctx)
	if err == nil {
		if updated == 0 {
			if conditional {
				if _, getErr := s.GetCredentialByUserID(ctx, input.UserID); getErr == nil {
					return 0, authdomain.ErrTokenInvalid
				}
			}
			// Ent Update().Save 返回受影响行数，0 表示过滤条件未匹配到用户。
			return 0, identity.ErrUserNotFound
		}
		return s.GetTokenVersion(ctx, input.UserID)
	}
	return 0, fmt.Errorf("update user credentials by user_id %s: %w", input.UserID.String(), err)
}

func toCredential(entUser *ent.User) *authdomain.UserCredential {
	if entUser == nil {
		return nil
	}
	return &authdomain.UserCredential{
		UserID:       entUser.UserID,
		Username:     entUser.Username,
		PasswordHash: entUser.PasswordHash,
		Status:       identity.UserStatus(entUser.Status),
		TokenVersion: entUser.TokenVersion,
	}
}
