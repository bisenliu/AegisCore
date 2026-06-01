package jwt

import (
	"errors"
	"fmt"

	"github.com/aegiscore/common/config"
	jwtv5 "github.com/golang-jwt/jwt/v5"
)

var (
	ErrMissingSecret = errors.New("jwt secret is required")
	ErrMissingUserID = errors.New("jwt user_id is required")
)

type Claims struct {
	UserID string `json:"user_id"`
	jwtv5.RegisteredClaims
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
