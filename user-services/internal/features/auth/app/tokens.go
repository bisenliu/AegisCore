package app

import (
	"context"
	"fmt"
	"time"

	"github.com/aegiscore/common/contract/response"
	"github.com/aegiscore/common/runtime/config"
	"github.com/aegiscore/common/runtime/logger"
	commonauth "github.com/aegiscore/common/security/auth"
	authapi "github.com/aegiscore/user-services/internal/features/auth/api"
	"github.com/aegiscore/user-services/internal/messages"
	"github.com/google/uuid"
	"go.uber.org/zap"
)

const (
	// defaultAccessTokenTTL 是配置为非正数时 access/改密 token 的兜底生命周期。
	defaultAccessTokenTTL = 15 * time.Minute
	// defaultRefreshTokenTTL 是配置为非正数时 refresh 会话的兜底生命周期。
	defaultRefreshTokenTTL = 7 * 24 * time.Hour
)

// AuthTokenIssuer 签发和解析认证流程使用的 JWT。
type AuthTokenIssuer interface {
	IssueTokenPair(ctx context.Context, userID string, tokenVersion int64, sessionID string) (*issuedTokenPair, error)
	IssuePasswordChangeToken(ctx context.Context, userID string, tokenVersion int64, sessionID string) (*authapi.TokenResponse, error)
	ParseRefreshToken(ctx context.Context, token string) (*commonauth.Claims, error)
	ParsePasswordChangeToken(ctx context.Context, token string) (*commonauth.Claims, uuid.UUID, error)
}

type issuedTokenPair struct {
	Response   *authapi.TokenResponse
	RefreshTTL time.Duration
}

type authTokenIssuer struct {
	jwt    *commonauth.JWTService
	config *config.Config
}

func newAuthTokenIssuer(jwt *commonauth.JWTService, cfg *config.Config) AuthTokenIssuer {
	return &authTokenIssuer{jwt: jwt, config: cfg}
}

// IssueTokenPair 为一个认证会话签发 access 和 refresh token。
func (i *authTokenIssuer) IssueTokenPair(ctx context.Context, userID string, tokenVersion int64, sessionID string) (*issuedTokenPair, error) {
	accessTTL := i.accessTokenTTL()
	refreshTTL := i.refreshTokenTTL()
	access, err := i.jwt.SignAccessToken(commonauth.SignInput{UserID: userID, TokenVersion: tokenVersion, SessionID: sessionID, TTL: accessTTL})
	if err != nil {
		logger.Error(ctx, "sign access token failed", logger.StackTrace(zap.String("user_id", userID), zap.String("session_id", sessionID), zap.Int64("token_version", tokenVersion), zap.Error(err))...)
		return nil, response.FromError(fmt.Errorf("sign access token: %w", err))
	}
	refresh, err := i.jwt.SignRefreshToken(commonauth.SignInput{UserID: userID, TokenVersion: tokenVersion, SessionID: sessionID, TTL: refreshTTL})
	if err != nil {
		logger.Error(ctx, "sign refresh token failed", logger.StackTrace(zap.String("user_id", userID), zap.String("session_id", sessionID), zap.Int64("token_version", tokenVersion), zap.Error(err))...)
		return nil, response.FromError(fmt.Errorf("sign refresh token: %w", err))
	}
	return &issuedTokenPair{
		Response:   &authapi.TokenResponse{AccessToken: access, RefreshToken: refresh, TokenType: commonauth.TokenTypeBearer, ExpiresIn: int64(accessTTL.Seconds())},
		RefreshTTL: refreshTTL,
	}, nil
}

// IssuePasswordChangeToken 签发受限 token，并有意不返回 refresh token。
func (i *authTokenIssuer) IssuePasswordChangeToken(ctx context.Context, userID string, tokenVersion int64, sessionID string) (*authapi.TokenResponse, error) {
	ttl := i.accessTokenTTL()
	token, err := i.jwt.SignPasswordChangeToken(commonauth.SignInput{UserID: userID, TokenVersion: tokenVersion, SessionID: sessionID, TTL: ttl})
	if err != nil {
		logger.Error(ctx, "sign password change token failed", logger.StackTrace(zap.String("user_id", userID), zap.String("session_id", sessionID), zap.Int64("token_version", tokenVersion), zap.Error(err))...)
		return nil, response.FromError(fmt.Errorf("sign password change token: %w", err))
	}
	return &authapi.TokenResponse{AccessToken: token, TokenType: commonauth.TokenTypeBearer, ExpiresIn: int64(ttl.Seconds()), PasswordChangeRequired: true}, nil
}

// ParseRefreshToken 规范化可选 Bearer 输入并校验 refresh token claims。
func (i *authTokenIssuer) ParseRefreshToken(ctx context.Context, token string) (*commonauth.Claims, error) {
	claims, err := i.jwt.ParseRefreshToken(commonauth.StripBearerPrefix(token))
	if err != nil {
		logger.Warn(ctx, "refresh token invalid", zap.Bool("token_present", token != ""))
		return nil, response.TokenInvalidError(messages.MissingSession)
	}
	if claims.Subject != commonauth.SubjectRefresh {
		logger.Warn(ctx, "refresh token subject rejected", zap.String("user_id", claims.UserID), zap.String("session_id", claims.SessionID), zap.String("subject", claims.Subject))
		return nil, response.TokenInvalidError(messages.MissingSession)
	}
	if _, err := uuid.Parse(claims.UserID); err != nil {
		logger.Warn(ctx, "refresh token user id invalid", zap.String("user_id", claims.UserID), zap.String("session_id", claims.SessionID))
		return nil, response.TokenInvalidError(messages.MissingSession)
	}
	return claims, nil
}

// ParsePasswordChangeToken 规范化可选 Bearer 输入，并返回已校验的改密 claims。
func (i *authTokenIssuer) ParsePasswordChangeToken(ctx context.Context, token string) (*commonauth.Claims, uuid.UUID, error) {
	claims, err := i.jwt.ParsePasswordChangeToken(commonauth.StripBearerPrefix(token))
	if err != nil {
		logger.Warn(ctx, "password change token invalid", zap.Bool("token_present", token != ""))
		return nil, uuid.Nil, response.TokenInvalidError(messages.MissingSession)
	}
	parsedUserID, err := uuid.Parse(claims.UserID)
	if err != nil {
		logger.Warn(ctx, "password change token user id invalid", zap.String("user_id", claims.UserID), zap.String("session_id", claims.SessionID))
		return nil, uuid.Nil, response.TokenInvalidError(messages.MissingSession)
	}
	return claims, parsedUserID, nil
}

func (i *authTokenIssuer) accessTokenTTL() time.Duration {
	ttl := i.config.Auth.JWT.AccessTokenTTL
	if ttl <= 0 {
		// 非正数 TTL 配置使用默认值，确保签发 token 始终会过期。
		return defaultAccessTokenTTL
	}
	return ttl
}

func (i *authTokenIssuer) refreshTokenTTL() time.Duration {
	ttl := i.config.Auth.JWT.RefreshTokenTTL
	if ttl <= 0 {
		// 非正数 TTL 配置使用默认值，确保 refresh 会话始终会过期。
		return defaultRefreshTokenTTL
	}
	return ttl
}
