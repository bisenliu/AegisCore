package service

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/aegiscore/common/contract/response"
	"github.com/aegiscore/common/runtime/config"
	"github.com/aegiscore/common/runtime/logger"
	"github.com/aegiscore/common/security/auth"
	"github.com/aegiscore/common/security/password"
	"github.com/aegiscore/user-services/internal/domain"
	"github.com/aegiscore/user-services/internal/dto"
	"github.com/aegiscore/user-services/internal/errmsg"
	"github.com/aegiscore/user-services/internal/repository"
	"github.com/google/uuid"
	"go.uber.org/fx"
	"go.uber.org/zap"
)

const (
	defaultAccessTokenTTL  = 15 * time.Minute
	defaultRefreshTokenTTL = 7 * 24 * time.Hour
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
	repo     repository.UserRepository
	sessions repository.AuthSessionRepository
	jwt      *auth.JWTService
	config   *config.Config
}

func NewAuthService(params AuthServiceParams) AuthService {
	return &authService{repo: params.Repo, sessions: params.Sessions, jwt: params.JWT, config: params.Config}
}

func (s *authService) Login(ctx context.Context, req dto.LoginRequest) (*dto.TokenResponse, error) {
	logger.Info(ctx, "login user", zap.String("username", req.Username))
	user, err := s.authenticateUser(ctx, req.Username, req.Password)
	if err != nil {
		return nil, err
	}

	if user.RequiresPasswordChange() {
		logger.Warn(ctx, "login requires password change", zap.String("username", req.Username), zap.String("user_id", user.UserID.String()), zap.Int64("token_version", user.TokenVersion))
		return s.issuePasswordChangeToken(ctx, user.UserID.String(), user.TokenVersion, uuid.NewString())
	}

	logger.Info(ctx, "login user authenticated", zap.String("username", req.Username), zap.String("user_id", user.UserID.String()), zap.Int64("token_version", user.TokenVersion))
	return s.issueTokenPair(ctx, user.UserID.String(), user.TokenVersion, uuid.NewString())
}

