package command

import (
	"context"
	"errors"

	commonauth "github.com/aegiscore/common/security/auth"
	authapplication "github.com/aegiscore/user-service/internal/features/auth/application"
	authsessions "github.com/aegiscore/user-service/internal/features/auth/application/sessions"
	authtokens "github.com/aegiscore/user-service/internal/features/auth/application/tokens"
	authvalidators "github.com/aegiscore/user-service/internal/features/auth/application/validators"
	authdomain "github.com/aegiscore/user-service/internal/features/auth/domain"
)

// RefreshTokenUseCase 处理 refresh token 续签。
type RefreshTokenUseCase interface {
	Refresh(ctx context.Context, cmd RefreshTokenCommand) (*authtokens.TokenResult, error)
}

// RefreshTokenCommand 是换取 refresh token 的应用层输入。
type RefreshTokenCommand struct {
	RefreshToken string
}

type refreshTokenUseCase struct {
	tokens               authtokens.Issuer
	sessions             authsessions.Lifecycle
	metrics              authapplication.Metrics
	refreshTokenRotation bool
}

// NewRefreshTokenUseCase 构造 refresh token use case。
func NewRefreshTokenUseCase(tokens authtokens.Issuer, sessions authsessions.Lifecycle, metrics authapplication.Metrics, settings RefreshTokenSettings) RefreshTokenUseCase {
	return &refreshTokenUseCase{
		tokens:               tokens,
		sessions:             sessions,
		metrics:              metricsOrNop(metrics),
		refreshTokenRotation: settings.RefreshTokenRotation,
	}
}

// Refresh 校验 refresh 会话并签发新的 token 响应。
func (u *refreshTokenUseCase) Refresh(ctx context.Context, cmd RefreshTokenCommand) (*authtokens.TokenResult, error) {
	if err := authvalidators.ValidateRefreshToken(cmd.RefreshToken); err != nil {
		u.metrics.RefreshFailed(ctx, authapplication.MetricsReasonValidationFailed)
		return nil, err
	}

	claims, session, currentVersion, err := u.parseAndValidateRefreshSession(ctx, cmd.RefreshToken)
	if err != nil {
		reason := refreshFailureReason(err)
		u.metrics.RefreshFailed(ctx, reason)
		if reason == authapplication.MetricsReasonTokenVersionMismatch {
			u.metrics.TokenVersionMismatch(ctx, authapplication.MetricsSourceRefreshToken)
		}
		return nil, err
	}
	// 关闭 rotation 时沿用原 refresh session；开启后必须原子消费旧 session 并创建新 session，以缩短 refresh token 泄漏后的重放窗口。
	if !u.refreshTokenRotation {
		return u.refreshWithoutRotation(ctx, claims, session, currentVersion)
	}
	return u.refreshWithRotation(ctx, claims, session, currentVersion)
}

func (u *refreshTokenUseCase) parseAndValidateRefreshSession(ctx context.Context, refreshToken string) (*authtokens.Claims, authdomain.AuthSession, int64, error) {
	claims, _, err := u.tokens.ParseRefreshToken(ctx, refreshToken)
	if err != nil {
		return nil, authdomain.AuthSession{}, 0, err
	}
	session, currentVersion, err := u.sessions.ValidateRefreshSession(ctx, claims)
	if err != nil {
		return nil, authdomain.AuthSession{}, 0, err
	}
	return claims, session, currentVersion, nil
}

func (u *refreshTokenUseCase) refreshWithoutRotation(ctx context.Context, claims *authtokens.Claims, session authdomain.AuthSession, currentVersion int64) (*authtokens.TokenResult, error) {
	tokens, reason, err := issueTokenPair(ctx, u.tokens, u.sessions, claims.UserID, currentVersion, session.SessionID)
	if err != nil {
		u.metrics.RefreshFailed(ctx, reason)
		return nil, err
	}
	u.metrics.RefreshSucceeded(ctx)
	return tokens, nil
}

func (u *refreshTokenUseCase) refreshWithRotation(ctx context.Context, claims *authtokens.Claims, oldSession authdomain.AuthSession, currentVersion int64) (*authtokens.TokenResult, error) {
	sessionID, err := newAuthSessionID()
	if err != nil {
		u.metrics.RefreshFailed(ctx, authapplication.MetricsReasonTokenIssueFailed)
		return nil, err
	}
	tokens, err := u.tokens.IssueTokenPair(ctx, claims.UserID, currentVersion, sessionID)
	if err != nil {
		u.metrics.RefreshFailed(ctx, authapplication.MetricsReasonTokenIssueFailed)
		return nil, err
	}
	// 先签发 token 再轮换 session；若 Redis 轮换失败，调用方不会拿到 token 响应，避免产生没有服务端会话投影的 refresh token。
	newSession := authdomain.AuthSession{UserID: claims.UserID, SessionID: sessionID, TokenVersion: currentVersion}
	if err := u.sessions.RotateTokenSession(ctx, oldSession, newSession, tokens.RefreshTTL); err != nil {
		u.metrics.RefreshFailed(ctx, authapplication.MetricsReasonSessionRotateFailed)
		return nil, err
	}
	u.metrics.RefreshSucceeded(ctx)
	return tokens.Response, nil
}

func refreshFailureReason(err error) string {
	switch {
	case errors.Is(err, commonauth.ErrTokenVersionMismatch):
		return authapplication.MetricsReasonTokenVersionMismatch
	case errors.Is(err, authdomain.ErrAuthSessionNotFound):
		return authapplication.MetricsReasonRefreshSessionInvalid
	case errors.Is(err, authdomain.ErrAuthSessionMismatch):
		return authapplication.MetricsReasonRefreshSessionMismatch
	case errors.Is(err, authdomain.ErrTokenInvalid):
		return authapplication.MetricsReasonRefreshTokenInvalid
	default:
		return authapplication.MetricsReasonSystemError
	}
}
