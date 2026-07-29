package tokens

import (
	"context"
	"errors"
	"fmt"
	"time"

	jwtv5 "github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"go.uber.org/zap"

	runtimeid "github.com/aegiscore/common/runtime/id"
	"github.com/aegiscore/common/runtime/logger"
	commonauth "github.com/aegiscore/common/security/auth"
	serviceconfig "github.com/aegiscore/user-service/internal/config"
	authdomain "github.com/aegiscore/user-service/internal/features/auth/domain"
)

const (
	// SubjectAccess 标识可被受保护 API 中间件接受的访问令牌。
	SubjectAccess = "access"
	// SubjectRefresh 标识刷新令牌，不能作为访问令牌使用。
	SubjectRefresh = "refresh"
	// SubjectPasswordChange 标识只允许强制修改密码的短期令牌。
	SubjectPasswordChange = "password_change"

	// defaultAccessTokenTTL 是配置为非正数时 access token 的兜底生命周期。
	defaultAccessTokenTTL = 15 * time.Minute
	// defaultRefreshTokenTTL 是配置为非正数时 refresh 会话的兜底生命周期。
	defaultRefreshTokenTTL = 7 * 24 * time.Hour
	// defaultPasswordChangeTokenTTL 是配置为非正数时改密 token 的兜底生命周期。
	defaultPasswordChangeTokenTTL = 5 * time.Minute
)

var (
	errMissingUserID       = errors.New("jwt user_id is required")
	errInvalidUserID       = errors.New("jwt user_id is invalid")
	errMissingTokenID      = errors.New("jwt jti is required")
	errInvalidTokenID      = errors.New("jwt jti is invalid")
	errMissingTokenVersion = errors.New("jwt token_version is required")
	errMissingSessionID    = errors.New("jwt session_id is required")
	errInvalidSubject      = errors.New("jwt subject is invalid")
)

// Claims 包含 user-service 认证和会话校验所需的 JWT claims。
type Claims struct {
	UserID       uuid.UUID `json:"user_id"`
	TokenVersion int64     `json:"token_version"`
	SessionID    string    `json:"session_id"`
	jwtv5.RegisteredClaims
}

// Issuer 签发和解析认证流程使用的 JWT。
type Issuer interface {
	IssueTokenPair(ctx context.Context, userID uuid.UUID, tokenVersion int64, sessionID string) (*IssuedTokenPair, error)
	IssuePasswordChangeToken(ctx context.Context, userID uuid.UUID, tokenVersion int64, sessionID string) (*TokenResult, error)
	ParseRefreshToken(ctx context.Context, token string) (*Claims, uuid.UUID, error)
	ParsePasswordChangeToken(ctx context.Context, token string) (*Claims, uuid.UUID, error)
}

// IssuedTokenPair 表示已签发的 token 响应和 refresh 会话生命周期。
type IssuedTokenPair struct {
	Response   *TokenResult
	RefreshTTL time.Duration
}

type authTokenIssuer struct {
	verifier *commonauth.JWTService
	config   serviceconfig.JWTConfig
}

type signInput struct {
	UserID       uuid.UUID
	TokenVersion int64
	SessionID    string
	TTL          time.Duration
}

// NewIssuer 构造 token 签发解析组件。
func NewIssuer(verifier *commonauth.JWTService, settings serviceconfig.AuthSettings) Issuer {
	return &authTokenIssuer{verifier: verifier, config: settings.JWT}
}

// NewAccessTokenVerifier 构造受保护 HTTP 路由使用的 access token verifier。
func NewAccessTokenVerifier(verifier *commonauth.JWTService, settings serviceconfig.AuthSettings) commonauth.AccessTokenVerifier {
	return &authTokenIssuer{verifier: verifier, config: settings.JWT}
}

// IssueTokenPair 为一个认证会话签发 access 和 refresh token。
func (i *authTokenIssuer) IssueTokenPair(ctx context.Context, userID uuid.UUID, tokenVersion int64, sessionID string) (*IssuedTokenPair, error) {
	accessTTL := i.accessTokenTTL()
	refreshTTL := i.refreshTokenTTL()
	access, err := i.sign(signInput{UserID: userID, TokenVersion: tokenVersion, SessionID: sessionID, TTL: accessTTL}, SubjectAccess)
	if err != nil {
		logger.Error(ctx, "sign access token failed", logger.StackTrace(zap.String("user_id", userID.String()), zap.String("session_id", sessionID), zap.Int64("token_version", tokenVersion), zap.Error(err))...)
		return nil, fmt.Errorf("sign access token: %w", err)
	}
	refresh, err := i.sign(signInput{UserID: userID, TokenVersion: tokenVersion, SessionID: sessionID, TTL: refreshTTL}, SubjectRefresh)
	if err != nil {
		logger.Error(ctx, "sign refresh token failed", logger.StackTrace(zap.String("user_id", userID.String()), zap.String("session_id", sessionID), zap.Int64("token_version", tokenVersion), zap.Error(err))...)
		return nil, fmt.Errorf("sign refresh token: %w", err)
	}
	return &IssuedTokenPair{
		Response:   &TokenResult{AccessToken: access, RefreshToken: refresh, TokenType: commonauth.TokenTypeBearer, ExpiresIn: int64(accessTTL.Seconds())},
		RefreshTTL: refreshTTL,
	}, nil
}

