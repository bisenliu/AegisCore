package app

import (
	"context"
	"errors"
	"fmt"

	"github.com/aegiscore/common/runtime/logger"
	"github.com/aegiscore/common/security/password"
	authdomain "github.com/aegiscore/user-service/internal/features/auth/domain"
	userdomain "github.com/aegiscore/user-service/internal/features/user/domain"
	"github.com/google/uuid"
	"go.uber.org/zap"
)

type credentialVerifier struct {
	repo UserCredentialStore
}

func newCredentialVerifier(repo UserCredentialStore) CredentialVerifier {
	return &credentialVerifier{repo: repo}
}

// VerifyPassword 校验 username/password 组合，并执行登录状态规则。
func (v *credentialVerifier) VerifyPassword(ctx context.Context, username string, plainPassword string) (*authdomain.UserCredential, error) {
	credential, err := v.repo.GetByUsername(ctx, username)
	if err != nil {
		if errors.Is(err, userdomain.ErrUserNotFound) {
			logger.Warn(ctx, "login user not found", zap.String("username", username))
			return nil, authdomain.ErrInvalidCredentials
		}
		logger.Error(ctx, "query login user failed", logger.StackTrace(zap.String("username", username), zap.Error(err))...)
		return nil, err
	}
	matched, err := password.VerifyContext(ctx, plainPassword, credential.PasswordHash)
	if err != nil {
		logger.Error(ctx, "verify login password failed", logger.StackTrace(zap.String("username", username), zap.String("user_id", credential.UserID.String()), zap.Error(err))...)
		return nil, authdomain.ErrInvalidCredentials
	}
	if !matched {
		logger.Warn(ctx, "login password mismatch", zap.String("username", username), zap.String("user_id", credential.UserID.String()))
		return nil, authdomain.ErrInvalidCredentials
	}
	if !credential.RequiresPasswordChange() && !credential.CanLogin() {
		// 必须改密用户只允许通过登录以获取受限 token；其他禁用状态直接登录失败。
		logger.Warn(ctx, "login user status rejected", zap.String("username", username), zap.String("user_id", credential.UserID.String()), zap.Int64("status", int64(credential.Status)))
		return nil, authdomain.ErrInvalidCredentials
	}

	return credential, nil
}

// ChangePassword 为当前受限于改密流程的用户替换凭证。
func (v *credentialVerifier) ChangePassword(ctx context.Context, userID uuid.UUID, newPassword string) (*authdomain.CredentialUpdateResult, error) {
	credential, err := v.repo.GetCredentialByUserID(ctx, userID)
	if err != nil {
		if errors.Is(err, userdomain.ErrUserNotFound) {
			logger.Warn(ctx, "change password user not found", zap.String("user_id", userID.String()))
			return nil, userdomain.ErrUserNotFound
		}
		logger.Error(ctx, "query change password user failed", logger.StackTrace(zap.String("user_id", userID.String()), zap.Error(err))...)
		return nil, err
	}
	if !credential.CanChangePassword() {
		// 此端点仅用于强制改密，不承担普通资料密码更新职责。
		logger.Warn(ctx, "change password status rejected", zap.String("user_id", userID.String()), zap.Int64("status", int64(credential.Status)))
		return nil, authdomain.ErrTokenInvalid
	}
	passwordHash, err := password.HashContext(ctx, newPassword)
	if err != nil {
		logger.Error(ctx, "hash changed password failed", logger.StackTrace(zap.String("user_id", userID.String()), zap.Error(err))...)
		return nil, fmt.Errorf("hash changed password: %w", err)
	}
	tokenVersion, err := v.repo.UpdateCredentials(ctx, authdomain.UpdateCredentialsInput{UserID: userID, PasswordHash: passwordHash, Status: userdomain.UserStatusNormal})
	if err != nil {
		if errors.Is(err, userdomain.ErrUserNotFound) {
			logger.Warn(ctx, "update credentials user not found", zap.String("user_id", userID.String()))
			return nil, userdomain.ErrUserNotFound
		}
		logger.Error(ctx, "update credentials failed", logger.StackTrace(zap.String("user_id", userID.String()), zap.Error(err))...)
		return nil, err
	}
	return &authdomain.CredentialUpdateResult{UserID: userID, TokenVersion: tokenVersion}, nil
}
