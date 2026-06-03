package service

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/aegiscore/common/auth"
	"github.com/aegiscore/common/config"
	"github.com/aegiscore/common/logger"
	"github.com/aegiscore/common/password"
	"github.com/aegiscore/common/response"
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
	Sessions SessionStore
	JWT      *auth.JWTService
	Config   *config.Config
}

type authService struct {
	repo     repository.UserRepository
	sessions SessionStore
	jwt      *auth.JWTService
	config   *config.Config
}

func NewAuthService(params AuthServiceParams) AuthService {
	return &authService{repo: params.Repo, sessions: params.Sessions, jwt: params.JWT, config: params.Config}
}

func (s *authService) Login(ctx context.Context, req dto.LoginRequest) (*dto.TokenResponse, error) {
	username := strings.TrimSpace(req.Username)
	plainPassword := strings.TrimSpace(req.Password)
	if username == "" || plainPassword == "" {
		return nil, response.UnauthenticatedError(errmsg.MsgInvalidCredentials)
	}

	user, err := s.repo.GetByUsername(ctx, username)
	if err != nil {
		if errors.Is(err, domain.ErrUserNotFound) {
			return nil, response.UnauthenticatedError(errmsg.MsgInvalidCredentials)
		}
		logger.Error(ctx, "query login user failed", zap.String("username", username), zap.Error(err))
		return nil, response.FromError(err)
	}
	matched, err := password.Verify(plainPassword, user.PasswordHash)
	if err != nil {
		logger.Error(ctx, "verify login password failed", zap.String("username", username), zap.Error(err))
		return nil, response.UnauthenticatedError(errmsg.MsgInvalidCredentials)
	}
	if !matched {
		return nil, response.UnauthenticatedError(errmsg.MsgInvalidCredentials)
	}
	status := domain.UserStatus(user.Status)
	if status == domain.UserStatusMustChangePassword {
		return s.issuePasswordChangeToken(ctx, user.UserID.String(), user.TokenVersion, uuid.NewString())
	}
	if !status.CanLogin() {
		return nil, response.UnauthenticatedError(errmsg.MsgInvalidCredentials)
	}

	return s.issueTokenPair(ctx, user.UserID.String(), user.TokenVersion, uuid.NewString())
}

func (s *authService) ChangePassword(ctx context.Context, req dto.ChangePasswordRequest) (*dto.ChangePasswordResponse, error) {
	plainPassword := strings.TrimSpace(req.NewPassword)
	if plainPassword == "" {
		return nil, response.ValidationFailedError(errmsg.MsgInvalidPassword)
	}
	claims, err := s.jwt.ParsePasswordChangeToken(normalizeRefreshToken(req.Token))
	if err != nil {
		return nil, response.TokenInvalidError(errmsg.MsgMissingSession)
	}
	currentVersion, err := s.sessions.GetCurrentTokenVersion(ctx, claims.UserID)
	if err != nil {
		return nil, response.FromError(err)
	}
	if currentVersion != claims.TokenVersion {
		return nil, response.TokenInvalidError(errmsg.MsgMissingSession)
	}
	parsedUserID, err := uuid.Parse(claims.UserID)
	if err != nil {
		return nil, response.TokenInvalidError(errmsg.MsgMissingSession)
	}
	user, err := s.repo.GetByUserID(ctx, parsedUserID)
	if err != nil {
		if errors.Is(err, domain.ErrUserNotFound) {
			return nil, response.NotFoundError(errmsg.MsgUserNotFound)
		}
		return nil, response.FromError(err)
	}
	if domain.UserStatus(user.Status) != domain.UserStatusMustChangePassword {
		return nil, response.TokenInvalidError(errmsg.MsgMissingSession)
	}
	passwordHash, err := password.Hash(plainPassword)
	if err != nil {
		logger.Error(ctx, "hash changed password failed", zap.String("user_id", claims.UserID), zap.Error(err))
		return nil, response.FromError(err)
	}
	if _, err := s.repo.UpdateCredentials(ctx, repository.UpdateCredentialsInput{UserID: parsedUserID, PasswordHash: passwordHash, Status: domain.UserStatusNormal}); err != nil {
		return nil, response.FromError(err)
	}
	if err := s.sessions.InvalidateUserTokenVersion(ctx, claims.UserID); err != nil {
		return nil, response.FromError(err)
	}
	return &dto.ChangePasswordResponse{Changed: true}, nil
}

