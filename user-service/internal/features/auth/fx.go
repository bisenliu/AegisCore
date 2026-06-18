package auth

import (
	"go.uber.org/fx"

	"github.com/aegiscore/common/runtime/config"
	commonauth "github.com/aegiscore/common/security/auth"
	authapplication "github.com/aegiscore/user-service/internal/features/auth/application"
	authcommand "github.com/aegiscore/user-service/internal/features/auth/application/command"
	authcredentials "github.com/aegiscore/user-service/internal/features/auth/application/credentials"
	authsessions "github.com/aegiscore/user-service/internal/features/auth/application/sessions"
	authtokens "github.com/aegiscore/user-service/internal/features/auth/application/tokens"
	authvalidators "github.com/aegiscore/user-service/internal/features/auth/application/validators"
	authpostgres "github.com/aegiscore/user-service/internal/features/auth/infrastructure/postgres"
	authredis "github.com/aegiscore/user-service/internal/features/auth/infrastructure/redis"
	authhttp "github.com/aegiscore/user-service/internal/features/auth/transport/http"
)

type tokenVersionValidatorResult struct {
	fx.Out

	Validator   commonauth.TokenVersionValidator
	Invalidator authvalidators.TokenVersionLocalInvalidator
}

// Module 组装认证功能的应用服务、HTTP 传输层和基础设施适配器。
var Module = fx.Module("feature-auth",
	fx.Provide(
		newAuthMetrics,
		fx.Annotate(
			authpostgres.NewCredentialStore,
			fx.As(new(authapplication.UserCredentialStore)),
		),
		fx.Annotate(
			authpostgres.NewCredentialStore,
			fx.As(new(authapplication.UserTokenVersionStore)),
		),
		fx.Annotate(
			authredis.NewSessionStore,
			fx.As(new(authapplication.AuthSessionStore)),
		),
		fx.Annotate(
			authredis.NewSessionPurgePool,
			fx.As(new(authredis.PurgeTaskPool)),
			fx.ResultTags(`name:"auth_session_purge_pool"`),
		),
		newTokenVersionValidator,
		authcredentials.NewVerifier,
		authtokens.NewIssuer,
		newAuthSessionLifecycle,
		authcommand.NewUseCaseDeps,
		authcommand.NewLoginUseCase,
		authcommand.NewRefreshTokenUseCase,
		authcommand.NewChangePasswordUseCase,
		authcommand.NewLogoutCurrentSessionUseCase,
		authcommand.NewLogoutAllSessionsUseCase,
		authhttp.NewAuthController,
	),
)

func newAuthSessionLifecycle(users authapplication.UserTokenVersionStore, sessions authapplication.AuthSessionStore, tokenVersions authvalidators.TokenVersionLocalInvalidator, cfg *config.Config) authsessions.Lifecycle {
	return authsessions.NewLifecycle(users, sessions, cfg.Auth.MaxActiveSessionsPerUser, tokenVersions)
}

func newTokenVersionValidator(users authapplication.UserTokenVersionStore, sessions authapplication.AuthSessionStore, metrics authapplication.Metrics) tokenVersionValidatorResult {
	validator := authvalidators.NewCachingValidator(users, sessions)
	return tokenVersionValidatorResult{Validator: authvalidators.NewMetricsTokenVersionValidator(validator, metrics), Invalidator: validator}
}
