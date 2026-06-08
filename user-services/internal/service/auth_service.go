package service

import (
	"context"

	"github.com/aegiscore/common/contract/response"
	"github.com/aegiscore/common/runtime/config"
	"github.com/aegiscore/common/runtime/logger"
	"github.com/aegiscore/common/security/auth"
	"github.com/aegiscore/user-services/internal/api/auth"
	"github.com/aegiscore/user-services/internal/messages"
	"github.com/aegiscore/user-services/internal/repository"
	"github.com/google/uuid"
	"go.uber.org/fx"
	"go.uber.org/zap"
)

// AuthService 定义认证、刷新、改密和登出用例。
type AuthService interface {
	Login(ctx context.Context, req authapi.LoginRequest) (*authapi.TokenResponse, error)
	ChangePassword(ctx context.Context, req authapi.ChangePasswordRequest) (*authapi.ChangePasswordResponse, error)
	Refresh(ctx context.Context, req authapi.RefreshTokenRequest) (*authapi.TokenResponse, error)
	Logout(ctx context.Context) (*authapi.LogoutResponse, error)
	LogoutAll(ctx context.Context) (*authapi.LogoutResponse, error)
}

// AuthServiceParams 包含构造认证服务所需的 Fx 输入。
type AuthServiceParams struct {
	fx.In

	Credentials   repository.UserCredentialRepository
	TokenVersions repository.UserTokenVersionRepository
	Sessions      repository.AuthSessionRepository
	JWT           *auth.JWTService
	Config        *config.Config
}

type authService struct {
	credentials          CredentialVerifier
	tokens               AuthTokenIssuer
	sessions             AuthSessionManager
	refreshTokenRotation bool
}

// NewAuthService 组合凭证、token、会话和轮换依赖。
func NewAuthService(params AuthServiceParams) AuthService {
	return &authService{
		credentials:          newCredentialVerifier(params.Credentials),
		tokens:               newAuthTokenIssuer(params.JWT, params.Config),
		sessions:             newAuthSessionManager(params.TokenVersions, params.Sessions),
		refreshTokenRotation: params.Config.Auth.RefreshTokenRotation,
	}
}

// Login 校验凭证，并签发普通 token 或受限改密 token。
func (s *authService) Login(ctx context.Context, req authapi.LoginRequest) (*authapi.TokenResponse, error) {
	logger.Info(ctx, "login user", zap.String("username", req.Username))
	user, err := s.credentials.VerifyPassword(ctx, req.Username, req.Password)
	if err != nil {
		return nil, err
	}

	if user.RequiresPasswordChange() {
		// 必须改密用户只认证到可获取受限改密 token 的程度。
		logger.Warn(ctx, "login requires password change", zap.String("username", req.Username), zap.String("user_id", user.UserID.String()), zap.Int64("token_version", user.TokenVersion))
		return s.tokens.IssuePasswordChangeToken(ctx, user.UserID.String(), user.TokenVersion, uuid.NewString())
	}

	logger.Info(ctx, "login user authenticated", zap.String("username", req.Username), zap.String("user_id", user.UserID.String()), zap.Int64("token_version", user.TokenVersion))
	return s.issueTokenPair(ctx, user.UserID.String(), user.TokenVersion, uuid.NewString())
}

// ChangePassword 校验受限 token，更新凭证并撤销现有会话。
func (s *authService) ChangePassword(ctx context.Context, req authapi.ChangePasswordRequest) (*authapi.ChangePasswordResponse, error) {
	parsedUserID, err := s.verifyPasswordChangeToken(ctx, req.Token)
	if err != nil {
		return nil, err
	}
	if _, err := s.credentials.ChangePassword(ctx, parsedUserID, req.NewPassword); err != nil {
		return nil, err
	}
	if _, err := s.sessions.RevokeAllUserSessions(ctx, parsedUserID); err != nil {
		return nil, err
	}
	return &authapi.ChangePasswordResponse{Changed: true}, nil
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

// Refresh 校验 refresh 会话并签发新的 token 响应。
func (s *authService) Refresh(ctx context.Context, req authapi.RefreshTokenRequest) (*authapi.TokenResponse, error) {
	claims, err := s.tokens.ParseRefreshToken(ctx, req.RefreshToken)
	if err != nil {
		return nil, err
	}
	session, currentVersion, err := s.sessions.ValidateRefreshSession(ctx, claims)
	if err != nil {
		return nil, err
	}

	sessionID := session.SessionID
	if s.refreshTokenRotation {
		// 先创建新会话再删除旧会话，确保 token 签发失败时旧会话仍可使用。
		sessionID = uuid.NewString()
		tokens, err := s.issueTokenPair(ctx, claims.UserID, currentVersion, sessionID)
		if err != nil {
			return nil, err
		}
		if err := s.sessions.DeleteSession(ctx, claims.UserID, session.SessionID); err != nil {
			if cleanupErr := s.sessions.DeleteSession(ctx, claims.UserID, sessionID); cleanupErr != nil {
				logger.Warn(ctx, "cleanup rotated auth session failed", zap.String("user_id", claims.UserID), zap.String("session_id", sessionID), zap.Error(cleanupErr))
			}
			return nil, err
		}
		return tokens, nil
	}
	return s.issueTokenPair(ctx, claims.UserID, currentVersion, sessionID)
}

// Logout 撤销当前 refresh token 会话，但不修改用户 token version。
func (s *authService) Logout(ctx context.Context) (*authapi.LogoutResponse, error) {
	userID, sessionID, err := authenticatedSession(ctx)
	if err != nil {
		logger.Warn(ctx, "logout missing authenticated session", zap.Error(err))
		return nil, err
	}
	if err := s.sessions.DeleteSession(ctx, userID, sessionID); err != nil {
		return nil, err
	}
	return &authapi.LogoutResponse{LoggedOut: true}, nil
}

// LogoutAll 递增认证用户的 token version，并移除全部 refresh 会话。
func (s *authService) LogoutAll(ctx context.Context) (*authapi.LogoutResponse, error) {
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
	if _, err = s.sessions.RevokeAllUserSessions(ctx, parsedUserID); err != nil {
		return nil, err
	}
	return &authapi.LogoutResponse{LoggedOut: true}, nil
}

func (s *authService) issueTokenPair(ctx context.Context, userID string, tokenVersion int64, sessionID string) (*authapi.TokenResponse, error) {
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
