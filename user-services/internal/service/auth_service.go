package service

import (
	"context"
	"errors"

	"github.com/aegiscore/common/contract/response"
	"github.com/aegiscore/common/runtime/config"
	"github.com/aegiscore/common/runtime/logger"
	"github.com/aegiscore/common/security/auth"
	"github.com/aegiscore/common/security/password"
	"github.com/aegiscore/user-services/internal/domain"
	"github.com/aegiscore/user-services/internal/dto"
	"github.com/aegiscore/user-services/internal/messages"
	"github.com/aegiscore/user-services/internal/repository"
	"github.com/google/uuid"
	"go.uber.org/fx"
	"go.uber.org/zap"
)

type AuthService interface {
	Login(ctx context.Context, req dto.LoginRequest) (*dto.TokenResponse, error)
	ChangePassword(ctx context.Context, req dto.ChangePasswordRequest) (*dto.ChangePasswordResponse, error)
	Refresh(ctx context.Context, req dto.RefreshTokenRequest) (*dto.TokenResponse, error)
	Logout(ctx context.Context) (*dto.LogoutResponse, error)
	LogoutAll(ctx context.Context) (*dto.LogoutResponse, error)
}

type AuthServiceParams struct {
	fx.In

	Repo     repository.UserRepository
	Sessions repository.AuthSessionRepository
	JWT      *auth.JWTService
	Config   *config.Config
}

type authService struct {
	repo        repository.UserRepository
	jwt         *auth.JWTService
	config      *config.Config
	credentials CredentialVerifier
	tokens      AuthTokenIssuer
	sessions    AuthSessionManager
}

func NewAuthService(params AuthServiceParams) AuthService {
	return &authService{
		repo:        params.Repo,
		jwt:         params.JWT,
		config:      params.Config,
		credentials: newCredentialVerifier(params.Repo),
		tokens:      newAuthTokenIssuer(params.JWT, params.Config),
		sessions:    newAuthSessionManager(params.Sessions),
	}
}

func (s *authService) Login(ctx context.Context, req dto.LoginRequest) (*dto.TokenResponse, error) {
	logger.Info(ctx, "login user", zap.String("username", req.Username))
	user, err := s.credentials.VerifyPassword(ctx, req.Username, req.Password)
	if err != nil {
		return nil, err
	}

	if user.RequiresPasswordChange() {
		logger.Warn(ctx, "login requires password change", zap.String("username", req.Username), zap.String("user_id", user.UserID.String()), zap.Int64("token_version", user.TokenVersion))
		return s.tokens.IssuePasswordChangeToken(ctx, user.UserID.String(), user.TokenVersion, uuid.NewString())
	}

	logger.Info(ctx, "login user authenticated", zap.String("username", req.Username), zap.String("user_id", user.UserID.String()), zap.Int64("token_version", user.TokenVersion))
	return s.issueTokenPair(ctx, user.UserID.String(), user.TokenVersion, uuid.NewString())
}

func (s *authService) ChangePassword(ctx context.Context, req dto.ChangePasswordRequest) (*dto.ChangePasswordResponse, error) {
	parsedUserID, err := s.verifyPasswordChangeToken(ctx, req.Token)
	if err != nil {
		return nil, err
	}
	user, err := s.repo.GetByUserID(ctx, parsedUserID)
	if err != nil {
		if errors.Is(err, domain.ErrUserNotFound) {
			logger.Warn(ctx, "change password user not found", zap.String("user_id", parsedUserID.String()))
			return nil, response.NotFoundError(messages.UserNotFound)
		}
		logger.Error(ctx, "query change password user failed", logger.StackTrace(zap.String("user_id", parsedUserID.String()), zap.Error(err))...)
		return nil, response.FromError(err)
	}
	if !user.CanChangePassword() {
		logger.Warn(ctx, "change password status rejected", zap.String("user_id", parsedUserID.String()), zap.Int64("status", int64(user.Status)))
		return nil, response.TokenInvalidError(messages.MissingSession)
	}
	passwordHash, err := password.Hash(req.NewPassword)
	if err != nil {
		logger.Error(ctx, "hash changed password failed", logger.StackTrace(zap.String("user_id", parsedUserID.String()), zap.Error(err))...)
		return nil, response.FromError(err)
	}
	if _, err := s.repo.UpdateCredentials(ctx, repository.UpdateCredentialsInput{UserID: parsedUserID, PasswordHash: passwordHash, Status: domain.UserStatusNormal}); err != nil {
		if errors.Is(err, domain.ErrUserNotFound) {
			logger.Warn(ctx, "update credentials user not found", zap.String("user_id", parsedUserID.String()))
			return nil, response.NotFoundError(messages.UserNotFound)
		}
		logger.Error(ctx, "update credentials failed", logger.StackTrace(zap.String("user_id", parsedUserID.String()), zap.Error(err))...)
		return nil, response.FromError(err)
	}
	if err := s.sessions.InvalidateUserTokenVersion(ctx, parsedUserID.String()); err != nil {
		logger.Error(ctx, "invalidate token version after password change failed", logger.StackTrace(zap.String("user_id", parsedUserID.String()), zap.Error(err))...)
		return nil, response.FromError(err)
	}
	return &dto.ChangePasswordResponse{Changed: true}, nil
}

