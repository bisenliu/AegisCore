package command

import (
	"github.com/aegiscore/common/runtime/config"
	commonauth "github.com/aegiscore/common/security/auth"
	authapplication "github.com/aegiscore/user-service/internal/features/auth/application"
	"go.uber.org/fx"
)

// UseCaseDepsParams 包含构造 auth command use case 共享依赖所需的 Fx 输入。
type UseCaseDepsParams struct {
	fx.In

	Credentials   authapplication.UserCredentialStore
	TokenVersions authapplication.UserTokenVersionStore
	Sessions      authapplication.AuthSessionStore
	JWT           *commonauth.JWTService
	Config        *config.Config
}

type UseCaseDeps struct {
	credentials          CredentialVerifier
	tokens               AuthTokenIssuer
	sessions             AuthSessionLifecycle
	refreshTokenRotation bool
}

// NewUseCaseDeps 组合凭证、token、会话和轮换依赖，供具体 command use case 使用。
func NewUseCaseDeps(params UseCaseDepsParams) *UseCaseDeps {
	return &UseCaseDeps{
		credentials:          newCredentialVerifier(params.Credentials),
		tokens:               newAuthTokenIssuer(params.JWT, params.Config),
		sessions:             newAuthSessionLifecycle(params.TokenVersions, params.Sessions),
		refreshTokenRotation: params.Config.Auth.RefreshTokenRotation,
	}
}
