package bootstrap

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/aegiscore/common/auth"
	"github.com/aegiscore/common/config"
	commoninfra "github.com/aegiscore/common/infrastructure"
	"github.com/aegiscore/common/logger"
	commonmw "github.com/aegiscore/common/middleware"
	commontz "github.com/aegiscore/common/timezone"
	"github.com/aegiscore/common/validation"
	"github.com/aegiscore/user-services/internal/controller"
	"github.com/aegiscore/user-services/internal/repository"
	"github.com/aegiscore/user-services/internal/repository/postgres"
	"github.com/aegiscore/user-services/internal/repository/redis"
	"github.com/aegiscore/user-services/internal/router"
	"github.com/aegiscore/user-services/internal/service"
	"github.com/gin-gonic/gin"
	"go.uber.org/fx"
	"go.uber.org/zap"
)

const defaultShutdownTimeout = 10 * time.Second

func NewApp(configPath string) *fx.App {
	return fx.New(
		fx.Supply(commoninfra.ConfigPath(configPath)),
		fx.Provide(
			commoninfra.NewConfig,
			commoninfra.NewLogger,
		),
		UserServiceModule,
	)
}

var UserServiceModule = fx.Module("aegiscore-user-services",
	commontz.Module,
	validation.Module,
	fx.Provide(
		NewPostgresPools,
		NewRedisClients,
		NewJWTService,
		NewNamedClients,
		postgres.NewUserRepository,
		redis.NewAuthSessionRepository,
		service.NewAuthService,
		service.NewUserService,
		controller.NewAuthController,
		controller.NewUserController,
		NewGinEngine,
		NewHTTPServer,
	),
	fx.Invoke(
		RegisterRoutes,
		// 确保 HTTP 服务器被实例化并将其生命周期 Hook 注册到 Fx 中
		func(*http.Server) {},
	),
)

type GinParams struct {
	fx.In

	Config *config.Config
	Log    *zap.Logger
}

func NewGinEngine(params GinParams) (*gin.Engine, error) {
	gin.SetMode(gin.ReleaseMode)
	engine := gin.New()
	if len(params.Config.HTTP.TrustedProxies) > 0 {
		if err := engine.SetTrustedProxies(params.Config.HTTP.TrustedProxies); err != nil {
			return nil, fmt.Errorf("set trusted proxies: %w", err)
		}
	}
	engine.Use(
		commonmw.TraceID(),
		commonmw.Recovery(params.Log),
		commonmw.RequestLogger(params.Log),
		commonmw.CORS(),
	)
	return engine, nil
}

func NewJWTService(cfg *config.Config) *auth.JWTService {
	return auth.NewJWTService(cfg.Auth)
}

type RegisterRouteParams struct {
	fx.In

	Config         *config.Config
	Log            *zap.Logger
	Engine         *gin.Engine
	JWT            *auth.JWTService
	AuthSessions   repository.AuthSessionRepository `optional:"true"`
	AuthController *controller.AuthController
	UserController *controller.UserController
}

func RegisterRoutes(params RegisterRouteParams) {
	router.RegisterRoutes(params.Engine, router.RouteParams{
		Environment:           params.Config.App.Environment,
		Log:                   params.Log,
		JWT:                   params.JWT,
		AuthConfig:            params.Config.Auth,
		TokenVersionValidator: params.AuthSessions,
		AuthController:        params.AuthController,
		UserController:        params.UserController,
	})
}

type HTTPServerParams struct {
	fx.In

	Lifecycle fx.Lifecycle
	Config    *config.Config
	Log       *zap.Logger
	Engine    *gin.Engine
}

func NewHTTPServer(params HTTPServerParams) *http.Server {
	addr := fmt.Sprintf("%s:%d", params.Config.HTTP.Host, params.Config.HTTP.Port)
	server := &http.Server{
		Addr:         addr,
		Handler:      params.Engine,
		ReadTimeout:  params.Config.HTTP.ReadTimeout,
		WriteTimeout: params.Config.HTTP.WriteTimeout,
		IdleTimeout:  params.Config.HTTP.IdleTimeout,
	}

	params.Lifecycle.Append(fx.Hook{
		OnStart: func(ctx context.Context) error {
			logger.WithContext(params.Log, ctx).Info("starting http server", zap.String("addr", addr))
			go func() {
				if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
					params.Log.Error("http server failed", logger.StackTrace(zap.Error(err))...)
				}
			}()
			return nil
		},
		OnStop: func(ctx context.Context) error {
			shutdownTimeout := params.Config.HTTP.ShutdownTimeout
			if shutdownTimeout == 0 {
				shutdownTimeout = defaultShutdownTimeout
			}
			shutdownCtx, cancel := context.WithTimeout(ctx, shutdownTimeout)
			defer cancel()
			logger.WithContext(params.Log, ctx).Info("stopping http server")
			return server.Shutdown(shutdownCtx)
		},
	})

	return server
}
