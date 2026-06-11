package bootstrap

import (
	"github.com/aegiscore/common/runtime/config"
	"github.com/aegiscore/common/security/auth"
)

// NewJWTService 根据用户服务认证配置构建共享 JWT service。
func NewJWTService(cfg *config.Config) *auth.JWTService {
	return auth.NewJWTService(cfg.Auth)
}
