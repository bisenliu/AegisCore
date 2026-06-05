package service

import (
	"context"
	"fmt"
	"time"

	"github.com/aegiscore/common/contract/response"
	"github.com/aegiscore/common/runtime/config"
	"github.com/aegiscore/common/runtime/logger"
	"github.com/aegiscore/common/security/auth"
	"github.com/aegiscore/user-services/internal/dto"
	"github.com/aegiscore/user-services/internal/messages"
	"github.com/google/uuid"
	"go.uber.org/zap"
)

const (
	defaultAccessTokenTTL  = 15 * time.Minute
	defaultRefreshTokenTTL = 7 * 24 * time.Hour
)

type AuthTokenIssuer interface {
	IssueTokenPair(ctx context.Context, userID string, tokenVersion int64, sessionID string) (*issuedTokenPair, error)
	IssuePasswordChangeToken(ctx context.Context, userID string, tokenVersion int64, sessionID string) (*dto.TokenResponse, error)
	ParseRefreshToken(ctx context.Context, token string) (*auth.Claims, error)
	ParsePasswordChangeToken(ctx context.Context, token string) (*auth.Claims, uuid.UUID, error)
}

type issuedTokenPair struct {
	Response   *dto.TokenResponse
	RefreshTTL time.Duration
}

type authTokenIssuer struct {
	jwt    *auth.JWTService
	config *config.Config
}

func newAuthTokenIssuer(jwt *auth.JWTService, cfg *config.Config) AuthTokenIssuer {
	return &authTokenIssuer{jwt: jwt, config: cfg}
}

func (i *authTokenIssuer) IssueTokenPair(ctx context.Context, userID string, tokenVersion int64, sessionID string) (*issuedTokenPair, error) {
	accessTTL := i.accessTokenTTL()
	refreshTTL := i.refreshTokenTTL()
	access, err := i.jwt.SignAccessToken(auth.SignInput{UserID: userID, TokenVersion: tokenVersion, SessionID: sessionID, TTL: accessTTL})
	if err != nil {
		logger.Error(ctx, "sign access token failed", logger.StackTrace(zap.String("user_id", userID), zap.String("session_id", sessionID), zap.Int64("token_version", tokenVersion), zap.Error(err))...)
		return nil, response.FromError(fmt.Errorf("sign access token: %w", err))
	}
	refresh, err := i.jwt.SignRefreshToken(auth.SignInput{UserID: userID, TokenVersion: tokenVersion, SessionID: sessionID, TTL: refreshTTL})
	if err != nil {
		logger.Error(ctx, "sign refresh token failed", logger.StackTrace(zap.String("user_id", userID), zap.String("session_id", sessionID), zap.Int64("token_version", tokenVersion), zap.Error(err))...)
		return nil, response.FromError(fmt.Errorf("sign refresh token: %w", err))
	}
	return &issuedTokenPair{
		Response:   &dto.TokenResponse{AccessToken: access, RefreshToken: refresh, TokenType: auth.TokenTypeBearer, ExpiresIn: int64(accessTTL.Seconds())},
		RefreshTTL: refreshTTL,
	}, nil
}

func (i *authTokenIssuer) IssuePasswordChangeToken(ctx context.Context, userID string, tokenVersion int64, sessionID string) (*dto.TokenResponse, error) {
	ttl := i.accessTokenTTL()
	token, err := i.jwt.SignPasswordChangeToken(auth.SignInput{UserID: userID, TokenVersion: tokenVersion, SessionID: sessionID, TTL: ttl})
	if err != nil {
		logger.Error(ctx, "sign password change token failed", logger.StackTrace(zap.String("user_id", userID), zap.String("session_id", sessionID), zap.Int64("token_version", tokenVersion), zap.Error(err))...)
		return nil, response.FromError(fmt.Errorf("sign password change token: %w", err))
	}
	return &dto.TokenResponse{AccessToken: token, TokenType: auth.TokenTypeBearer, ExpiresIn: int64(ttl.Seconds()), PasswordChangeRequired: true}, nil
}

func (i *authTokenIssuer) ParseRefreshToken(ctx context.Context, token string) (*auth.Claims, error) {
	claims, err := i.jwt.ParseRefreshToken(auth.StripBearerPrefix(token))
	if err != nil {
		logger.Warn(ctx, "refresh token invalid", zap.Bool("token_present", token != ""))
		return nil, response.TokenInvalidError(messages.MissingSession)
	}
	if claims.Subject != auth.SubjectRefresh {
		logger.Warn(ctx, "refresh token subject rejected", zap.String("user_id", claims.UserID), zap.String("session_id", claims.SessionID), zap.String("subject", claims.Subject))
		return nil, response.TokenInvalidError(messages.MissingSession)
	}
	if _, err := uuid.Parse(claims.UserID); err != nil {
		logger.Warn(ctx, "refresh token user id invalid", zap.String("user_id", claims.UserID), zap.String("session_id", claims.SessionID))
		return nil, response.TokenInvalidError(messages.MissingSession)
	}
	return claims, nil
}

func (i *authTokenIssuer) ParsePasswordChangeToken(ctx context.Context, token string) (*auth.Claims, uuid.UUID, error) {
	claims, err := i.jwt.ParsePasswordChangeToken(auth.StripBearerPrefix(token))
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
		return defaultAccessTokenTTL
	}
	return ttl
}

func (i *authTokenIssuer) refreshTokenTTL() time.Duration {
	ttl := i.config.Auth.JWT.RefreshTokenTTL
	if ttl <= 0 {
		return defaultRefreshTokenTTL
	}
	return ttl
}
