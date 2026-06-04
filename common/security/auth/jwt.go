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
	ErrMissingSecret       = errors.New("jwt secret is required")
	ErrMissingUserID       = errors.New("jwt user_id is required")
	ErrMissingTokenVersion = errors.New("jwt token_version is required")
	ErrMissingSessionID    = errors.New("jwt session_id is required")
	ErrInvalidSubject      = errors.New("jwt subject is invalid")
)

const (
	// SubjectAccess 标识可被受保护 API 中间件接受的访问令牌。
	SubjectAccess = "access"
	// SubjectRefresh 标识刷新令牌，不能作为访问令牌使用。
	SubjectRefresh = "refresh"
	// SubjectPasswordChange 标识只允许强制修改密码的短期令牌。
	SubjectPasswordChange = "password_change"
)

type Claims struct {
	UserID       string `json:"user_id"`
	TokenVersion int64  `json:"token_version"`
	SessionID    string `json:"session_id"`
	jwtv5.RegisteredClaims
}

type SignInput struct {
	UserID       string
	TokenVersion int64
	SessionID    string
	TTL          time.Duration
}

type JWTService struct {
	secret   []byte
	issuer   string
	audience string
}

func NewJWTService(cfg config.AuthConfig) *JWTService {
	return &JWTService{
		secret:   []byte(cfg.JWT.Secret),
		issuer:   cfg.JWT.Issuer,
		audience: cfg.JWT.Audience,
	}
}

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

func (s *JWTService) SignAccessToken(input SignInput) (string, error) {
	return s.sign(input, SubjectAccess)
}

func (s *JWTService) SignRefreshToken(input SignInput) (string, error) {
	return s.sign(input, SubjectRefresh)
}

func (s *JWTService) SignPasswordChangeToken(input SignInput) (string, error) {
	return s.sign(input, SubjectPasswordChange)
}

func (s *JWTService) parse(tokenString string) (*Claims, error) {
	if len(s.secret) == 0 {
		return nil, ErrMissingSecret
	}
	claims := &Claims{}
	options := []jwtv5.ParserOption{jwtv5.WithExpirationRequired()}
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
		return nil
	}
	return jwtv5.ClaimStrings{audience}
}
