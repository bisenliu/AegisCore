package permission

import (
	"context"
	"errors"
	"regexp"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/google/uuid"
	rediscmd "github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/require"
	"go.uber.org/fx"
	"go.uber.org/fx/fxtest"
	"go.uber.org/zap"

	"github.com/aegiscore/common/runtime/config"
	"github.com/aegiscore/common/runtime/localcache"
	commonmetrics "github.com/aegiscore/common/runtime/observability/metrics"
	serviceconfig "github.com/aegiscore/user-service/internal/config"
	permissionapplication "github.com/aegiscore/user-service/internal/features/permission/application"
	permissionauthorization "github.com/aegiscore/user-service/internal/features/permission/application/authorization"
	permissionquery "github.com/aegiscore/user-service/internal/features/permission/application/query"
	permissiondomain "github.com/aegiscore/user-service/internal/features/permission/domain"
	permissioncasbin "github.com/aegiscore/user-service/internal/features/permission/infrastructure/casbin"
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

	queries, graph := newPermissionModuleTestApp(t, provider, store, scanner,
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

			queries, _ := newPermissionModuleTestApp(t, provider, store, scanner)
			_, err := queries.GetRouteDiff(context.Background())
			require.NoError(t, err)

			if enabled {
				metricText := gatherPermissionMetricText(t, provider)
				require.Contains(t, metricText, `aegiscore_user_service_permission_route_diff{environment="test",kind="missing",service="aegiscore-user-service-module-test"} 1`)
				require.Contains(t, metricText, `aegiscore_user_service_permission_route_diff{environment="test",kind="stale",service="aegiscore-user-service-module-test"} 0`)
				return
			}

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

	var policyHealth permissionauthorization.PolicyHealth
	var watcherStatus permissionapplication.PolicyWatcherStatus
	var authorizer permissionauthorization.Authorizer
	app := fxtest.New(t,
		fx.Supply(
			provider,
			cfg,
			zap.NewNop(),
			fx.Annotate(redisClient, fx.ResultTags(`name:"cache_redis"`)),
		),
		fx.Replace(
			fx.Annotate(loader, fx.As(new(permissioncasbin.Loader))),
			fx.Annotate(roles, fx.As(new(permissioncasbin.UserRoleResolver))),
			fx.Annotate(permissionModuleUserRoleCacheCloser{}, fx.As(new(permissioncasbin.UserRoleCacheCloser)), fx.ResultTags(`name:"permission_user_role_cache_closer"`)),
			fx.Annotate(permissionModuleUserRoleCacheCloser{}, fx.As(new(localcache.StatsSource)), fx.ResultTags(`name:"permission_rbac_user_roles_cache"`)),
			fx.Annotate(&permissionModuleStore{}, fx.As(new(permissionapplication.PermissionStore))),
			fx.Annotate(permissionModuleScanner{}, fx.As(new(permissionapplication.RouteCatalogScanner))),
		),
		Module,
		fx.Populate(
			&watcherStatus,
			&authorizer,
			&policyHealth,
		),
	)
	app.RequireStart()
	require.True(t, watcherStatus.Running())
	app.RequireStop()
	require.False(t, watcherStatus.Running())

	require.NotNil(t, authorizer)
	require.NotNil(t, policyHealth)
}

func TestPermissionModuleStopsWatcherWhenLaterStartHookFails(t *testing.T) {
	redisServer := miniredis.RunT(t)
	redisClient := rediscmd.NewClient(&rediscmd.Options{Addr: redisServer.Addr()})
	provider := newPermissionModuleMetricsProvider(t, false)
	cfg := &config.Config{App: config.AppConfig{Name: "aegiscore-user-service-module-test"}}
	loader := permissionModulePolicyLoader{}
	roles := permissionModuleUserRoleResolver{}
	startErr := errors.New("later start failed")
	var watcherStatus permissionapplication.PolicyWatcherStatus
	app := fxtest.New(t,
		fx.Supply(
			provider,
			cfg,
			zap.NewNop(),
			fx.Annotate(redisClient, fx.ResultTags(`name:"cache_redis"`)),
		),
		fx.Replace(
			fx.Annotate(loader, fx.As(new(permissioncasbin.Loader))),
			fx.Annotate(roles, fx.As(new(permissioncasbin.UserRoleResolver))),
			fx.Annotate(permissionModuleUserRoleCacheCloser{}, fx.As(new(permissioncasbin.UserRoleCacheCloser)), fx.ResultTags(`name:"permission_user_role_cache_closer"`)),
			fx.Annotate(permissionModuleUserRoleCacheCloser{}, fx.As(new(localcache.StatsSource)), fx.ResultTags(`name:"permission_rbac_user_roles_cache"`)),
			fx.Annotate(&permissionModuleStore{}, fx.As(new(permissionapplication.PermissionStore))),
			fx.Annotate(permissionModuleScanner{}, fx.As(new(permissionapplication.RouteCatalogScanner))),
		),
		Module,
		fx.Populate(&watcherStatus),
		fx.Invoke(func(lifecycle fx.Lifecycle) {
			lifecycle.Append(fx.Hook{OnStart: func(context.Context) error { return startErr }})
		}),
	)
	startCtx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	err := app.Start(startCtx)
	require.ErrorIs(t, err, startErr)
	require.False(t, watcherStatus.Running())
	require.NoError(t, redisClient.Ping(context.Background()).Err())
	require.NoError(t, redisClient.Close())
}

func TestPermissionModuleStartsFailClosedWhenInitialPolicyLoadFails(t *testing.T) {
	redisServer := miniredis.RunT(t)
	redisClient := rediscmd.NewClient(&rediscmd.Options{Addr: redisServer.Addr()})
	t.Cleanup(func() { _ = redisClient.Close() })
	provider := newPermissionModuleMetricsProvider(t, false)
	cfg := &config.Config{App: config.AppConfig{Name: "aegiscore-user-service-module-test"}}
	loadErr := errors.New("initial policy load failed")
	loader := &permissionModuleFailOncePolicyLoader{err: loadErr}
	roles := permissionModuleUserRoleResolver{}
	var authorizer permissionauthorization.Authorizer
	var policyHealth permissionauthorization.PolicyHealth
	var watcherStatus permissionapplication.PolicyWatcherStatus
	app := fxtest.New(t,
		fx.Supply(
			provider,
			cfg,
			zap.NewNop(),
			fx.Annotate(redisClient, fx.ResultTags(`name:"cache_redis"`)),
		),
		fx.Replace(
			fx.Annotate(loader, fx.As(new(permissioncasbin.Loader))),
			fx.Annotate(roles, fx.As(new(permissioncasbin.UserRoleResolver))),
			fx.Annotate(permissionModuleUserRoleCacheCloser{}, fx.As(new(permissioncasbin.UserRoleCacheCloser)), fx.ResultTags(`name:"permission_user_role_cache_closer"`)),
			fx.Annotate(permissionModuleUserRoleCacheCloser{}, fx.As(new(localcache.StatsSource)), fx.ResultTags(`name:"permission_rbac_user_roles_cache"`)),
			fx.Annotate(&permissionModuleStore{}, fx.As(new(permissionapplication.PermissionStore))),
			fx.Annotate(permissionModuleScanner{}, fx.As(new(permissionapplication.RouteCatalogScanner))),
		),
		Module,
		fx.Populate(&authorizer, &policyHealth, &watcherStatus),
	)

	app.RequireStart()
	require.True(t, watcherStatus.Running())
	require.ErrorIs(t, policyHealth.LastError(), loadErr)
	allowed, err := authorizer.Enforce(context.Background(), uuid.NewString(), "/api/v1/users", "GET")
	require.NoError(t, err)
	require.False(t, allowed)
	app.RequireStop()
	require.False(t, watcherStatus.Running())
}

func TestStopRBACLifecycleJoinsWatcherAndCloserErrors(t *testing.T) {
	watcherErr := errors.New("watcher stop failed")
	closeErr := errors.New("cache close failed")
	closer := &permissionModuleErrCloser{err: closeErr}

	stopDone := make(chan error, 1)
	go func() {
		stopDone <- stopRBACLifecycle(context.Background(), func(context.Context) error { return watcherErr }, closer)
	}()
	var err error
	select {
	case err = <-stopDone:
	case <-time.After(time.Second):
		t.Fatal("RBAC lifecycle stop blocked")
	}
	require.ErrorIs(t, err, watcherErr)
	require.ErrorIs(t, err, closeErr)
	require.True(t, closer.closed)
}

func TestUserRoleResolverHolderFailsClosedAndClosesIdempotently(t *testing.T) {
	enabled := true
	size := int64(10)
	ttl := time.Minute
	loadTimeout := time.Second
	holder := &userRoleResolverHolder{params: permissioncasbin.UserRoleResolverParams{Config: &serviceconfig.Config{RBAC: serviceconfig.RBACConfig{UserRoleCache: serviceconfig.FeatureCacheConfig{Enabled: &enabled, Size: &size, TTL: &ttl, LoadTimeout: &loadTimeout}}}}}
	_, err := holder.RolesForUser(context.Background(), uuid.MustParse("018f0000-0000-7000-8000-000000000701"))
	require.ErrorContains(t, err, "not started")
	require.Equal(t, "rbac_user_roles", holder.Name())
	require.Zero(t, holder.Stats().Capacity)
	require.NoError(t, holder.Start(context.Background()))
	require.EqualValues(t, 10, holder.Stats().Capacity)
	require.NoError(t, holder.Close())
	require.NoError(t, holder.Close())
	_, err = holder.RolesForUser(context.Background(), uuid.MustParse("018f0000-0000-7000-8000-000000000701"))
	require.ErrorContains(t, err, "not started")
	require.Zero(t, holder.Stats().Capacity)
}

func TestPermissionModuleRequiresMetricsProvider(t *testing.T) {
	redisServer := miniredis.RunT(t)
	redisClient := rediscmd.NewClient(&rediscmd.Options{Addr: redisServer.Addr()})
	t.Cleanup(func() { _ = redisClient.Close() })
	app := fx.New(
		fxtest.WithTestLogger(t),
		fx.Supply(
			&config.Config{App: config.AppConfig{Name: "aegiscore-user-service-module-test"}},
			zap.NewNop(),
			fx.Annotate(redisClient, fx.ResultTags(`name:"cache_redis"`)),
		),
		fx.Replace(
			fx.Annotate(permissionModulePolicyLoader{}, fx.As(new(permissioncasbin.Loader))),
			fx.Annotate(permissionModuleUserRoleResolver{}, fx.As(new(permissioncasbin.UserRoleResolver))),
			fx.Annotate(permissionModuleUserRoleCacheCloser{}, fx.As(new(permissioncasbin.UserRoleCacheCloser)), fx.ResultTags(`name:"permission_user_role_cache_closer"`)),
			fx.Annotate(permissionModuleUserRoleCacheCloser{}, fx.As(new(localcache.StatsSource)), fx.ResultTags(`name:"permission_rbac_user_roles_cache"`)),
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
) (permissionquery.PermissionQueryService, fx.DotGraph) {
	t.Helper()
	var queries permissionquery.PermissionQueryService
	var graph fx.DotGraph
	appOptions := []fx.Option{
		fx.Supply(provider),
		Module,
		fx.Replace(
			fx.Annotate(store, fx.As(new(permissionapplication.PermissionStore))),
			fx.Annotate(scanner, fx.As(new(permissionapplication.RouteCatalogScanner))),
			fx.Annotate(permissionModuleUserRoleCacheCloser{}, fx.As(new(permissioncasbin.UserRoleCacheCloser)), fx.ResultTags(`name:"permission_user_role_cache_closer"`)),
			fx.Annotate(permissionModuleUserRoleCacheCloser{}, fx.As(new(localcache.StatsSource)), fx.ResultTags(`name:"permission_rbac_user_roles_cache"`)),
			fx.Annotate(noopPermissionPolicyInitializer{}, fx.As(new(permissionPolicyInitializer)), fx.ResultTags(`name:"permission_policy_initializer"`)),
			fx.Annotate(noopPermissionApplicationWatcher{}, fx.As(new(permissionApplicationWatcher))),
		),
		fx.Populate(&queries, &graph),
	}
	appOptions = append(appOptions, options...)
	_ = fxtest.New(t, appOptions...)
	return queries, graph
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

type noopPermissionPolicyInitializer struct{}

func (noopPermissionPolicyInitializer) InitializeFailClosed(context.Context) {}

type noopPermissionApplicationWatcher struct{}

func (noopPermissionApplicationWatcher) Start() {}

func (noopPermissionApplicationWatcher) Stop(context.Context) error { return nil }

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

type permissionModuleFailOncePolicyLoader struct {
	err  error
	done bool
}

func (l *permissionModuleFailOncePolicyLoader) LoadPolicies(context.Context) (permissioncasbin.PolicySet, error) {
	if !l.done {
		l.done = true
		return permissioncasbin.PolicySet{}, l.err
	}
	return permissioncasbin.PolicySet{}, nil
}

type permissionModuleUserRoleResolver struct{}

func (permissionModuleUserRoleResolver) RolesForUser(context.Context, uuid.UUID) ([]uuid.UUID, error) {
	return nil, nil
}

func (permissionModuleUserRoleResolver) InvalidateUserRole(uuid.UUID) {}

func (permissionModuleUserRoleResolver) InvalidateAllUserRoles() {}

type permissionModuleUserRoleCacheCloser struct{}

func (permissionModuleUserRoleCacheCloser) Start(context.Context) error { return nil }

func (permissionModuleUserRoleCacheCloser) Close() error { return nil }

func (permissionModuleUserRoleCacheCloser) Name() string { return "rbac_user_roles" }

func (permissionModuleUserRoleCacheCloser) Stats() localcache.Stats { return localcache.Stats{} }

type permissionModuleErrCloser struct {
	err    error
	closed bool
}

func (c *permissionModuleErrCloser) Close() error {
	c.closed = true
	return c.err
}
