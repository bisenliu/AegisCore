package bootstrap

import (
	"github.com/aegiscore/common/auth"
	"github.com/aegiscore/common/config"
)

func NewJWTService(cfg *config.Config) *auth.JWTService {
	return auth.NewJWTService(cfg.Auth)
}
