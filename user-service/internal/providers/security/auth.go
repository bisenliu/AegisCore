package security

import (
	"fmt"

	"github.com/aegiscore/common/security/auth"
	serviceconfig "github.com/aegiscore/user-service/internal/config"
)

// NewJWTService 根据用户服务认证配置构建共享 JWT verifier。
func NewJWTService(settings serviceconfig.AuthSettings) (*auth.JWTService, error) {
	if err := validateAuthTokenPolicy(settings); err != nil {
		return nil, err
	}
	return auth.NewJWTService(auth.JWTConfig{Secret: settings.JWT.Secret, Issuer: settings.JWT.Issuer, Audience: settings.JWT.Audience}), nil
}

// validateAuthTokenPolicy 检查通行凭证的有效时长配置是否合理（如访问凭证和续期凭证的时长必须大于零）。
func validateAuthTokenPolicy(settings serviceconfig.AuthSettings) error {
	if settings.JWT.RefreshTokenTTL <= settings.JWT.AccessTokenTTL {
		return fmt.Errorf("auth jwt refresh_token_ttl must be greater than access_token_ttl")
	}
	return nil
}
