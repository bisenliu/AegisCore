package service

import (
	"context"
	"errors"

	"github.com/aegiscore/common/contract/response"
	"github.com/aegiscore/common/runtime/logger"
	"github.com/aegiscore/common/security/password"
	"github.com/aegiscore/user-services/internal/domain"
	"github.com/aegiscore/user-services/internal/errmsg"
	"github.com/aegiscore/user-services/internal/repository"
	"go.uber.org/zap"
)

type CredentialVerifier interface {
	VerifyPassword(ctx context.Context, username string, plainPassword string) (*domain.User, error)
}

type credentialVerifier struct {
	repo repository.UserRepository
}

func newCredentialVerifier(repo repository.UserRepository) CredentialVerifier {
	return &credentialVerifier{repo: repo}
}

func (v *credentialVerifier) VerifyPassword(ctx context.Context, username string, plainPassword string) (*domain.User, error) {
	user, err := v.repo.GetByUsername(ctx, username)
	if err != nil {
		if errors.Is(err, domain.ErrUserNotFound) {
			logger.Warn(ctx, "login user not found", zap.String("username", username))
			return nil, response.UnauthenticatedError(errmsg.MsgInvalidCredentials)
		}
		logger.Error(ctx, "query login user failed", logger.StackTrace(zap.String("username", username), zap.Error(err))...)
		return nil, response.FromError(err)
	}
	matched, err := password.Verify(plainPassword, user.PasswordHash)
	if err != nil {
		logger.Error(ctx, "verify login password failed", logger.StackTrace(zap.String("username", username), zap.String("user_id", user.UserID.String()), zap.Error(err))...)
		return nil, response.UnauthenticatedError(errmsg.MsgInvalidCredentials)
	}
	if !matched {
		logger.Warn(ctx, "login password mismatch", zap.String("username", username), zap.String("user_id", user.UserID.String()))
		return nil, response.UnauthenticatedError(errmsg.MsgInvalidCredentials)
	}
	if !user.RequiresPasswordChange() && !user.CanLogin() {
		logger.Warn(ctx, "login user status rejected", zap.String("username", username), zap.String("user_id", user.UserID.String()), zap.Int64("status", int64(user.Status)))
		return nil, response.UnauthenticatedError(errmsg.MsgInvalidCredentials)
	}

	return user, nil
}
