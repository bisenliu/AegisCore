package bootstrap

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"github.com/aegiscore/common/config"
	commoninfra "github.com/aegiscore/common/infrastructure"
	commonmw "github.com/aegiscore/common/middleware"
	"github.com/aegiscore/user-services/internal/controller"
	"github.com/aegiscore/user-services/internal/entclient"
	"github.com/aegiscore/user-services/internal/repository"
	"github.com/aegiscore/user-services/internal/router"
	"github.com/aegiscore/user-services/internal/service"
	"github.com/gin-gonic/gin"
	"go.uber.org/fx"
)

func NewApp(configPath string) *fx.App {
	return fx.New(
		fx.Supply(commoninfra.ConfigPath(configPath)),
		commoninfra.Module,
		Module,
	)
}

var Module = fx.Module("aegiscore-user-services",
	fx.Provide(
		NewPostgresPools,
		NewRedisClients,
		entclient.NewClients,
		repository.NewUserRepository,
		service.NewUserService,
		controller.NewUserController,
		NewGinEngine,
		NewHTTPServer,
	),
	fx.Invoke(RegisterRoutes),
)

type GinParams struct {
	fx.In

	Config *config.Config
	Log    *slog.Logger
}

func NewGinEngine(params GinParams) (*gin.Engine, error) {
	gin.SetMode(gin.ReleaseMode)
	engine := gin.New()
	if len(params.Config.HTTP.TrustedProxies) > 0 {
		if err := engine.SetTrustedProxies(params.Config.HTTP.TrustedProxies); err != nil {
			return nil, fmt.Errorf("set trusted proxies: %w", err)
		}
	}
	engine.Use(commonmw.RequestID(), commonmw.Recovery(params.Log), commonmw.RequestLogger(params.Log), commonmw.CORS())
	return engine, nil
}

type RegisterRouteParams struct {
	fx.In

	Engine         *gin.Engine
	UserController *controller.UserController
}

func RegisterRoutes(params RegisterRouteParams) {
	router.RegisterRoutes(params.Engine, router.RouteParams{UserController: params.UserController})
}

type HTTPServerParams struct {
	fx.In

	Lifecycle fx.Lifecycle
	Config    *config.Config
	Log       *slog.Logger
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
			params.Log.InfoContext(ctx, "starting http server", slog.String("addr", addr))
			go func() {
				if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
					params.Log.Error("http server failed", slog.Any("error", err))
				}
			}()
			return nil
		},
		OnStop: func(ctx context.Context) error {
			shutdownTimeout := params.Config.HTTP.ShutdownTimeout
			if shutdownTimeout == 0 {
				shutdownTimeout = 10 * time.Second
			}
			shutdownCtx, cancel := context.WithTimeout(ctx, shutdownTimeout)
			defer cancel()
			params.Log.InfoContext(ctx, "stopping http server")
			return server.Shutdown(shutdownCtx)
		},
	})

	return server
}
