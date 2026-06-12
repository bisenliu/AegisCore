package command

import (
	"context"

	"github.com/google/uuid"

	commonauth "github.com/aegiscore/common/security/auth"
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
	deps *UseCaseDeps
}

// NewRefreshTokenUseCase 构造 refresh token use case。
func NewRefreshTokenUseCase(deps *UseCaseDeps) RefreshTokenUseCase {
	return &refreshTokenUseCase{deps: deps}
}

// Refresh 校验 refresh 会话并签发新的 token 响应。
func (u *refreshTokenUseCase) Refresh(ctx context.Context, cmd RefreshTokenCommand) (*authtokens.TokenResult, error) {
	if err := authvalidators.ValidateRefreshToken(cmd.RefreshToken); err != nil {
		return nil, err
	}

	claims, session, currentVersion, err := u.parseAndValidateRefreshSession(ctx, cmd.RefreshToken)
	if err != nil {
		return nil, err
	}
	if !u.deps.refreshTokenRotation {
		return u.refreshWithoutRotation(ctx, claims, session, currentVersion)
	}
	return u.refreshWithRotation(ctx, claims, session, currentVersion)
}

func (u *refreshTokenUseCase) parseAndValidateRefreshSession(ctx context.Context, refreshToken string) (*commonauth.Claims, authdomain.AuthSession, int64, error) {
	claims, err := u.deps.tokens.ParseRefreshToken(ctx, refreshToken)
	if err != nil {
		return nil, authdomain.AuthSession{}, 0, err
	}
	session, currentVersion, err := u.deps.sessions.ValidateRefreshSession(ctx, claims)
	if err != nil {
		return nil, authdomain.AuthSession{}, 0, err
	}
	return claims, session, currentVersion, nil
}

func (u *refreshTokenUseCase) refreshWithoutRotation(ctx context.Context, claims *commonauth.Claims, session authdomain.AuthSession, currentVersion int64) (*authtokens.TokenResult, error) {
	return u.deps.issueTokenPair(ctx, claims.UserID, currentVersion, session.SessionID)
}

func (u *refreshTokenUseCase) refreshWithRotation(ctx context.Context, claims *commonauth.Claims, oldSession authdomain.AuthSession, currentVersion int64) (*authtokens.TokenResult, error) {
	sessionID := uuid.NewString()
	tokens, err := u.deps.tokens.IssueTokenPair(ctx, claims.UserID, currentVersion, sessionID)
	if err != nil {
		return nil, err
	}
	newSession := authdomain.AuthSession{UserID: claims.UserID, SessionID: sessionID, TokenVersion: currentVersion}
	if err := u.deps.sessions.RotateTokenSession(ctx, oldSession, newSession, tokens.RefreshTTL); err != nil {
		return nil, err
	}
	return tokens.Response, nil
}
