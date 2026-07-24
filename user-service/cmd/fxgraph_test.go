package main

import (
	"database/sql"
	"net/http"
	"path/filepath"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/require"
	"go.uber.org/fx"

	runtimefxgraph "github.com/aegiscore/common/runtime/fxgraph"
	"github.com/aegiscore/common/runtime/localcache"
	commonlogger "github.com/aegiscore/common/runtime/logger"
	commonmetrics "github.com/aegiscore/common/runtime/observability/metrics"
	commontracing "github.com/aegiscore/common/runtime/observability/tracing"

	"github.com/aegiscore/user-service/internal/bootstrap"
	serviceconfig "github.com/aegiscore/user-service/internal/config"
	authredis "github.com/aegiscore/user-service/internal/features/auth/infrastructure/redis"
	permissionapplication "github.com/aegiscore/user-service/internal/features/permission/application"
	"github.com/aegiscore/user-service/internal/router"
)

func TestFxGraphCommandWritesGraph(t *testing.T) {
	called := 0
	deps := testRootCommandDependencies(t)
	deps.fxGraphWriter = func(path string, opts ...fx.Option) (string, error) {
		called++
		require.Equal(t, "docs/test.dot", path)
		dot, err := runtimefxgraph.RenderDOT(opts...)
		require.NoError(t, err)
		assertFxGraphContainsAppNodes(t, dot)
		return dot, nil
	}

	root := newRootCommand(deps)
	root.SetArgs([]string{"fxgraph", "--config", filepath.Join("..", "configs", "config.yaml"), "--output", "docs/test.dot"})
	require.NoError(t, root.Execute())
	require.Equal(t, 1, called)
}

func TestFxGraphOptionsRenderDOT(t *testing.T) {
	cfg, err := serviceconfig.NewConfig(serviceconfig.ConfigPath(filepath.Join("..", "configs", "config.yaml")))
	require.NoError(t, err)

	dot, err := runtimefxgraph.RenderDOT(fxGraphOptions(cfg)...)
	require.NoError(t, err)
	assertFxGraphContainsAppNodes(t, dot)
}

func TestFxGraphOptionsUseWiringModuleOnly(t *testing.T) {
	cfg, err := serviceconfig.NewConfig(serviceconfig.ConfigPath(filepath.Join("..", "configs", "config.yaml")))
	require.NoError(t, err)

	dot, err := runtimefxgraph.RenderDOT(fxGraphOptions(cfg)...)
	require.NoError(t, err)
	for _, runtimeOnly := range []string{
		"timezone.Init",
		"RegisterRuntimeDependencyMetrics",
		"RegisterRoutes",
		"registerRBACLifecycle",
		"func(*http.Server)",
		"func(*bootstrap.PprofServer)",
	} {
		require.NotContains(t, dot, runtimeOnly)
	}
}

func TestFxGraphOptionsDoNotConstructRuntimeResources(t *testing.T) {
	cfg, err := serviceconfig.NewConfig(serviceconfig.ConfigPath(filepath.Join("..", "configs", "config.yaml")))
	require.NoError(t, err)

	_, err = runtimefxgraph.RenderDOT(append(fxGraphOptions(cfg), fxGraphSideEffectGuards(t)...)...)
	require.NoError(t, err)
}

func TestFxGraphOptionsDoNotMutateProcessState(t *testing.T) {
	cfg, err := serviceconfig.NewConfig(serviceconfig.ConfigPath(filepath.Join("..", "configs", "config.yaml")))
	require.NoError(t, err)

	previousLocal := time.Local
	previousGinMode := gin.Mode()
	t.Cleanup(func() {
		time.Local = previousLocal
		gin.SetMode(previousGinMode)
	})
	time.Local = time.UTC
	gin.SetMode(gin.DebugMode)

	_, err = runtimefxgraph.RenderDOT(fxGraphOptions(cfg)...)
	require.NoError(t, err)
	require.Same(t, time.UTC, time.Local)
	require.Equal(t, gin.DebugMode, gin.Mode())
}

