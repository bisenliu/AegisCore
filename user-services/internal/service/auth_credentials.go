package service

import (
	"context"
	"errors"

	"github.com/aegiscore/common/contract/response"
	"github.com/aegiscore/common/runtime/logger"
	"github.com/aegiscore/common/security/password"
	"github.com/aegiscore/user-services/internal/domain"
	"github.com/aegiscore/user-services/internal/messages"
	"github.com/aegiscore/user-services/internal/repository"
	"github.com/google/uuid"
	"go.uber.org/zap"
)

type CredentialVerifier interface {
	VerifyPassword(ctx context.Context, username string, plainPassword string) (*domain.User, error)
	ChangePassword(ctx context.Context, userID uuid.UUID, newPassword string) (*CredentialUpdateResult, error)
}

type CredentialUpdateResult struct {
	UserID       uuid.UUID
	TokenVersion int64
}

type credentialVerifier struct {
	repo repository.UserCredentialRepository
}

func newCredentialVerifier(repo repository.UserCredentialRepository) CredentialVerifier {
	return &credentialVerifier{repo: repo}
}

func (v *credentialVerifier) VerifyPassword(ctx context.Context, username string, plainPassword string) (*domain.User, error) {
	user, err := v.repo.GetByUsername(ctx, username)
	if err != nil {
		if errors.Is(err, domain.ErrUserNotFound) {
			logger.Warn(ctx, "login user not found", zap.String("username", username))
			return nil, response.UnauthenticatedError(messages.InvalidCredentials)
		}
		logger.Error(ctx, "query login user failed", logger.StackTrace(zap.String("username", username), zap.Error(err))...)
		return nil, response.FromError(err)
	}
	matched, err := password.Verify(plainPassword, user.PasswordHash)
	if err != nil {
		logger.Error(ctx, "verify login password failed", logger.StackTrace(zap.String("username", username), zap.String("user_id", user.UserID.String()), zap.Error(err))...)
		return nil, response.UnauthenticatedError(messages.InvalidCredentials)
	}
	if !matched {
		logger.Warn(ctx, "login password mismatch", zap.String("username", username), zap.String("user_id", user.UserID.String()))
		return nil, response.UnauthenticatedError(messages.InvalidCredentials)
	}
	if !user.RequiresPasswordChange() && !user.CanLogin() {
		logger.Warn(ctx, "login user status rejected", zap.String("username", username), zap.String("user_id", user.UserID.String()), zap.Int64("status", int64(user.Status)))
		return nil, response.UnauthenticatedError(messages.InvalidCredentials)
	}

	return user, nil
}

func (v *credentialVerifier) ChangePassword(ctx context.Context, userID uuid.UUID, newPassword string) (*CredentialUpdateResult, error) {
	user, err := v.repo.GetByUserID(ctx, userID)
	if err != nil {
		if errors.Is(err, domain.ErrUserNotFound) {
			logger.Warn(ctx, "change password user not found", zap.String("user_id", userID.String()))
			return nil, response.NotFoundError(messages.UserNotFound)
		}
		logger.Error(ctx, "query change password user failed", logger.StackTrace(zap.String("user_id", userID.String()), zap.Error(err))...)
		return nil, response.FromError(err)
	}
	if !user.CanChangePassword() {
		logger.Warn(ctx, "change password status rejected", zap.String("user_id", userID.String()), zap.Int64("status", int64(user.Status)))
		return nil, response.TokenInvalidError(messages.MissingSession)
	}
	passwordHash, err := password.Hash(newPassword)
	if err != nil {
		logger.Error(ctx, "hash changed password failed", logger.StackTrace(zap.String("user_id", userID.String()), zap.Error(err))...)
		return nil, response.FromError(err)
	}
	tokenVersion, err := v.repo.UpdateCredentials(ctx, repository.UpdateCredentialsInput{UserID: userID, PasswordHash: passwordHash, Status: domain.UserStatusNormal})
	if err != nil {
		if errors.Is(err, domain.ErrUserNotFound) {
			logger.Warn(ctx, "update credentials user not found", zap.String("user_id", userID.String()))
			return nil, response.NotFoundError(messages.UserNotFound)
		}
		logger.Error(ctx, "update credentials failed", logger.StackTrace(zap.String("user_id", userID.String()), zap.Error(err))...)
		return nil, response.FromError(err)
	}
	return &CredentialUpdateResult{UserID: userID, TokenVersion: tokenVersion}, nil
}