func (s *authService) authenticateUser(ctx context.Context, username, plainPassword string) (*domain.User, error) {
	user, err := s.repo.GetByUsername(ctx, username)
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

func (s *authService) ChangePassword(ctx context.Context, req dto.ChangePasswordRequest) (*dto.ChangePasswordResponse, error) {
	parsedUserID, err := s.verifyPasswordChangeToken(ctx, req.Token)
	if err != nil {
		return nil, err
	}
	user, err := s.repo.GetByUserID(ctx, parsedUserID)
	if err != nil {
		if errors.Is(err, domain.ErrUserNotFound) {
			logger.Warn(ctx, "change password user not found", zap.String("user_id", parsedUserID.String()))
			return nil, response.NotFoundError(errmsg.MsgUserNotFound)
		}
		logger.Error(ctx, "query change password user failed", logger.StackTrace(zap.String("user_id", parsedUserID.String()), zap.Error(err))...)
		return nil, response.FromError(err)
	}
	if !user.CanChangePassword() {
		logger.Warn(ctx, "change password status rejected", zap.String("user_id", parsedUserID.String()), zap.Int64("status", int64(user.Status)))
		return nil, response.TokenInvalidError(errmsg.MsgMissingSession)
	}
	passwordHash, err := password.Hash(req.NewPassword)
	if err != nil {
		logger.Error(ctx, "hash changed password failed", logger.StackTrace(zap.String("user_id", parsedUserID.String()), zap.Error(err))...)
		return nil, response.FromError(err)
	}
	if _, err := s.repo.UpdateCredentials(ctx, repository.UpdateCredentialsInput{UserID: parsedUserID, PasswordHash: passwordHash, Status: domain.UserStatusNormal}); err != nil {
		if errors.Is(err, domain.ErrUserNotFound) {
			logger.Warn(ctx, "update credentials user not found", zap.String("user_id", parsedUserID.String()))
			return nil, response.NotFoundError(errmsg.MsgUserNotFound)
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
	claims, err := s.jwt.ParsePasswordChangeToken(auth.StripBearerPrefix(token))
	if err != nil {
		logger.Warn(ctx, "password change token invalid", zap.Bool("token_present", token != ""))
		return uuid.Nil, response.TokenInvalidError(errmsg.MsgMissingSession)
	}
	currentVersion, err := s.sessions.GetCurrentTokenVersion(ctx, claims.UserID)
	if err != nil {
		if errors.Is(err, domain.ErrUserNotFound) {
			logger.Warn(ctx, "password change user not found", zap.String("user_id", claims.UserID), zap.String("session_id", claims.SessionID))
			return uuid.Nil, response.NotFoundError(errmsg.MsgUserNotFound)
		}
		logger.Error(ctx, "get password change token version failed", logger.StackTrace(zap.String("user_id", claims.UserID), zap.String("session_id", claims.SessionID), zap.Error(err))...)
		return uuid.Nil, response.FromError(err)
	}
	if currentVersion != claims.TokenVersion {
		logger.Warn(ctx, "password change token version mismatch", zap.String("user_id", claims.UserID), zap.String("session_id", claims.SessionID), zap.Int64("current_token_version", currentVersion), zap.Int64("token_version", claims.TokenVersion))
		return uuid.Nil, response.TokenInvalidError(errmsg.MsgMissingSession)
	}
	parsedUserID, err := uuid.Parse(claims.UserID)
	if err != nil {
		logger.Warn(ctx, "password change token user id invalid", zap.String("user_id", claims.UserID), zap.String("session_id", claims.SessionID))
		return uuid.Nil, response.TokenInvalidError(errmsg.MsgMissingSession)
	}
	return parsedUserID, nil
}

func (s *authService) Refresh(ctx context.Context, req dto.RefreshTokenRequest) (*dto.TokenResponse, error) {
	claims, err := s.jwt.ParseRefreshToken(req.RefreshToken)
	if err != nil {
		logger.Warn(ctx, "refresh token invalid", zap.Bool("token_present", req.RefreshToken != ""))
		return nil, response.TokenInvalidError(errmsg.MsgMissingSession)
	}
	if claims.Subject != auth.SubjectRefresh {
		logger.Warn(ctx, "refresh token subject rejected", zap.String("user_id", claims.UserID), zap.String("session_id", claims.SessionID), zap.String("subject", claims.Subject))
		return nil, response.TokenInvalidError(errmsg.MsgMissingSession)
	}
	if _, err := uuid.Parse(claims.UserID); err != nil {
		logger.Warn(ctx, "refresh token user id invalid", zap.String("user_id", claims.UserID), zap.String("session_id", claims.SessionID))
		return nil, response.TokenInvalidError(errmsg.MsgMissingSession)
	}
	session, err := s.sessions.GetSession(ctx, claims.SessionID)
	if err != nil {
		if errors.Is(err, repository.ErrAuthSessionNotFound) {
			logger.Warn(ctx, "refresh session not found", zap.String("user_id", claims.UserID), zap.String("session_id", claims.SessionID))
			return nil, response.TokenInvalidError(errmsg.MsgMissingSession)
		}
		logger.Error(ctx, "get refresh session failed", logger.StackTrace(zap.String("user_id", claims.UserID), zap.String("session_id", claims.SessionID), zap.Error(err))...)
		return nil, response.FromError(err)
	}
	if session.UserID != claims.UserID || session.TokenVersion != claims.TokenVersion {
		logger.Warn(ctx, "refresh session mismatch", zap.String("user_id", claims.UserID), zap.String("session_id", claims.SessionID), zap.Int64("session_token_version", session.TokenVersion), zap.Int64("token_version", claims.TokenVersion))
		return nil, response.TokenInvalidError(errmsg.MsgMissingSession)
	}
	currentVersion, err := s.sessions.GetCurrentTokenVersion(ctx, claims.UserID)
	if err != nil {
		if errors.Is(err, domain.ErrUserNotFound) {
			logger.Warn(ctx, "refresh user not found", zap.String("user_id", claims.UserID), zap.String("session_id", claims.SessionID))
			return nil, response.NotFoundError(errmsg.MsgUserNotFound)
		}
		logger.Error(ctx, "get refresh token version failed", logger.StackTrace(zap.String("user_id", claims.UserID), zap.String("session_id", claims.SessionID), zap.Error(err))...)
		return nil, response.FromError(err)
	}
	if currentVersion != session.TokenVersion {
		logger.Warn(ctx, "refresh token version mismatch", zap.String("user_id", claims.UserID), zap.String("session_id", claims.SessionID), zap.Int64("current_token_version", currentVersion), zap.Int64("session_token_version", session.TokenVersion))
		return nil, response.TokenInvalidError(errmsg.MsgMissingSession)
	}

	sessionID := session.SessionID
	if s.config.Auth.RefreshTokenRotation {
		if err := s.sessions.DeleteSession(ctx, claims.UserID, session.SessionID); err != nil {
			logger.Error(ctx, "delete rotated refresh session failed", logger.StackTrace(zap.String("user_id", claims.UserID), zap.String("session_id", session.SessionID), zap.Error(err))...)
			return nil, response.FromError(err)
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
		logger.Error(ctx, "delete auth session failed", logger.StackTrace(zap.String("user_id", userID), zap.String("session_id", sessionID), zap.Error(err))...)
		return nil, response.FromError(err)
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
		return nil, response.UnauthenticatedError(errmsg.MsgMissingSession)
	}
	if _, err := s.repo.IncrementTokenVersion(ctx, parsedUserID); err != nil {
		if errors.Is(err, domain.ErrUserNotFound) {
			logger.Warn(ctx, "logout all user not found", zap.String("user_id", userID))
			return nil, response.NotFoundError(errmsg.MsgUserNotFound)
		}
		logger.Error(ctx, "increment token version failed", logger.StackTrace(zap.String("user_id", userID), zap.Error(err))...)
		return nil, response.FromError(err)
	}
	if err := s.sessions.InvalidateUserTokenVersion(ctx, userID); err != nil {
		logger.Error(ctx, "invalidate token version after logout all failed", logger.StackTrace(zap.String("user_id", userID), zap.Error(err))...)
		return nil, response.FromError(err)
	}
	if err := s.sessions.DeleteAllUserSessions(ctx, userID); err != nil {
		logger.Error(ctx, "delete all user sessions failed", logger.StackTrace(zap.String("user_id", userID), zap.Error(err))...)
		return nil, response.FromError(err)
	}
	return &dto.LogoutResponse{LoggedOut: true}, nil
}

func (s *authService) issueTokenPair(ctx context.Context, userID string, tokenVersion int64, sessionID string) (*dto.TokenResponse, error) {
	accessTTL := s.config.Auth.JWT.AccessTokenTTL
	if accessTTL <= 0 {
		accessTTL = defaultAccessTokenTTL
	}
	refreshTTL := s.config.Auth.JWT.RefreshTokenTTL
	if refreshTTL <= 0 {
		refreshTTL = defaultRefreshTokenTTL
	}
	access, err := s.jwt.SignAccessToken(auth.SignInput{UserID: userID, TokenVersion: tokenVersion, SessionID: sessionID, TTL: accessTTL})
	if err != nil {
		logger.Error(ctx, "sign access token failed", logger.StackTrace(zap.String("user_id", userID), zap.String("session_id", sessionID), zap.Int64("token_version", tokenVersion), zap.Error(err))...)
		return nil, response.FromError(fmt.Errorf("sign access token: %w", err))
	}
	refresh, err := s.jwt.SignRefreshToken(auth.SignInput{UserID: userID, TokenVersion: tokenVersion, SessionID: sessionID, TTL: refreshTTL})
	if err != nil {
		logger.Error(ctx, "sign refresh token failed", logger.StackTrace(zap.String("user_id", userID), zap.String("session_id", sessionID), zap.Int64("token_version", tokenVersion), zap.Error(err))...)
		return nil, response.FromError(fmt.Errorf("sign refresh token: %w", err))
	}
	if err := s.sessions.CreateSession(ctx, repository.AuthSession{UserID: userID, SessionID: sessionID, TokenVersion: tokenVersion, ExpiresAt: time.Now().Add(refreshTTL)}, refreshTTL); err != nil {
		logger.Error(ctx, "create auth session failed", logger.StackTrace(zap.String("user_id", userID), zap.String("session_id", sessionID), zap.Int64("token_version", tokenVersion), zap.Error(err))...)
		return nil, response.FromError(err)
	}
	return &dto.TokenResponse{AccessToken: access, RefreshToken: refresh, TokenType: auth.TokenTypeBearer, ExpiresIn: int64(accessTTL.Seconds())}, nil
}

func (s *authService) issuePasswordChangeToken(ctx context.Context, userID string, tokenVersion int64, sessionID string) (*dto.TokenResponse, error) {
	ttl := s.config.Auth.JWT.AccessTokenTTL
	if ttl <= 0 {
		ttl = defaultAccessTokenTTL
	}
	token, err := s.jwt.SignPasswordChangeToken(auth.SignInput{UserID: userID, TokenVersion: tokenVersion, SessionID: sessionID, TTL: ttl})
	if err != nil {
		logger.Error(ctx, "sign password change token failed", logger.StackTrace(zap.String("user_id", userID), zap.String("session_id", sessionID), zap.Int64("token_version", tokenVersion), zap.Error(err))...)
		return nil, response.FromError(fmt.Errorf("sign password change token: %w", err))
	}
	return &dto.TokenResponse{AccessToken: token, TokenType: auth.TokenTypeBearer, ExpiresIn: int64(ttl.Seconds()), PasswordChangeRequired: true}, nil
}

func authenticatedSession(ctx context.Context) (string, string, error) {
	userIDString, ok := auth.UserIDFromContext(ctx)
	if !ok {
		return "", "", response.UnauthenticatedError(errmsg.MsgMissingSession)
	}
	if _, err := uuid.Parse(userIDString); err != nil {
		return "", "", response.UnauthenticatedError(errmsg.MsgMissingSession)
	}
	sessionID, ok := auth.SessionIDFromContext(ctx)
	if !ok {
		return "", "", response.UnauthenticatedError(errmsg.MsgMissingSession)
	}
	return userIDString, sessionID, nil
}
