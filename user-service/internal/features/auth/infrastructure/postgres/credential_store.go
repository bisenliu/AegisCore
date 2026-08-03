package postgres

import (
	"context"
	"fmt"

	"github.com/google/uuid"

	authapplication "github.com/aegiscore/user-service/internal/features/auth/application"
	authdomain "github.com/aegiscore/user-service/internal/features/auth/domain"
	"github.com/aegiscore/user-service/internal/persistence/ent"
	"github.com/aegiscore/user-service/internal/persistence/ent/predicate"
	entuser "github.com/aegiscore/user-service/internal/persistence/ent/user"
	"github.com/aegiscore/user-service/internal/shared/identity"
)

type CredentialStore struct {
	client *ent.Client
}

var _ authapplication.UserCredentialStore = (*CredentialStore)(nil)
var _ authapplication.UserTokenVersionStore = (*CredentialStore)(nil)

// NewCredentialStore 构造基于 Ent 的认证凭据和 token version store。
func NewCredentialStore(client *ent.Client) *CredentialStore {
	return &CredentialStore{client: client}
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
	found, err := s.client.User.Query().Where(entuser.UserIDEQ(userID), entuser.DeletedAtIsNil()).Select(entuser.FieldID).Only(ctx)
	if err != nil {
		if ent.IsNotFound(err) {
			return 0, identity.ErrUserNotFound
		}
		return 0, fmt.Errorf("query user before increment token version by user_id %s: %w", userID.String(), err)
	}
	updated, err := s.client.User.UpdateOneID(found.ID).
		Where(entuser.DeletedAtIsNil()).
		AddTokenVersion(1).
		Select(entuser.FieldTokenVersion).
		Save(ctx)
	if err != nil {
		if ent.IsNotFound(err) {
			return 0, identity.ErrUserNotFound
		}
		return 0, fmt.Errorf("increment user token version by user_id %s: %w", userID.String(), err)
	}
	return updated.TokenVersion, nil
}

// UpdateCredentials 替换密码哈希和状态，递增 token version 并返回新版本。
func (s *CredentialStore) UpdateCredentials(ctx context.Context, input authdomain.UpdateCredentialsInput) (int64, error) {
	found, err := s.client.User.Query().Where(entuser.UserIDEQ(input.UserID), entuser.DeletedAtIsNil()).Select(entuser.FieldID).Only(ctx)
	if err != nil {
		if ent.IsNotFound(err) {
			return 0, identity.ErrUserNotFound
		}
		return 0, fmt.Errorf("query user before update credentials by user_id %s: %w", input.UserID.String(), err)
	}
	predicates := []predicate.User{entuser.DeletedAtIsNil()}
	conditional := false
	if input.ExpectedStatus != nil {
		predicates = append(predicates, entuser.StatusEQ(int64(*input.ExpectedStatus)))
		conditional = true
	}
	if input.ExpectedTokenVersion != nil {
		predicates = append(predicates, entuser.TokenVersionEQ(*input.ExpectedTokenVersion))
		conditional = true
	}
	updated, err := s.client.User.UpdateOneID(found.ID).
		Where(predicates...).
		SetPasswordHash(input.PasswordHash).
		SetStatus(int64(input.Status)).
		AddTokenVersion(1).
		Select(entuser.FieldTokenVersion).
		Save(ctx)
	if err != nil {
		if ent.IsNotFound(err) {
			if conditional {
				if _, getErr := s.GetCredentialByUserID(ctx, input.UserID); getErr == nil {
					return 0, authdomain.ErrTokenInvalid
				}
			}
			return 0, identity.ErrUserNotFound
		}
		return 0, fmt.Errorf("update user credentials by user_id %s: %w", input.UserID.String(), err)
	}
	return updated.TokenVersion, nil
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
