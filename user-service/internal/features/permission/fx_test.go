package permission

import (
	"context"
	"regexp"
	"testing"

	"github.com/alicebob/miniredis/v2"
	"github.com/google/uuid"
	rediscmd "github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/require"
	"go.uber.org/fx"
	"go.uber.org/fx/fxtest"
	"go.uber.org/zap"

	"github.com/aegiscore/common/runtime/config"
	commonmetrics "github.com/aegiscore/common/runtime/observability/metrics"
	permissionapplication "github.com/aegiscore/user-service/internal/features/permission/application"
	permissionauthorization "github.com/aegiscore/user-service/internal/features/permission/application/authorization"
	permissionquery "github.com/aegiscore/user-service/internal/features/permission/application/query"
	permissiondomain "github.com/aegiscore/user-service/internal/features/permission/domain"
	permissioncasbin "github.com/aegiscore/user-service/internal/features/permission/infrastructure/casbin"
	permissionredis "github.com/aegiscore/user-service/internal/features/permission/infrastructure/redis"
)

func TestPermissionModuleInjectsRouteDiffMetrics(t *testing.T) {
	store := &permissionModuleStore{permissions: []permissiondomain.Permission{
		{PermissionID: uuid.MustParse("018f0000-0000-7000-8000-000000000301"), HTTPMethod: "GET", PathTemplate: "/api/v1/users"},
		{PermissionID: uuid.MustParse("018f0000-0000-7000-8000-000000000302"), HTTPMethod: "GET", PathTemplate: "/api/v1/stale"},
	}}
	scanner := permissionModuleScanner{routes: []permissionapplication.DiscoveredRoute{
		{Method: "GET", Path: "/api/v1/users"},
		{Method: "POST", Path: "/api/v1/users"},
	}}
	metrics := &routeDiffMetricsSpy{Metrics: permissionapplication.NopMetrics()}
	provider := newPermissionModuleMetricsProvider(t, false)

	queries, _, graph := newPermissionModuleTestApp(t, provider, store, scanner,
		fx.Replace(fx.Annotate(metrics, fx.As(new(permissionapplication.Metrics)))),
	)

	_, err := queries.GetRouteDiff(context.Background())
	require.NoError(t, err)
	require.Equal(t, 1, metrics.calls)
	require.Equal(t, 1, metrics.missing)
	require.Equal(t, 1, metrics.stale)

	graphText := string(graph)
	queryConstructor := regexp.MustCompile(`(constructor_\d+) \[shape=plaintext label="NewPermissionQueryService"\];`).FindStringSubmatch(graphText)
	require.Len(t, queryConstructor, 2, graphText)
	require.Contains(t, graphText, queryConstructor[1]+` -> "application.Metrics" [ltail=`)
}

func TestPermissionModuleBuildsWithMetricsConfigurations(t *testing.T) {
	for _, enabled := range []bool{true, false} {
		t.Run(map[bool]string{true: "enabled", false: "disabled"}[enabled], func(t *testing.T) {
			provider := newPermissionModuleMetricsProvider(t, enabled)
			store := &permissionModuleStore{}
			scanner := permissionModuleScanner{routes: []permissionapplication.DiscoveredRoute{
				{Method: "GET", Path: "/api/v1/users"},
			}}

			queries, recorder, _ := newPermissionModuleTestApp(t, provider, store, scanner)
			_, err := queries.GetRouteDiff(context.Background())
			require.NoError(t, err)

			if enabled {
				metricText := gatherPermissionMetricText(t, provider)
				require.IsType(t, &prometheusMetrics{}, recorder)
				require.Contains(t, metricText, `aegiscore_user_service_permission_route_diff{environment="test",kind="missing",service="aegiscore-user-service-module-test"} 1`)
				require.Contains(t, metricText, `aegiscore_user_service_permission_route_diff{environment="test",kind="stale",service="aegiscore-user-service-module-test"} 0`)
				return
			}

			require.IsType(t, permissionapplication.NopMetrics(), recorder)
			require.Nil(t, provider.Gatherer())
		})
	}
}

func TestPermissionModuleProjectsRBACInfrastructureSameInstancesAndStarts(t *testing.T) {
	redisServer := miniredis.RunT(t)
	redisClient := rediscmd.NewClient(&rediscmd.Options{Addr: redisServer.Addr()})
	t.Cleanup(func() { _ = redisClient.Close() })
	provider := newPermissionModuleMetricsProvider(t, false)
	cfg := &config.Config{App: config.AppConfig{Name: "aegiscore-user-service-module-test"}}
	loader := permissionModulePolicyLoader{}
	roles := permissionModuleUserRoleResolver{}

	var engine *permissioncasbin.Engine
	var authorizationEngine permissionauthorization.Engine
	var reloadEngine permissionapplication.PolicyReloadEngine
	var store *permissionredis.Store
	var publisher permissionapplication.PolicyVersionPublisher
	var tracker *permissionredis.VersionTracker
	var trackerPort permissionapplication.PolicyVersionTracker
	var watcher *permissionredis.Watcher
	var watcherStatus permissionredis.WatcherStatus
	var authorizer permissionauthorization.Authorizer
	var reloadMetrics commonmetrics.ReloadMetrics
	app := fxtest.New(t,
		fx.NopLogger,
		fx.Supply(
			provider,
			cfg,
			zap.NewNop(),
			fx.Annotate(redisClient, fx.ResultTags(`name:"cache_redis"`)),
		),
		fx.Replace(
			fx.Annotate(loader, fx.As(new(permissioncasbin.Loader))),
			fx.Annotate(roles, fx.As(new(permissioncasbin.UserRoleResolver))),
			fx.Annotate(permissionModuleUserRoleCacheCloser{}, fx.As(new(permissioncasbin.UserRoleCacheCloser))),
			fx.Annotate(&permissionModuleStore{}, fx.As(new(permissionapplication.PermissionStore))),
			fx.Annotate(permissionModuleScanner{}, fx.As(new(permissionapplication.RouteCatalogScanner))),
		),
		Module,
		fx.Populate(
			&engine,
			&authorizationEngine,
			&reloadEngine,
			&store,
			&publisher,
			&tracker,
			&trackerPort,
			&watcher,
			&watcherStatus,
			&authorizer,
			&reloadMetrics,
		),
	)
	app.RequireStart()
	require.True(t, watcher.Running())
	app.RequireStop()
	require.False(t, watcher.Running())

	require.Same(t, engine, authorizationEngine.(*permissioncasbin.Engine))
	require.Same(t, engine, reloadEngine.(*permissioncasbin.Engine))
	require.Same(t, store, publisher.(*permissionredis.Store))
	require.Same(t, tracker, trackerPort.(*permissionredis.VersionTracker))
	require.Same(t, watcher, watcherStatus.(*permissionredis.Watcher))
	require.NotNil(t, authorizer)
	require.NotNil(t, reloadMetrics)
}