func (s *authService) verifyPasswordChangeToken(ctx context.Context, token string) (uuid.UUID, error) {
	claims, parsedUserID, err := s.tokens.ParsePasswordChangeToken(ctx, token)
	if err != nil {
		return uuid.Nil, err
	}
	if err := s.sessions.ValidatePasswordChangeClaims(ctx, claims); err != nil {
		return uuid.Nil, err
	}
	return parsedUserID, nil
}

func (s *authService) Refresh(ctx context.Context, req dto.RefreshTokenRequest) (*dto.TokenResponse, error) {
	claims, err := s.tokens.ParseRefreshToken(ctx, req.RefreshToken)
	if err != nil {
		return nil, err
	}
	session, currentVersion, err := s.sessions.ValidateRefreshSession(ctx, claims)
	if err != nil {
		return nil, err
	}

	sessionID := session.SessionID
	if s.config.Auth.RefreshTokenRotation {
		if err := s.sessions.DeleteSession(ctx, claims.UserID, session.SessionID); err != nil {
			return nil, err
		}
		sessionID = uuid.NewString()
	}
	return s.issueTokenPair(ctx, claims.UserID, currentVersion, sessionID)
}

func (s *authService) Logout(ctx context.Context) (*dto.LogoutResponse, error) {
	userID, sessionID, err := authenticatedSession(ctx)
	if err != nil {
		logger.Warn(ctx, "logout missing authenticated session", zap.Error(err))
		return nil, err
	}
	if err := s.sessions.DeleteSession(ctx, userID, sessionID); err != nil {
		return nil, err
	}
	return &dto.LogoutResponse{LoggedOut: true}, nil
}

func (s *authService) LogoutAll(ctx context.Context) (*dto.LogoutResponse, error) {
	userID, _, err := authenticatedSession(ctx)
	if err != nil {
		logger.Warn(ctx, "logout all missing authenticated session", zap.Error(err))
		return nil, err
	}
	parsedUserID, err := uuid.Parse(userID)
	if err != nil {
		logger.Warn(ctx, "logout all user id invalid", zap.String("user_id", userID))
		return nil, response.UnauthenticatedError(messages.MissingSession)
	}
	if _, err := s.repo.IncrementTokenVersion(ctx, parsedUserID); err != nil {
		if errors.Is(err, domain.ErrUserNotFound) {
			logger.Warn(ctx, "logout all user not found", zap.String("user_id", userID))
			return nil, response.NotFoundError(messages.UserNotFound)
		}
		logger.Error(ctx, "increment token version failed", logger.StackTrace(zap.String("user_id", userID), zap.Error(err))...)
		return nil, response.FromError(err)
	}
	if err := s.sessions.InvalidateUserTokenVersion(ctx, userID); err != nil {
		return nil, err
	}
	if err := s.sessions.DeleteAllUserSessions(ctx, userID); err != nil {
		return nil, err
	}
	return &dto.LogoutResponse{LoggedOut: true}, nil
}

func (s *authService) issueTokenPair(ctx context.Context, userID string, tokenVersion int64, sessionID string) (*dto.TokenResponse, error) {
	tokens, err := s.tokens.IssueTokenPair(ctx, userID, tokenVersion, sessionID)
	if err != nil {
		return nil, err
	}
	if err := s.sessions.CreateTokenSession(ctx, userID, sessionID, tokenVersion, tokens.RefreshTTL); err != nil {
		return nil, err
	}
	return tokens.Response, nil
}

func authenticatedSession(ctx context.Context) (string, string, error) {
	userIDString, ok := auth.UserIDFromContext(ctx)
	if !ok {
		return "", "", response.UnauthenticatedError(messages.MissingSession)
	}
	if _, err := uuid.Parse(userIDString); err != nil {
		return "", "", response.UnauthenticatedError(messages.MissingSession)
	}
	sessionID, ok := auth.SessionIDFromContext(ctx)
	if !ok {
		return "", "", response.UnauthenticatedError(messages.MissingSession)
	}
	return userIDString, sessionID, nil
}
