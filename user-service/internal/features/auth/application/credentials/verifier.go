package credentials

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"go.uber.org/zap"

	"github.com/aegiscore/common/runtime/logger"
	"github.com/aegiscore/common/security/password"
	authapplication "github.com/aegiscore/user-service/internal/features/auth/application"
	"github.com/aegiscore/user-service/internal/features/auth/application/authctx"
	authdomain "github.com/aegiscore/user-service/internal/features/auth/domain"
	"github.com/aegiscore/user-service/internal/shared/identity"
)

// Verifier 校验登录凭证并完成强制改密。
type Verifier interface {
	VerifyPassword(ctx context.Context, username string, plainPassword string) (*authdomain.UserCredential, error)
	ChangePassword(ctx context.Context, userID uuid.UUID, expectedTokenVersion int64, newPassword string) (*authdomain.CredentialUpdateResult, error)
}

type verifier struct {
	repo            authapplication.UserCredentialStore
	passwordService *password.Service
}

// NewVerifier 构造凭据校验组件。
func NewVerifier(repo authapplication.UserCredentialStore, passwordService *password.Service) Verifier {
	return &verifier{repo: repo, passwordService: passwordService}
}

// VerifyPassword 校验 username/password 组合，并执行登录状态规则。
func (v *verifier) VerifyPassword(ctx context.Context, username string, plainPassword string) (*authdomain.UserCredential, error) {
	credential, err := v.repo.GetByUsername(ctx, username)
	if err != nil {
		if errors.Is(err, identity.ErrUserNotFound) {
			fields := append([]zap.Field{zap.String("username", username)}, authctx.ClientContextFields(ctx)...)
			logger.Warn(ctx, "login user not found", fields...)
			return nil, authdomain.ErrInvalidCredentials
		}
		logger.Error(ctx, "query login user failed", logger.StackTrace(zap.String("username", username), zap.Error(err))...)
		return nil, err
	}
	matched, err := v.passwordService.VerifyContext(ctx, plainPassword, credential.PasswordHash)
	if err != nil {
		if errors.Is(err, password.ErrPasswordKDFBusy) {
			fields := append([]zap.Field{zap.String("username", username), zap.String("user_id", credential.UserID.String()), zap.Error(err)}, authctx.ClientContextFields(ctx)...)
			logger.Warn(ctx, "password kdf busy", fields...)
			return nil, err
		}
		logger.Error(ctx, "verify login password failed", logger.StackTrace(zap.String("username", username), zap.String("user_id", credential.UserID.String()), zap.Error(err))...)
		return nil, authdomain.ErrInvalidCredentials
	}
	if !matched {
		fields := append([]zap.Field{zap.String("username", username), zap.String("user_id", credential.UserID.String())}, authctx.ClientContextFields(ctx)...)
		logger.Warn(ctx, "login password mismatch", fields...)
		return nil, authdomain.ErrInvalidCredentials
	}
	if !credential.RequiresPasswordChange() && !credential.CanLogin() {
		// 必须改密用户只允许通过登录以获取受限 token；其他禁用状态直接登录失败。
		fields := append([]zap.Field{zap.String("username", username), zap.String("user_id", credential.UserID.String()), zap.Int64("status", int64(credential.Status))}, authctx.ClientContextFields(ctx)...)
		logger.Warn(ctx, "login user status rejected", fields...)
		return nil, errors.Join(authdomain.ErrInvalidCredentials, authdomain.ErrUserStatusRejected)
	}

	return credential, nil
}

// ChangePassword 为当前受限于改密流程的用户替换凭证。
func (v *verifier) ChangePassword(ctx context.Context, userID uuid.UUID, expectedTokenVersion int64, newPassword string) (*authdomain.CredentialUpdateResult, error) {
	credential, err := v.repo.GetCredentialByUserID(ctx, userID)
	if err != nil {
		if errors.Is(err, identity.ErrUserNotFound) {
			logger.Warn(ctx, "change password user not found", zap.String("user_id", userID.String()))
			return nil, identity.ErrUserNotFound
		}
		logger.Error(ctx, "query change password user failed", logger.StackTrace(zap.String("user_id", userID.String()), zap.Error(err))...)
		return nil, err
	}
	if !credential.CanChangePassword() {
		// 此端点仅用于强制改密，不承担普通资料密码更新职责。
		logger.Warn(ctx, "change password status rejected", zap.String("user_id", userID.String()), zap.Int64("status", int64(credential.Status)))
		return nil, authdomain.ErrTokenInvalid
	}
	passwordHash, err := v.passwordService.HashContext(ctx, newPassword)
	if err != nil {
		logger.Error(ctx, "hash changed password failed", logger.StackTrace(zap.String("user_id", userID.String()), zap.Error(err))...)
		return nil, fmt.Errorf("hash changed password: %w", err)
	}
	expectedStatus := identity.UserStatusMustChangePassword
	tokenVersion, err := v.repo.UpdateCredentials(ctx, authdomain.UpdateCredentialsInput{UserID: userID, PasswordHash: passwordHash, Status: identity.UserStatusNormal, ExpectedStatus: &expectedStatus, ExpectedTokenVersion: &expectedTokenVersion})
	if err != nil {
		if errors.Is(err, identity.ErrUserNotFound) {
			logger.Warn(ctx, "update credentials user not found", zap.String("user_id", userID.String()))
			return nil, identity.ErrUserNotFound
		}
		if errors.Is(err, authdomain.ErrTokenInvalid) {
			logger.Warn(ctx, "update credentials condition rejected", zap.String("user_id", userID.String()))
			return nil, authdomain.ErrTokenInvalid
		}
		logger.Error(ctx, "update credentials failed", logger.StackTrace(zap.String("user_id", userID.String()), zap.Error(err))...)
		return nil, err
	}
	return &authdomain.CredentialUpdateResult{UserID: userID, TokenVersion: tokenVersion}, nil
}