func (s *authService) Refresh(ctx context.Context, req dto.RefreshTokenRequest) (*dto.TokenResponse, error) {
	refreshToken := normalizeRefreshToken(req.RefreshToken)
	if refreshToken == "" {
		return nil, response.TokenInvalidError(errmsg.MsgMissingSession)
	}
	claims, err := s.jwt.ParseRefreshToken(refreshToken)
	if err != nil {
		return nil, response.TokenInvalidError(errmsg.MsgMissingSession)
	}
	if claims.Subject != auth.SubjectRefresh {
		return nil, response.TokenInvalidError(errmsg.MsgMissingSession)
	}
	if _, err := uuid.Parse(claims.UserID); err != nil {
		return nil, response.TokenInvalidError(errmsg.MsgMissingSession)
	}
	session, err := s.sessions.GetSession(ctx, claims.SessionID)
	if err != nil {
		if errors.Is(err, ErrSessionNotFound) {
			return nil, response.TokenInvalidError(errmsg.MsgMissingSession)
		}
		return nil, response.FromError(err)
	}
	if session.UserID != claims.UserID || session.TokenVersion != claims.TokenVersion {
		return nil, response.TokenInvalidError(errmsg.MsgMissingSession)
	}
	currentVersion, err := s.sessions.GetCurrentTokenVersion(ctx, claims.UserID)
	if err != nil {
		return nil, response.FromError(err)
	}
	if currentVersion != session.TokenVersion {
		return nil, response.TokenInvalidError(errmsg.MsgMissingSession)
	}

	sessionID := session.SessionID
	if s.config.Auth.RefreshTokenRotation {
		if err := s.sessions.DeleteSession(ctx, claims.UserID, session.SessionID); err != nil {
			return nil, response.FromError(err)
		}
		sessionID = uuid.NewString()
	}
	return s.issueTokenPair(ctx, claims.UserID, currentVersion, sessionID)
}

func normalizeRefreshToken(token string) string {
	token = strings.TrimSpace(token)
	if len(token) >= len(auth.TokenPrefix) && strings.EqualFold(token[:len(auth.TokenPrefix)], auth.TokenPrefix) {
		return strings.TrimSpace(token[len(auth.TokenPrefix):])
	}
	return token
}

func (s *authService) Logout(ctx context.Context) (*dto.LogoutResponse, error) {
	userID, sessionID, err := authenticatedSession(ctx)
	if err != nil {
		return nil, err
	}
	if err := s.sessions.DeleteSession(ctx, userID, sessionID); err != nil {
		return nil, response.FromError(err)
	}
	return &dto.LogoutResponse{LoggedOut: true}, nil
}

func (s *authService) LogoutAll(ctx context.Context) (*dto.LogoutResponse, error) {
	userID, _, err := authenticatedSession(ctx)
	if err != nil {
		return nil, err
	}
	parsedUserID, err := uuid.Parse(userID)
	if err != nil {
		return nil, response.UnauthenticatedError(errmsg.MsgMissingSession)
	}
	if _, err := s.repo.IncrementTokenVersion(ctx, parsedUserID); err != nil {
		return nil, response.FromError(err)
	}
	if err := s.sessions.InvalidateUserTokenVersion(ctx, userID); err != nil {
		return nil, response.FromError(err)
	}
	if err := s.sessions.DeleteAllUserSessions(ctx, userID); err != nil {
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
		return nil, response.FromError(fmt.Errorf("sign access token: %w", err))
	}
	refresh, err := s.jwt.SignRefreshToken(auth.SignInput{UserID: userID, TokenVersion: tokenVersion, SessionID: sessionID, TTL: refreshTTL})
	if err != nil {
		return nil, response.FromError(fmt.Errorf("sign refresh token: %w", err))
	}
	if err := s.sessions.CreateSession(ctx, Session{UserID: userID, SessionID: sessionID, TokenVersion: tokenVersion, ExpiresAt: time.Now().Add(refreshTTL)}, refreshTTL); err != nil {
		return nil, response.FromError(err)
	}
	return &dto.TokenResponse{AccessToken: access, RefreshToken: refresh, TokenType: auth.TokenTypeBearer, ExpiresIn: int64(accessTTL.Seconds())}, nil
}

func (s *authService) issuePasswordChangeToken(_ context.Context, userID string, tokenVersion int64, sessionID string) (*dto.TokenResponse, error) {
	ttl := s.config.Auth.JWT.AccessTokenTTL
	if ttl <= 0 {
		ttl = defaultAccessTokenTTL
	}
	token, err := s.jwt.SignPasswordChangeToken(auth.SignInput{UserID: userID, TokenVersion: tokenVersion, SessionID: sessionID, TTL: ttl})
	if err != nil {
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
