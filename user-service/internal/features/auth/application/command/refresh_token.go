package command

import (
	"context"
	"errors"

	platformauth "github.com/aegiscore/platform/security/auth"
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

// parseAndValidateRefreshSession 检查续期凭证的真实性并核对服务器上的会话记录；若会话已被注销或版本落后则拒绝续期。
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

// refreshWithoutRotation 在不更换续期凭证的模式下延长登录状态，仅换发新的短期访问凭证。
func (u *refreshTokenUseCase) refreshWithoutRotation(ctx context.Context, claims *authtokens.Claims, session authdomain.AuthSession, currentVersion int64) (*authtokens.TokenResult, error) {
	tokens, reason, err := issueTokenPair(ctx, u.tokens, u.sessions, claims.UserID, currentVersion, session.SessionID)
	if err != nil {
		u.metrics.RefreshFailed(ctx, reason)
		return nil, err
	}
	u.metrics.RefreshSucceeded(ctx)
	return tokens, nil
}

// refreshWithRotation 在续期时同时换发新的续期凭证，并让旧的续期凭证立即作废，提升安全性。
func (u *refreshTokenUseCase) refreshWithRotation(ctx context.Context, claims *authtokens.Claims, oldSession authdomain.AuthSession, currentVersion int64) (*authtokens.TokenResult, error) {
	sessionID, err := newAuthSessionID()
	if err != nil {
		u.metrics.RefreshFailed(ctx, authapplication.MetricsReasonTokenIssueFailed)
		return nil, err
	}
	issued, err := u.tokens.IssueTokenPair(ctx, claims.UserID, currentVersion, sessionID)
	if err != nil {
		u.metrics.RefreshFailed(ctx, authapplication.MetricsReasonTokenIssueFailed)
		return nil, err
	}
	// 先签发 token 再轮换 session；若 Redis 轮换失败，调用方不会拿到 token 响应，避免产生没有服务端会话投影的 refresh token。
	newSession := authdomain.AuthSession{UserID: claims.UserID, SessionID: sessionID, TokenVersion: currentVersion}
	if err := u.sessions.RotateTokenSession(ctx, oldSession, newSession, issued.RefreshTTL); err != nil {
		u.metrics.RefreshFailed(ctx, authapplication.MetricsReasonSessionRotateFailed)
		return nil, err
	}
	u.metrics.RefreshSucceeded(ctx)
	return issued.Result, nil
}

// refreshFailureReason 将凭证续期失败的原因归纳为统一标识，供系统监控统计使用。
func refreshFailureReason(err error) string {
	switch {
	case errors.Is(err, platformauth.ErrTokenVersionMismatch):
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
