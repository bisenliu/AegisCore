package app

import (
	"context"

	"github.com/aegiscore/common/runtime/config"
	"github.com/aegiscore/common/runtime/logger"
	commonauth "github.com/aegiscore/common/security/auth"
	authdomain "github.com/aegiscore/user-services/internal/features/auth/domain"
	"github.com/google/uuid"
	"go.uber.org/fx"
	"go.uber.org/zap"
)

// AuthServiceParams 包含构造认证服务所需的 Fx 输入。
type AuthServiceParams struct {
	fx.In

	Credentials   UserCredentialStore
	TokenVersions UserTokenVersionStore
	Sessions      AuthSessionStore
	JWT           *commonauth.JWTService
	Config        *config.Config
}

type authService struct {
	credentials          CredentialVerifier
	tokens               AuthTokenIssuer
	sessions             AuthSessionLifecycle
	refreshTokenRotation bool
}

// NewAuthService 组合凭证、token、会话和轮换依赖。
func NewAuthService(params AuthServiceParams) AuthService {
	return &authService{
		credentials:          newCredentialVerifier(params.Credentials),
		tokens:               newAuthTokenIssuer(params.JWT, params.Config),
		sessions:             newAuthSessionLifecycle(params.TokenVersions, params.Sessions),
		refreshTokenRotation: params.Config.Auth.RefreshTokenRotation,
	}
}

// Login 校验凭证，并签发普通 token 或受限改密 token。
func (s *authService) Login(ctx context.Context, cmd LoginCommand) (*TokenResult, error) {
	logger.Info(ctx, "login user", zap.String("username", cmd.Username))
	user, err := s.credentials.VerifyPassword(ctx, cmd.Username, cmd.Password)
	if err != nil {
		return nil, err
	}

	if user.RequiresPasswordChange() {
		// 必须改密用户只认证到可获取受限改密 token 的程度。
		logger.Warn(ctx, "login requires password change", zap.String("username", cmd.Username), zap.String("user_id", user.UserID.String()), zap.Int64("token_version", user.TokenVersion))
		return s.tokens.IssuePasswordChangeToken(ctx, user.UserID.String(), user.TokenVersion, uuid.NewString())
	}

	logger.Info(ctx, "login user authenticated", zap.String("username", cmd.Username), zap.String("user_id", user.UserID.String()), zap.Int64("token_version", user.TokenVersion))
	return s.issueTokenPair(ctx, user.UserID.String(), user.TokenVersion, uuid.NewString())
}

// ChangePassword 校验受限 token，更新凭证并撤销现有会话。
func (s *authService) ChangePassword(ctx context.Context, cmd ChangePasswordCommand) (*ChangePasswordResult, error) {
	parsedUserID, err := s.verifyPasswordChangeToken(ctx, cmd.Token)
	if err != nil {
		return nil, err
	}
	updated, err := s.credentials.ChangePassword(ctx, parsedUserID, cmd.NewPassword)
	if err != nil {
		return nil, err
	}
	if err := s.sessions.RevokeUserSessionsAtVersion(ctx, updated.UserID, updated.TokenVersion); err != nil {
		logger.Error(ctx, "password change session revocation projection failed", logger.StackTrace(zap.String("user_id", updated.UserID.String()), zap.Int64("token_version", updated.TokenVersion), zap.Error(err))...)
	}
	return &ChangePasswordResult{Changed: true}, nil
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
func (s *authService) Refresh(ctx context.Context, cmd RefreshTokenCommand) (*TokenResult, error) {
	claims, session, currentVersion, err := s.parseAndValidateRefreshSession(ctx, cmd.RefreshToken)
	if err != nil {
		return nil, err
	}
	if !s.refreshTokenRotation {
		return s.refreshWithoutRotation(ctx, claims, session, currentVersion)
	}
	return s.refreshWithRotation(ctx, claims, session, currentVersion)
}

func (s *authService) parseAndValidateRefreshSession(ctx context.Context, refreshToken string) (*commonauth.Claims, authdomain.AuthSession, int64, error) {
	claims, err := s.tokens.ParseRefreshToken(ctx, refreshToken)
	if err != nil {
		return nil, authdomain.AuthSession{}, 0, err
	}
	session, currentVersion, err := s.sessions.ValidateRefreshSession(ctx, claims)
	if err != nil {
		return nil, authdomain.AuthSession{}, 0, err
	}
	return claims, session, currentVersion, nil
}

func (s *authService) refreshWithoutRotation(ctx context.Context, claims *commonauth.Claims, session authdomain.AuthSession, currentVersion int64) (*TokenResult, error) {
	return s.issueTokenPair(ctx, claims.UserID, currentVersion, session.SessionID)
}

func (s *authService) refreshWithRotation(ctx context.Context, claims *commonauth.Claims, oldSession authdomain.AuthSession, currentVersion int64) (*TokenResult, error) {
	sessionID := uuid.NewString()
	tokens, err := s.tokens.IssueTokenPair(ctx, claims.UserID, currentVersion, sessionID)
	if err != nil {
		return nil, err
	}
	newSession := authdomain.AuthSession{UserID: claims.UserID, SessionID: sessionID, TokenVersion: currentVersion}
	if err := s.sessions.RotateTokenSession(ctx, oldSession, newSession, tokens.RefreshTTL); err != nil {
		return nil, err
	}
	return tokens.Response, nil
}

// Logout 撤销当前 refresh token 会话，但不修改用户 token version。
func (s *authService) Logout(ctx context.Context) (*LogoutResult, error) {
	userID, sessionID, err := authenticatedSession(ctx)
	if err != nil {
		logger.Warn(ctx, "logout missing authenticated session", zap.Error(err))
		return nil, err
	}
	if err := s.sessions.DeleteSession(ctx, userID, sessionID); err != nil {
		return nil, err
	}
	return &LogoutResult{LoggedOut: true}, nil
}

// LogoutAll 递增认证用户的 token version，并移除全部 refresh 会话。
func (s *authService) LogoutAll(ctx context.Context) (*LogoutResult, error) {
	userID, _, err := authenticatedSession(ctx)
	if err != nil {
		logger.Warn(ctx, "logout all missing authenticated session", zap.Error(err))
		return nil, err
	}
	parsedUserID, err := uuid.Parse(userID)
	if err != nil {
		logger.Warn(ctx, "logout all user id invalid", zap.String("user_id", userID))
		return nil, authdomain.ErrMissingSession
	}
	if _, err = s.sessions.RevokeAllUserSessions(ctx, parsedUserID); err != nil {
		return nil, err
	}
	return &LogoutResult{LoggedOut: true}, nil
}

func (s *authService) issueTokenPair(ctx context.Context, userID string, tokenVersion int64, sessionID string) (*TokenResult, error) {
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
	userIDString, ok := commonauth.UserIDFromContext(ctx)
	if !ok {
		return "", "", authdomain.ErrMissingSession
	}
	if _, err := uuid.Parse(userIDString); err != nil {
		return "", "", authdomain.ErrMissingSession
	}
	sessionID, ok := commonauth.SessionIDFromContext(ctx)
	if !ok {
		return "", "", authdomain.ErrMissingSession
	}
	return userIDString, sessionID, nil
}