// IssuePasswordChangeToken 签发受限 token，并有意不返回 refresh token。
func (i *authTokenIssuer) IssuePasswordChangeToken(ctx context.Context, userID uuid.UUID, tokenVersion int64, sessionID string) (*TokenResult, error) {
	ttl := i.passwordChangeTokenTTL()
	token, err := i.sign(signInput{UserID: userID, TokenVersion: tokenVersion, SessionID: sessionID, TTL: ttl}, SubjectPasswordChange)
	if err != nil {
		logger.Error(ctx, "sign password change token failed", logger.StackTrace(zap.String("user_id", userID.String()), zap.String("session_id", sessionID), zap.Int64("token_version", tokenVersion), zap.Error(err))...)
		return nil, fmt.Errorf("sign password change token: %w", err)
	}
	return &TokenResult{AccessToken: token, TokenType: commonauth.TokenTypeBearer, ExpiresIn: int64(ttl.Seconds())}, nil
}

func (i *authTokenIssuer) passwordChangeTokenTTL() time.Duration {
	ttl := i.config.PasswordChangeTokenTTL
	if ttl <= 0 {
		// 非正数 TTL 配置使用默认值，确保改密 token 始终短期有效。
		return defaultPasswordChangeTokenTTL
	}
	return ttl
}

// ParseRefreshToken 规范化可选 Bearer 输入并校验 refresh token claims。
func (i *authTokenIssuer) ParseRefreshToken(ctx context.Context, token string) (*Claims, uuid.UUID, error) {
	claims, err := i.parse(commonauth.StripBearerPrefix(token), SubjectRefresh, false)
	if err != nil {
		logger.Warn(ctx, "refresh token invalid", zap.Bool("token_present", token != ""))
		return nil, uuid.Nil, authdomain.ErrTokenInvalid
	}
	return claims, claims.UserID, nil
}

// ParsePasswordChangeToken 规范化可选 Bearer 输入，并返回已校验的改密 claims。
func (i *authTokenIssuer) ParsePasswordChangeToken(ctx context.Context, token string) (*Claims, uuid.UUID, error) {
	claims, err := i.parse(commonauth.StripBearerPrefix(token), SubjectPasswordChange, true)
	if err != nil {
		logger.Warn(ctx, "password change token invalid", zap.Bool("token_present", token != ""))
		return nil, uuid.Nil, authdomain.ErrTokenInvalid
	}
	return claims, claims.UserID, nil
}

func (i *authTokenIssuer) VerifyAccessToken(tokenString string) (commonauth.AccessToken, error) {
	claims, err := i.parse(tokenString, SubjectAccess, true)
	if err != nil {
		return commonauth.AccessToken{}, err
	}
	return commonauth.AccessToken{UserID: claims.UserID.String(), SessionID: claims.SessionID, TokenVersion: claims.TokenVersion}, nil
}

func (i *authTokenIssuer) parse(tokenString string, subject string, requireSession bool) (*Claims, error) {
	claims := &Claims{}
	if err := i.verifier.VerifyClaims(tokenString, claims); err != nil {
		return nil, err
	}
	if claims.ID == "" {
		return nil, errMissingTokenID
	}
	if _, err := uuid.Parse(claims.ID); err != nil {
		return nil, errInvalidTokenID
	}
	if claims.UserID == uuid.Nil {
		return nil, errMissingUserID
	}
	if claims.TokenVersion <= 0 {
		return nil, errMissingTokenVersion
	}
	if requireSession && claims.SessionID == "" {
		return nil, errMissingSessionID
	}
	if claims.Subject != subject {
		return nil, errInvalidSubject
	}
	return claims, nil
}

func (i *authTokenIssuer) sign(input signInput, subject string) (string, error) {
	if i.config.Secret == "" {
		return "", commonauth.ErrMissingSecret
	}
	if input.UserID == uuid.Nil {
		return "", errMissingUserID
	}
	if input.TokenVersion <= 0 {
		return "", errMissingTokenVersion
	}
	if input.SessionID == "" {
		return "", errMissingSessionID
	}
	tokenID, err := runtimeid.NewUUID()
	if err != nil {
		return "", fmt.Errorf("generate jwt jti: %w", err)
	}
	claims := Claims{
		UserID:       input.UserID,
		TokenVersion: input.TokenVersion,
		SessionID:    input.SessionID,
		RegisteredClaims: jwtv5.RegisteredClaims{
			ID:        tokenID.String(),
			Issuer:    i.config.Issuer,
			Audience:  audienceClaim(i.config.Audience),
			Subject:   subject,
			ExpiresAt: jwtv5.NewNumericDate(time.Now().Add(input.TTL)),
		},
	}
	return jwtv5.NewWithClaims(jwtv5.SigningMethodHS256, claims).SignedString([]byte(i.config.Secret))
}

func (i *authTokenIssuer) accessTokenTTL() time.Duration {
	ttl := i.config.AccessTokenTTL
	if ttl <= 0 {
		// 非正数 TTL 配置使用默认值，确保签发 token 始终会过期。
		return defaultAccessTokenTTL
	}
	return ttl
}

func (i *authTokenIssuer) refreshTokenTTL() time.Duration {
	ttl := i.config.RefreshTokenTTL
	if ttl <= 0 {
		// 非正数 TTL 配置使用默认值，确保 refresh 会话始终会过期。
		return defaultRefreshTokenTTL
	}
	return ttl
}

func audienceClaim(audience string) jwtv5.ClaimStrings {
	if audience == "" {
		return nil
	}
	return jwtv5.ClaimStrings{audience}
}
