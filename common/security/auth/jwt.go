package auth

import (
	"errors"
	"fmt"

	jwtv5 "github.com/golang-jwt/jwt/v5"
)

var (
	// ErrMissingSecret 表示缺少配置的 secret，JWT 解析无法继续。
	ErrMissingSecret = errors.New("jwt secret is required")
)

// JWTConfig 包含通用 JWT 验签设置。
type JWTConfig struct {
	Secret   string
	Issuer   string
	Audience string
}

// JWTService 负责校验 HMAC JWT，并支持可选 issuer 与 audience 校验。
type JWTService struct {
	secret   []byte
	issuer   string
	audience string
}

// NewJWTService 根据通用 JWT 配置创建 verifier。
func NewJWTService(cfg JWTConfig) *JWTService {
	return &JWTService{secret: []byte(cfg.Secret), issuer: cfg.Issuer, audience: cfg.Audience}
}

// VerifyClaims 校验 token 并将 claims 写入调用方提供的结构。
func (s *JWTService) VerifyClaims(tokenString string, claims jwtv5.Claims) error {
	if len(s.secret) == 0 {
		return ErrMissingSecret
	}
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
		return err
	}
	if token == nil || !token.Valid {
		return jwtv5.ErrTokenInvalidClaims
	}
	return nil
}
