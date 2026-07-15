package permission

import (
	"context"
	"regexp"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"go.uber.org/fx"
	"go.uber.org/fx/fxtest"

	"github.com/aegiscore/common/runtime/config"
	commonmetrics "github.com/aegiscore/common/runtime/observability/metrics"
	permissionapplication "github.com/aegiscore/user-service/internal/features/permission/application"
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