func TestFxGraphRenderDOTFailsWithoutServiceConfig(t *testing.T) {
	cfg, err := serviceconfig.NewConfig(serviceconfig.ConfigPath(filepath.Join("..", "configs", "config.yaml")))
	require.NoError(t, err)

	_, err = runtimefxgraph.RenderDOT(
		// Fx 分类：开发工具 - 只提供共享 runtime config，模拟缺失 user-service 私有配置的错误图。
		fx.Supply(serviceconfig.NewRuntimeConfig(cfg)),
		fx.Provide(
			// Fx 分类：基础运行时 - 与正式 AppOptions 相同的日志 provider。
			commonlogger.NewLogger,
		),
		// Fx 分类：开发工具 - 复用正式 composition root 校验缺失输入会失败。
		bootstrap.AppModule,
	)
	require.Error(t, err)
	require.ErrorContains(t, err, "missing type: *config.Config")
}

func assertFxGraphContainsAppNodes(t *testing.T, dot string) {
	t.Helper()
	require.NotEmpty(t, dot)
	for _, expected := range []string{
		"github.com/aegiscore/user-service/internal/bootstrap",
		"github.com/aegiscore/user-service/internal/features/auth",
		"github.com/aegiscore/user-service/internal/features/permission",
		"github.com/aegiscore/user-service/internal/features/role",
		"github.com/aegiscore/user-service/internal/features/user",
		"*config.Config",
		"*gin.Engine",
	} {
		require.Contains(t, dot, expected)
	}
	require.Contains(t, dot, "constructor_")
	require.Contains(t, dot, " -> ")
	require.NotContains(t, dot, "config.ConfigPath")
}

func fxGraphSideEffectGuards(t *testing.T) []fx.Option {
	t.Helper()
	fail := func(name string) {
		t.Helper()
		t.Fatalf("fxgraph constructed runtime resource: %s", name)
	}
	return []fx.Option{
		fx.Decorate(func(provider *commonmetrics.Provider) *commonmetrics.Provider {
			fail("metrics provider")
			return provider
		}),
		fx.Decorate(func(provider *commontracing.Provider) *commontracing.Provider {
			fail("tracing provider")
			return provider
		}),
		fx.Decorate(fx.Annotate(func(db *sql.DB) *sql.DB {
			fail("primary postgres")
			return db
		}, fx.ParamTags(`name:"primary_db"`), fx.ResultTags(`name:"primary_db"`))),
		fx.Decorate(fx.Annotate(func(client *redis.Client) *redis.Client {
			fail("cache redis")
			return client
		}, fx.ParamTags(`name:"cache_redis"`), fx.ResultTags(`name:"cache_redis"`))),
		fx.Decorate(func(engine *gin.Engine) *gin.Engine {
			fail("gin engine")
			return engine
		}),
		fx.Decorate(func(server *http.Server) *http.Server {
			fail("http server")
			return server
		}),
		fx.Decorate(func(server *bootstrap.PprofServer) *bootstrap.PprofServer {
			fail("pprof server")
			return server
		}),
		fx.Decorate(func(checks router.HealthChecks) router.HealthChecks {
			fail("health checks")
			return checks
		}),
		fx.Decorate(fx.Annotate(func(pool authredis.PurgeTaskPool) authredis.PurgeTaskPool {
			fail("auth session purge pool")
			return pool
		}, fx.ParamTags(`name:"auth_session_purge_pool"`), fx.ResultTags(`name:"auth_session_purge_pool"`))),
		fx.Decorate(fx.Annotate(func(source localcache.StatsSource) localcache.StatsSource {
			fail("auth token version cache")
			return source
		}, fx.ParamTags(`name:"auth_token_version_cache"`), fx.ResultTags(`name:"auth_token_version_cache"`))),
		fx.Decorate(fx.Annotate(func(source localcache.StatsSource) localcache.StatsSource {
			fail("rbac user roles cache")
			return source
		}, fx.ParamTags(`name:"rbac_user_roles_cache"`), fx.ResultTags(`name:"rbac_user_roles_cache"`))),
		fx.Decorate(func(status permissionapplication.PolicyWatcherStatus) permissionapplication.PolicyWatcherStatus {
			fail("rbac policy watcher")
			return status
		}),
	}
}
