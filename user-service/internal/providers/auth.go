package providers

import (
	"fmt"

	"github.com/aegiscore/common/security/auth"
	"github.com/aegiscore/common/security/password"
	serviceconfig "github.com/aegiscore/user-service/internal/config"
)

// NewJWTService 根据用户服务认证配置构建共享 JWT verifier。
func NewJWTService(cfg *serviceconfig.Config) (*auth.JWTService, error) {
	if err := validateAuthTokenPolicy(cfg.Auth); err != nil {
		return nil, err
	}
	return auth.NewJWTService(auth.JWTConfig{Secret: cfg.Auth.JWT.Secret, Issuer: cfg.Auth.JWT.Issuer, Audience: cfg.Auth.JWT.Audience}), nil
}

// NewPasswordService 根据用户服务认证配置构建密码 KDF 服务实例。
func NewPasswordService(cfg *serviceconfig.Config) (*password.Service, error) {
	return password.NewService(password.Options{
		Concurrency: cfg.Auth.PasswordKDF.Argon2Concurrency,
		QueueSize:   cfg.Auth.PasswordKDF.Argon2QueueSize,
	})
}

func validateAuthTokenPolicy(cfg serviceconfig.AuthConfig) error {
	if cfg.JWT.RefreshTokenTTL <= cfg.JWT.AccessTokenTTL {
		return fmt.Errorf("auth jwt refresh_token_ttl must be greater than access_token_ttl")
	}
	return nil
}