func TestPermissionModuleRequiresMetricsProvider(t *testing.T) {
	redisServer := miniredis.RunT(t)
	redisClient := rediscmd.NewClient(&rediscmd.Options{Addr: redisServer.Addr()})
	t.Cleanup(func() { _ = redisClient.Close() })
	app := fx.New(
		fx.NopLogger,
		fx.Supply(
			&config.Config{App: config.AppConfig{Name: "aegiscore-user-service-module-test"}},
			zap.NewNop(),
			fx.Annotate(redisClient, fx.ResultTags(`name:"cache_redis"`)),
		),
		fx.Replace(
			fx.Annotate(permissionModulePolicyLoader{}, fx.As(new(permissioncasbin.Loader))),
			fx.Annotate(permissionModuleUserRoleResolver{}, fx.As(new(permissioncasbin.UserRoleResolver))),
			fx.Annotate(permissionModuleUserRoleCacheCloser{}, fx.As(new(permissioncasbin.UserRoleCacheCloser))),
			fx.Annotate(&permissionModuleStore{}, fx.As(new(permissionapplication.PermissionStore))),
			fx.Annotate(permissionModuleScanner{}, fx.As(new(permissionapplication.RouteCatalogScanner))),
		),
		Module,
	)

	require.Error(t, app.Err())
	require.Contains(t, app.Err().Error(), "metrics.Provider")
}

func newPermissionModuleTestApp(
	t *testing.T,
	provider *commonmetrics.Provider,
	store permissionapplication.PermissionStore,
	scanner permissionapplication.RouteCatalogScanner,
	options ...fx.Option,
) (permissionquery.PermissionQueryService, permissionapplication.Metrics, fx.DotGraph) {
	t.Helper()
	var queries permissionquery.PermissionQueryService
	var recorder permissionapplication.Metrics
	var graph fx.DotGraph
	appOptions := []fx.Option{
		fx.NopLogger,
		fx.Supply(provider),
		Module,
		fx.Replace(
			fx.Annotate(store, fx.As(new(permissionapplication.PermissionStore))),
			fx.Annotate(scanner, fx.As(new(permissionapplication.RouteCatalogScanner))),
			fx.Annotate(permissionModuleUserRoleCacheCloser{}, fx.As(new(permissioncasbin.UserRoleCacheCloser))),
			&permissioncasbin.Engine{},
			&permissionredis.Watcher{},
		),
		fx.Populate(&queries, &recorder, &graph),
	}
	appOptions = append(appOptions, options...)
	_ = fxtest.New(t, appOptions...)
	return queries, recorder, graph
}

func newPermissionModuleMetricsProvider(t *testing.T, enabled bool) *commonmetrics.Provider {
	t.Helper()
	provider, err := commonmetrics.NewProvider(commonmetrics.Options{
		Config:      config.MetricsConfig{Enabled: enabled},
		ServiceName: "aegiscore-user-service-module-test",
		Environment: "test",
	})
	require.NoError(t, err)
	return provider
}

type permissionModuleStore struct {
	permissionapplication.PermissionStore
	permissions []permissiondomain.Permission
}

func (s *permissionModuleStore) ListAll(context.Context) ([]permissiondomain.Permission, error) {
	return s.permissions, nil
}

type permissionModuleScanner struct {
	routes []permissionapplication.DiscoveredRoute
}

func (s permissionModuleScanner) DiscoverRoutes(context.Context) ([]permissionapplication.DiscoveredRoute, error) {
	return s.routes, nil
}

type routeDiffMetricsSpy struct {
	permissionapplication.Metrics
	calls   int
	missing int
	stale   int
}

func (s *routeDiffMetricsSpy) RouteDiffObserved(_ context.Context, missing int, stale int) {
	s.calls++
	s.missing = missing
	s.stale = stale
}

type permissionModulePolicyLoader struct{}

func (permissionModulePolicyLoader) LoadPolicies(context.Context) (permissioncasbin.PolicySet, error) {
	return permissioncasbin.PolicySet{}, nil
}

type permissionModuleUserRoleResolver struct{}

func (permissionModuleUserRoleResolver) RolesForUser(context.Context, uuid.UUID) ([]uuid.UUID, error) {
	return nil, nil
}

func (permissionModuleUserRoleResolver) InvalidateUserRole(uuid.UUID) {}

func (permissionModuleUserRoleResolver) InvalidateAllUserRoles() {}

type permissionModuleUserRoleCacheCloser struct{}

func (permissionModuleUserRoleCacheCloser) Close() error { return nil }
