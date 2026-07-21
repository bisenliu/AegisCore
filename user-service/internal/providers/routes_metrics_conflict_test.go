package providers

import (
	"testing"

	"github.com/gin-gonic/gin"
	_ "github.com/mattn/go-sqlite3"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"

	"github.com/aegiscore/common/runtime/config"
	commonauth "github.com/aegiscore/common/security/auth"
	serviceconfig "github.com/aegiscore/user-service/internal/config"
	authtokens "github.com/aegiscore/user-service/internal/features/auth/application/tokens"
	authhttp "github.com/aegiscore/user-service/internal/features/auth/transport/http"
	permissionhttp "github.com/aegiscore/user-service/internal/features/permission/transport/http"
	rolehttp "github.com/aegiscore/user-service/internal/features/role/transport/http"
	userhttp "github.com/aegiscore/user-service/internal/features/user/transport/http"
	"github.com/aegiscore/user-service/internal/router"
)

func TestRegisterRoutesRejectsMetricsPathConflict(t *testing.T) {
	gin.SetMode(gin.TestMode)
	cfg := &config.Config{
		App: config.AppConfig{Name: "configured-user-service", Environment: "local"},
		Observability: config.ObservabilityConfig{
			Metrics: config.MetricsConfig{Enabled: true, Path: "/api/v1/metrics"},
		},
	}
	validator := mustRouteTestValidator(t)
	authRoutes := &routeAuthRoutes{controller: authhttp.NewAuthController(authhttp.AuthControllerOptions{
		Login:          &routeAuthAuthUseCases{},
		Refresh:        &routeAuthAuthUseCases{},
		ChangePassword: &routeAuthAuthUseCases{},
		LogoutCurrent:  &routeAuthAuthUseCases{},
		LogoutAll:      &routeAuthAuthUseCases{},
		Validator:      validator,
	})}
	err := RegisterRoutes(RegisterRouteParams{
		Config:              cfg,
		Log:                 zap.NewNop(),
		Engine:              gin.New(),
		JWT:                 authtokens.NewAccessTokenVerifier(commonauth.NewJWTService(commonauth.JWTConfig{Secret: "secret"}), &serviceconfig.Config{Auth: serviceconfig.AuthConfig{JWT: serviceconfig.JWTConfig{Secret: "secret"}}}),
		TokenVersions:       &routeTokenVersionValidator{version: 1},
		Authorizer:          &routeAuthorizer{allowed: true},
		Metrics:             newRouteTestMetricsProvider(t, cfg),
		PublicRoutes:        []router.PublicRouteRegistrar{authRoutes},
		AuthenticatedRoutes: []router.AuthenticatedRouteRegistrar{authRoutes},
		AuthorizedRoutes: []router.AuthorizedRouteRegistrar{
			&routePermissionRoutes{controller: permissionhttp.NewPermissionController(nil, validator)},
			&routeRoleRoutes{controller: rolehttp.NewRoleController(nil, nil, validator)},
			&routeUserRoutes{controller: userhttp.NewUserController(&routeAuthUserCommands{}, &routeAuthUserQueries{}, validator)},
		},
	})
	require.ErrorContains(t, err, "invalid metrics path")
}
