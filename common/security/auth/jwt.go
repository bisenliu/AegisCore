package auth

import (
	"errors"
	"fmt"
	"time"

	"github.com/aegiscore/common/runtime/config"
	jwtv5 "github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
)

var (
	// ErrMissingSecret 表示缺少配置的 secret，JWT 签发或解析无法继续。
	ErrMissingSecret = errors.New("jwt secret is required")
	// ErrMissingUserID 表示 JWT 输入或 claim 缺少有效的 user_id UUID。
	ErrMissingUserID = errors.New("jwt user_id is required")
	// ErrMissingTokenVersion 表示 JWT 输入或 claim 缺少正数 token version。
	ErrMissingTokenVersion = errors.New("jwt token_version is required")
	// ErrMissingSessionID 表示 JWT 输入或 claim 缺少会话标识。
	ErrMissingSessionID = errors.New("jwt session_id is required")
	// ErrInvalidSubject 表示 JWT subject 不适用于当前 token 流程。
	ErrInvalidSubject = errors.New("jwt subject is invalid")
)

const (
	// SubjectAccess 标识可被受保护 API 中间件接受的访问令牌。
	SubjectAccess = "access"
	// SubjectRefresh 标识刷新令牌，不能作为访问令牌使用。
	SubjectRefresh = "refresh"
	// SubjectPasswordChange 标识只允许强制修改密码的短期令牌。
	SubjectPasswordChange = "password_change"
)

// Claims 包含认证和会话校验所需的 AegisCore JWT claims。
type Claims struct {
	UserID       string `json:"user_id"`
	TokenVersion int64  `json:"token_version"`
	SessionID    string `json:"session_id"`
	jwtv5.RegisteredClaims
}

// SignInput 包含签发 JWT 所需的身份、会话、token version 和 TTL。
type SignInput struct {
	UserID       string
	TokenVersion int64
	SessionID    string
	TTL          time.Duration
}

// JWTService 负责签发和解析 HMAC JWT，并支持可选 issuer 与 audience 校验。
type JWTService struct {
	secret   []byte
	issuer   string
	audience string
}

// NewJWTService 根据认证配置创建 JWTService。
func NewJWTService(cfg config.AuthConfig) *JWTService {
	return &JWTService{
		secret:   []byte(cfg.JWT.Secret),
		issuer:   cfg.JWT.Issuer,
		audience: cfg.JWT.Audience,
	}
}

// ParseToken 校验 access token，并返回受保护 API 中间件所需 claims。
func (s *JWTService) ParseToken(tokenString string) (*Claims, error) {
	claims, err := s.parse(tokenString)
	if err != nil {
		return nil, err
	}
	if claims.TokenVersion <= 0 {
		return nil, ErrMissingTokenVersion
	}
	if claims.SessionID == "" {
		return nil, ErrMissingSessionID
	}
	if claims.Subject != SubjectAccess {
		return nil, ErrInvalidSubject
	}
	return claims, nil
}

// ParseRefreshToken 校验 refresh token，并要求 subject 为 refresh。
func (s *JWTService) ParseRefreshToken(tokenString string) (*Claims, error) {
	claims, err := s.parse(tokenString)
	if err != nil {
		return nil, err
	}
	if claims.Subject != SubjectRefresh {
		return nil, ErrInvalidSubject
	}
	return claims, nil
}

// ParsePasswordChangeToken 校验受限改密 token 及必要会话 claims。
func (s *JWTService) ParsePasswordChangeToken(tokenString string) (*Claims, error) {
	claims, err := s.parse(tokenString)
	if err != nil {
		return nil, err
	}
	if claims.TokenVersion <= 0 {
		return nil, ErrMissingTokenVersion
	}
	if claims.SessionID == "" {
		return nil, ErrMissingSessionID
	}
	if claims.Subject != SubjectPasswordChange {
		return nil, ErrInvalidSubject
	}
	return claims, nil
}

// SignAccessToken 为受保护 API 请求签发 access token。
func (s *JWTService) SignAccessToken(input SignInput) (string, error) {
	return s.sign(input, SubjectAccess)
}

// SignRefreshToken 为会话轮换和续期签发 refresh token。
func (s *JWTService) SignRefreshToken(input SignInput) (string, error) {
	return s.sign(input, SubjectRefresh)
}

// SignPasswordChangeToken 为强制改密流程签发受限 token。
func (s *JWTService) SignPasswordChangeToken(input SignInput) (string, error) {
	return s.sign(input, SubjectPasswordChange)
}

func (s *JWTService) parse(tokenString string) (*Claims, error) {
	if len(s.secret) == 0 {
		return nil, ErrMissingSecret
	}
	claims := &Claims{}
	options := []jwtv5.ParserOption{jwtv5.WithExpirationRequired()}
	// issuer 和 audience 只在部署配置显式提供时才作为兼容性校验。
	if s.issuer != "" {
		options = append(options, jwtv5.WithIssuer(s.issuer))
	}
	if s.audience != "" {
		options = append(options, jwtv5.WithAudience(s.audience))
	}

	token, err := jwtv5.ParseWithClaims(tokenString, claims, func(token *jwtv5.Token) (any, error) {
		if _, ok := token.Method.(*jwtv5.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %s", token.Method.Alg())
		}
		return s.secret, nil
	}, options...)
	if err != nil {
		return nil, err
	}
	if token == nil || !token.Valid {
		return nil, jwtv5.ErrTokenInvalidClaims
	}
	if claims.UserID == "" {
		return nil, ErrMissingUserID
	}
	if _, err := uuid.Parse(claims.UserID); err != nil {
		// token 中的用户 ID 是外部 UUID，不是数据库 ID，因此提前拒绝非 UUID subject。
		return nil, ErrMissingUserID
	}
	return claims, nil
}

func (s *JWTService) sign(input SignInput, subject string) (string, error) {
	if len(s.secret) == 0 {
		return "", ErrMissingSecret
	}
	if input.UserID == "" {
		return "", ErrMissingUserID
	}
	if _, err := uuid.Parse(input.UserID); err != nil {
		return "", ErrMissingUserID
	}
	if input.TokenVersion <= 0 {
		return "", ErrMissingTokenVersion
	}
	if input.SessionID == "" {
		return "", ErrMissingSessionID
	}
	expiresAt := time.Now().Add(input.TTL)
	claims := Claims{
		UserID:       input.UserID,
		TokenVersion: input.TokenVersion,
		SessionID:    input.SessionID,
		RegisteredClaims: jwtv5.RegisteredClaims{
			Issuer:    s.issuer,
			Audience:  audienceClaim(s.audience),
			Subject:   subject,
			ExpiresAt: jwtv5.NewNumericDate(expiresAt),
		},
	}
	return jwtv5.NewWithClaims(jwtv5.SigningMethodHS256, claims).SignedString(s.secret)
}

func audienceClaim(audience string) jwtv5.ClaimStrings {
	if audience == "" {
		// 省略 aud 可保持 token 与未配置 audience 校验的部署兼容。
		return nil
	}
	return jwtv5.ClaimStrings{audience}
}
