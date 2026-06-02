package jwt

import (
	"errors"
	"fmt"
	"time"

	"github.com/aegiscore/common/config"
	"github.com/aegiscore/common/contextutil"
	jwtv5 "github.com/golang-jwt/jwt/v5"
)

var (
	ErrMissingSecret       = errors.New("jwt secret is required")
	ErrMissingUserID       = errors.New("jwt user_id is required")
	ErrMissingTokenVersion = errors.New("jwt token_version is required")
	ErrMissingSessionID    = errors.New("jwt session_id is required")
)

const (
	TokenTypeBearer = contextutil.TokenTypeBearer
	SubjectAccess   = "access"
	SubjectRefresh  = "refresh"
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

type Service struct {
	secret   []byte
	issuer   string
	audience string
}

func NewService(cfg config.AuthConfig) *Service {
	return &Service{
		secret:   []byte(cfg.JWT.Secret),
		issuer:   cfg.JWT.Issuer,
		audience: cfg.JWT.Audience,
	}
}

func (s *Service) ParseToken(tokenString string) (*Claims, error) {
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
	return claims, nil
}

func (s *Service) ParseRefreshToken(tokenString string) (*Claims, error) {
	return s.parse(tokenString)
}

func (s *Service) SignAccessToken(input SignInput) (string, error) {
	return s.sign(input, SubjectAccess)
}

func (s *Service) SignRefreshToken(input SignInput) (string, error) {
	return s.sign(input, SubjectRefresh)
}

func (s *Service) parse(tokenString string) (*Claims, error) {
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
	return claims, nil
}

func (s *Service) sign(input SignInput, subject string) (string, error) {
	if len(s.secret) == 0 {
		return "", ErrMissingSecret
	}
	if input.UserID == "" {
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
