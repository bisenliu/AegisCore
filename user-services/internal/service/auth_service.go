package service

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/aegiscore/common/config"
	"github.com/aegiscore/common/contextutil"
	commonjwt "github.com/aegiscore/common/jwt"
	"github.com/aegiscore/common/logger"
	commonpassword "github.com/aegiscore/common/password"
	"github.com/aegiscore/common/response"
	"github.com/aegiscore/user-services/internal/apperror"
	"github.com/aegiscore/user-services/internal/dto"
	"github.com/aegiscore/user-services/internal/repository"
	"github.com/google/uuid"
	"go.uber.org/fx"
	"go.uber.org/zap"
)

type AuthService interface {
	Login(ctx context.Context, req dto.LoginRequest) (*dto.TokenResponse, error)
	Refresh(ctx context.Context, req dto.RefreshTokenRequest) (*dto.TokenResponse, error)
	Logout(ctx context.Context) (*dto.LogoutResponse, error)
	LogoutAll(ctx context.Context) (*dto.LogoutResponse, error)
}

type AuthServiceParams struct {
	fx.In

	Repo     repository.UserRepository
	Sessions SessionStore
	JWT      *commonjwt.Service
	Config   *config.Config
}

type authService struct {
	repo     repository.UserRepository
	sessions SessionStore
	jwt      *commonjwt.Service
	config   *config.Config
}

func NewAuthService(params AuthServiceParams) AuthService {
	return &authService{repo: params.Repo, sessions: params.Sessions, jwt: params.JWT, config: params.Config}
}

func (s *authService) Login(ctx context.Context, req dto.LoginRequest) (*dto.TokenResponse, error) {
	username := strings.TrimSpace(req.Username)
	plainPassword := strings.TrimSpace(req.Password)
	if username == "" || plainPassword == "" {
		return nil, response.UnauthenticatedError(apperror.MsgInvalidCredentials)
	}

	user, err := s.repo.GetByUsername(ctx, username)
	if err != nil {
		appErr := response.FromError(err)
		if appErr.Code == response.CodeNotFound {
			return nil, response.UnauthenticatedError(apperror.MsgInvalidCredentials)
		}
		logger.Error(ctx, "query login user failed", zap.String("username", username), zap.Error(err))
		return nil, response.FromError(err)
	}
	matched, err := commonpassword.Verify(plainPassword, user.Password)
	if err != nil {
		logger.Error(ctx, "verify login password failed", zap.String("username", username), zap.Error(err))
		return nil, response.UnauthenticatedError(apperror.MsgInvalidCredentials)
	}
	if !matched {
		return nil, response.UnauthenticatedError(apperror.MsgInvalidCredentials)
	}

	return s.issueTokenPair(ctx, user.UserID.String(), user.TokenVersion, uuid.NewString())
}

func (s *authService) Refresh(ctx context.Context, req dto.RefreshTokenRequest) (*dto.TokenResponse, error) {
	refreshToken := normalizeRefreshToken(req.RefreshToken)
	if refreshToken == "" {
		return nil, response.TokenInvalidError(apperror.MsgMissingSession)
	}
	claims, err := s.jwt.ParseRefreshToken(refreshToken)
	if err != nil {
		return nil, response.TokenInvalidError(apperror.MsgMissingSession)
	}
	if claims.Subject != commonjwt.SubjectRefresh {
		return nil, response.TokenInvalidError(apperror.MsgMissingSession)
	}
	if _, err := uuid.Parse(claims.UserID); err != nil {
		return nil, response.TokenInvalidError(apperror.MsgMissingSession)
	}
	session, err := s.sessions.GetSession(ctx, claims.SessionID)
	if err != nil {
		if errors.Is(err, ErrSessionNotFound) {
			return nil, response.TokenInvalidError(apperror.MsgMissingSession)
		}
		return nil, response.FromError(err)
	}
	if session.UserID != claims.UserID || session.TokenVersion != claims.TokenVersion {
		return nil, response.TokenInvalidError(apperror.MsgMissingSession)
	}
	currentVersion, err := s.sessions.GetCurrentTokenVersion(ctx, claims.UserID)
	if err != nil {
		return nil, response.FromError(err)
	}
	if currentVersion != session.TokenVersion {
		return nil, response.TokenInvalidError(apperror.MsgMissingSession)
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
	if len(token) >= len(contextutil.TokenPrefix) && strings.EqualFold(token[:len(contextutil.TokenPrefix)], contextutil.TokenPrefix) {
		return strings.TrimSpace(token[len(contextutil.TokenPrefix):])
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
		return nil, response.UnauthenticatedError(apperror.MsgMissingSession)
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
		accessTTL = 15 * time.Minute
	}
	refreshTTL := s.config.Auth.JWT.RefreshTokenTTL
	if refreshTTL <= 0 {
		refreshTTL = 7 * 24 * time.Hour
	}
	access, err := s.jwt.SignAccessToken(commonjwt.SignInput{UserID: userID, TokenVersion: tokenVersion, SessionID: sessionID, TTL: accessTTL})
	if err != nil {
		return nil, response.FromError(fmt.Errorf("sign access token: %w", err))
	}
	refresh, err := s.jwt.SignRefreshToken(commonjwt.SignInput{UserID: userID, TokenVersion: tokenVersion, SessionID: sessionID, TTL: refreshTTL})
	if err != nil {
		return nil, response.FromError(fmt.Errorf("sign refresh token: %w", err))
	}
	if err := s.sessions.CreateSession(ctx, Session{UserID: userID, SessionID: sessionID, TokenVersion: tokenVersion, ExpiresAt: time.Now().Add(refreshTTL)}, refreshTTL); err != nil {
		return nil, response.FromError(err)
	}
	return &dto.TokenResponse{AccessToken: access, RefreshToken: refresh, TokenType: commonjwt.TokenTypeBearer, ExpiresIn: int64(accessTTL.Seconds())}, nil
}

func authenticatedSession(ctx context.Context) (string, string, error) {
	userIDString, ok := contextutil.UserIDFromContext(ctx)
	if !ok {
		return "", "", response.UnauthenticatedError(apperror.MsgMissingSession)
	}
	if _, err := uuid.Parse(userIDString); err != nil {
		return "", "", response.UnauthenticatedError(apperror.MsgMissingSession)
	}
	sessionID, ok := contextutil.SessionIDFromContext(ctx)
	if !ok {
		return "", "", response.UnauthenticatedError(apperror.MsgMissingSession)
	}
	return userIDString, sessionID, nil
}
