package providers

import (
	"fmt"

	"github.com/aegiscore/common/runtime/config"
	"github.com/aegiscore/common/security/auth"
	"github.com/aegiscore/common/security/password"
)

// NewJWTService 根据用户服务认证配置构建共享 JWT service。
func NewJWTService(cfg *config.Config) (*auth.JWTService, error) {
	if err := validateAuthTokenPolicy(cfg.Auth); err != nil {
		return nil, err
	}
	return auth.NewJWTService(cfg.Auth), nil
}

// NewPasswordService 根据用户服务认证配置构建密码 KDF 服务实例。
func NewPasswordService(cfg *config.Config) (*password.Service, error) {
	return password.NewService(password.Options{
		Concurrency: cfg.Auth.PasswordKDF.Argon2Concurrency,
		QueueSize:   cfg.Auth.PasswordKDF.Argon2QueueSize,
	})
}

func validateAuthTokenPolicy(cfg config.AuthConfig) error {
	if cfg.JWT.RefreshTokenTTL <= cfg.JWT.AccessTokenTTL {
		return fmt.Errorf("auth jwt refresh_token_ttl must be greater than access_token_ttl")
	}
	return nil
}
