package bootstrap

import (
	"github.com/aegiscore/common/runtime/config"
	"github.com/aegiscore/common/security/auth"
)

func NewJWTService(cfg *config.Config) *auth.JWTService {
	return auth.NewJWTService(cfg.Auth)
}
