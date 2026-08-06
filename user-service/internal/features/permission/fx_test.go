package permission

import (
	"context"
	"errors"
	"sync/atomic"
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
	permissioncasbin "github.com/aegiscore/user-service/internal/features/permission/infrastructure/casbin"
)

func TestPermissionModuleProjectsRBACInfrastructureSameInstancesAndStarts(t *testing.T) {
	redisServer := miniredis.RunT(t)
	redisClient := rediscmd.NewClient(&rediscmd.Options{Addr: redisServer.Addr()})
	t.Cleanup(func() { _ = redisClient.Close() })
	provider := newPermissionModuleMetricsProvider(t, false)
	settings := serviceconfig.RBACSettings{AppName: "aegiscore-user-service-module-test", PolicyWatcher: serviceconfig.DefaultPolicyWatcherConfig()}
	settings.OutboxDispatcher = serviceconfig.DefaultOutboxDispatcherConfig()
	loader := permissionModulePolicyLoader{}
	roles := permissionModuleUserRoleResolver{}

	var policyHealth permissionauthorization.PolicyHealth
	var watcherStatus permissionapplication.PolicyWatcherStatus
	var authorizer permissionauthorization.Authorizer
	var notifier permissionapplication.PolicyChangeNotifier
	var runtime *PermissionRuntime
	app := fxtest.New(t,
		fx.Supply(
			provider,
			settings,
			zap.NewNop(),
			fx.Annotate(redisClient, fx.As(new(rediscmd.UniversalClient)), fx.ResultTags(`name:"cache_redis"`)),
		),
		fx.Replace(
			fx.Annotate(loader, fx.As(new(permissioncasbin.Loader))),
			fx.Annotate(roles, fx.As(new(permissioncasbin.UserRoleResolver))),
			fx.Annotate(permissionModuleUserRoleCacheStats{}, fx.As(new(localcache.StatsSource)), fx.ResultTags(`name:"permission_rbac_user_roles_cache"`)),
			fx.Annotate(&permissionModuleStore{}, fx.As(new(permissionapplication.PermissionStore))),
			fx.Annotate(&permissionModuleOutboxStore{}, fx.As(new(permissionapplication.OutboxStore))),
			fx.Annotate(permissionModuleRevisionSource{}, fx.As(new(permissionapplication.LatestPolicyRevisionSource))),
		),
		Module,
		fx.Populate(
			&watcherStatus,
			&authorizer,
			&policyHealth,
			&notifier,
			&runtime,
		),
	)
	require.NotNil(t, runtime)
	app.RequireStart()
	require.True(t, watcherStatus.Status().Running)
	require.True(t, runtime.WatcherStatus.Status().Running)
	dispatcherStatus, err := runtime.DispatcherStatus.Status(context.Background())
	require.NoError(t, err)
	require.True(t, dispatcherStatus.Running)
	require.Same(t, runtime.Dispatcher, runtime.DispatcherStatus)
	app.RequireStop()
	require.False(t, watcherStatus.Status().Running)
	require.False(t, runtime.WatcherStatus.Status().Running)
	dispatcherStatus, err = runtime.DispatcherStatus.Status(context.Background())
	require.NoError(t, err)
	require.False(t, dispatcherStatus.Running)

	require.NotNil(t, authorizer)
	require.NotNil(t, policyHealth)
	require.NotNil(t, notifier)
	require.NotNil(t, runtime.Authorizer)
	require.NotNil(t, runtime.PolicyHealth)
	require.NotNil(t, runtime.WatcherStatus)
	require.NotNil(t, runtime.Watcher)
	require.NotNil(t, runtime.Dispatcher)
	require.NotNil(t, runtime.DispatcherStatus)
	require.NotNil(t, runtime.Notifier)
	require.NotNil(t, runtime.Initializer)
}

func TestPermissionModuleStopsWatcherWhenLaterStartHookFails(t *testing.T) {
	redisServer := miniredis.RunT(t)
	redisClient := rediscmd.NewClient(&rediscmd.Options{Addr: redisServer.Addr()})
	provider := newPermissionModuleMetricsProvider(t, false)
	settings := serviceconfig.RBACSettings{AppName: "aegiscore-user-service-module-test", PolicyWatcher: serviceconfig.DefaultPolicyWatcherConfig()}
	settings.OutboxDispatcher = serviceconfig.DefaultOutboxDispatcherConfig()
	loader := permissionModulePolicyLoader{}
	roles := permissionModuleUserRoleResolver{}
	startErr := errors.New("later start failed")
	var watcherStatus permissionapplication.PolicyWatcherStatus
	app := fxtest.New(t,
		fx.Supply(
			provider,
			settings,
			zap.NewNop(),
			fx.Annotate(redisClient, fx.As(new(rediscmd.UniversalClient)), fx.ResultTags(`name:"cache_redis"`)),
		),
		fx.Replace(
			fx.Annotate(loader, fx.As(new(permissioncasbin.Loader))),
			fx.Annotate(roles, fx.As(new(permissioncasbin.UserRoleResolver))),
			fx.Annotate(permissionModuleUserRoleCacheStats{}, fx.As(new(localcache.StatsSource)), fx.ResultTags(`name:"permission_rbac_user_roles_cache"`)),
			fx.Annotate(&permissionModuleStore{}, fx.As(new(permissionapplication.PermissionStore))),
			fx.Annotate(&permissionModuleOutboxStore{}, fx.As(new(permissionapplication.OutboxStore))),
			fx.Annotate(permissionModuleRevisionSource{}, fx.As(new(permissionapplication.LatestPolicyRevisionSource))),
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
	require.False(t, watcherStatus.Status().Running)
	require.NoError(t, redisClient.Ping(context.Background()).Err())
	require.NoError(t, redisClient.Close())
}

func TestPermissionModuleStartsFailClosedWhenInitialPolicyLoadFails(t *testing.T) {
	redisServer := miniredis.RunT(t)
	redisClient := rediscmd.NewClient(&rediscmd.Options{Addr: redisServer.Addr()})
	t.Cleanup(func() { _ = redisClient.Close() })
	provider := newPermissionModuleMetricsProvider(t, false)
	settings := serviceconfig.RBACSettings{AppName: "aegiscore-user-service-module-test", PolicyWatcher: serviceconfig.DefaultPolicyWatcherConfig()}
	settings.OutboxDispatcher = serviceconfig.DefaultOutboxDispatcherConfig()
	loadErr := errors.New("initial policy load failed")
	allowRecovery := make(chan struct{})
	loader := &permissionModuleFailOncePolicyLoader{err: loadErr, allowRecovery: allowRecovery}
	roles := permissionModuleUserRoleResolver{}
	var authorizer permissionauthorization.Authorizer
	var policyHealth permissionauthorization.PolicyHealth
	var watcherStatus permissionapplication.PolicyWatcherStatus
	app := fxtest.New(t,
		fx.Supply(
			provider,
			settings,
			zap.NewNop(),
			fx.Annotate(redisClient, fx.As(new(rediscmd.UniversalClient)), fx.ResultTags(`name:"cache_redis"`)),
		),
		fx.Replace(
			fx.Annotate(loader, fx.As(new(permissioncasbin.Loader))),
			fx.Annotate(roles, fx.As(new(permissioncasbin.UserRoleResolver))),
			fx.Annotate(permissionModuleUserRoleCacheStats{}, fx.As(new(localcache.StatsSource)), fx.ResultTags(`name:"permission_rbac_user_roles_cache"`)),
			fx.Annotate(&permissionModuleStore{}, fx.As(new(permissionapplication.PermissionStore))),
			fx.Annotate(&permissionModuleOutboxStore{}, fx.As(new(permissionapplication.OutboxStore))),
			fx.Annotate(permissionModuleRevisionSource{}, fx.As(new(permissionapplication.LatestPolicyRevisionSource))),
		),
		Module,
		fx.Populate(&authorizer, &policyHealth, &watcherStatus),
	)

	app.RequireStart()
	require.True(t, watcherStatus.Status().Running)
	require.ErrorIs(t, policyHealth.ProjectionStatus().LastError, loadErr)
	allowed, err := authorizer.Enforce(context.Background(), uuid.NewString(), "/api/v1/users", "GET")
	require.NoError(t, err)
	require.False(t, allowed)
	close(allowRecovery)
	require.Eventually(t, func() bool {
		return policyHealth.ProjectionStatus().Ready()
	}, time.Second, time.Millisecond)
	app.RequireStop()
	require.False(t, watcherStatus.Status().Running)
}

func TestStopRBACLifecycleJoinsDispatcherAndWatcherErrors(t *testing.T) {
	dispatcherErr := errors.New("dispatcher stop failed")
	watcherErr := errors.New("watcher stop failed")

	stopDone := make(chan error, 1)
	go func() {
		stopDone <- stopRBACLifecycle(context.Background(), func(context.Context) error { return dispatcherErr }, func(context.Context) error { return watcherErr })
	}()
	var err error
	select {
	case err = <-stopDone:
	case <-time.After(time.Second):
		t.Fatal("RBAC lifecycle stop blocked")
	}
	require.ErrorIs(t, err, watcherErr)
	require.ErrorIs(t, err, dispatcherErr)
}

func TestRegisterRBACLifecycleStopsDispatcherAndWatcherAfterWatcherError(t *testing.T) {
	watcherErr := errors.New("watcher stop failed")
	lifecycle := &permissionModuleLifecycle{}
	watcher := &permissionModuleApplicationWatcher{stopErr: watcherErr}
	dispatcher := &permissionModuleDispatcher{stopErr: errors.New("dispatcher stop failed")}

	registerRBACLifecycle(RegisterRBACLifecycleParams{
		Lifecycle: lifecycle,
		Runtime:   &PermissionRuntime{Initializer: &permissionModulePolicyInitializer{}, Watcher: watcher, Dispatcher: dispatcher},
	})
	require.Len(t, lifecycle.hooks, 1)

	err := lifecycle.hooks[0].OnStop(context.Background())
	require.ErrorIs(t, err, dispatcher.stopErr)
	require.ErrorIs(t, err, watcherErr)
	require.Equal(t, 1, watcher.stopCalls)
	require.Equal(t, 1, dispatcher.stopCalls)
}

func TestRegisterRBACLifecycleOrdersStartAndStop(t *testing.T) {
	var order []string
	lifecycle := &permissionModuleLifecycle{}
	initializer := &permissionModulePolicyInitializer{order: &order}
	watcher := &permissionModuleApplicationWatcher{order: &order}
	dispatcher := &permissionModuleDispatcher{order: &order}
	registerRBACLifecycle(RegisterRBACLifecycleParams{
		Lifecycle: lifecycle,
		Runtime:   &PermissionRuntime{Initializer: initializer, Watcher: watcher, Dispatcher: dispatcher},
	})

	require.NoError(t, lifecycle.hooks[0].OnStart(context.Background()))
	require.Equal(t, []string{"initializer.initialize", "watcher.start", "dispatcher.start"}, order)
	order = nil
	require.NoError(t, lifecycle.hooks[0].OnStop(context.Background()))
	require.Equal(t, []string{"dispatcher.stop", "watcher.stop"}, order)
}

func TestRegisterRBACLifecycleRollsBackWhenWatcherStartFails(t *testing.T) {
	startErr := errors.New("watcher start failed")
	stopErr := errors.New("watcher rollback failed")
	var order []string
	lifecycle := &permissionModuleLifecycle{}
	watcher := &permissionModuleApplicationWatcher{startErr: startErr, stopErr: stopErr, order: &order}
	dispatcher := &permissionModuleDispatcher{order: &order}
	registerRBACLifecycle(RegisterRBACLifecycleParams{
		Lifecycle: lifecycle,
		Runtime: &PermissionRuntime{
			Initializer: &permissionModulePolicyInitializer{order: &order},
			Watcher:     watcher,
			Dispatcher:  dispatcher,
		},
	})

	err := lifecycle.hooks[0].OnStart(context.Background())
	require.ErrorIs(t, err, startErr)
	require.ErrorIs(t, err, stopErr)
	require.Equal(t, []string{"initializer.initialize", "watcher.start", "watcher.stop"}, order)
	require.False(t, dispatcher.started)
}

func TestRegisterRBACLifecycleRollsBackWhenDispatcherStartFails(t *testing.T) {
	startErr := errors.New("dispatcher start failed")
	watcherErr := errors.New("watcher rollback failed")
	var order []string
	lifecycle := &permissionModuleLifecycle{}
	watcher := &permissionModuleApplicationWatcher{stopErr: watcherErr, order: &order}
	dispatcher := &permissionModuleDispatcher{startErr: startErr, order: &order}
	registerRBACLifecycle(RegisterRBACLifecycleParams{
		Lifecycle: lifecycle,
		Runtime: &PermissionRuntime{
			Initializer: &permissionModulePolicyInitializer{order: &order},
			Watcher:     watcher,
			Dispatcher:  dispatcher,
		},
	})

	err := lifecycle.hooks[0].OnStart(context.Background())
	require.ErrorIs(t, err, startErr)
	require.ErrorIs(t, err, watcherErr)
	require.Equal(t, []string{"initializer.initialize", "watcher.start", "dispatcher.start", "watcher.stop"}, order)
}

func TestPermissionModuleRequiresMetricsProvider(t *testing.T) {
	redisServer := miniredis.RunT(t)
	redisClient := rediscmd.NewClient(&rediscmd.Options{Addr: redisServer.Addr()})
	t.Cleanup(func() { _ = redisClient.Close() })
	app := fx.New(
		fxtest.WithTestLogger(t),
		fx.Supply(
			serviceconfig.RBACSettings{AppName: "aegiscore-user-service-module-test"},
			zap.NewNop(),
			fx.Annotate(redisClient, fx.As(new(rediscmd.UniversalClient)), fx.ResultTags(`name:"cache_redis"`)),
		),
		fx.Replace(
			fx.Annotate(permissionModulePolicyLoader{}, fx.As(new(permissioncasbin.Loader))),
			fx.Annotate(permissionModuleUserRoleResolver{}, fx.As(new(permissioncasbin.UserRoleResolver))),
			fx.Annotate(permissionModuleUserRoleCacheStats{}, fx.As(new(localcache.StatsSource)), fx.ResultTags(`name:"permission_rbac_user_roles_cache"`)),
			fx.Annotate(&permissionModuleStore{}, fx.As(new(permissionapplication.PermissionStore))),
			fx.Annotate(&permissionModuleOutboxStore{}, fx.As(new(permissionapplication.OutboxStore))),
			fx.Annotate(permissionModuleRevisionSource{}, fx.As(new(permissionapplication.LatestPolicyRevisionSource))),
		),
		Module,
	)

	require.Error(t, app.Err())
	require.Contains(t, app.Err().Error(), "metrics.Provider")
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
}

type permissionModuleOutboxStore struct{}

type permissionModuleRevisionSource struct{}

func (permissionModuleRevisionSource) LatestPolicyRevision(context.Context) (int64, error) {
	return 0, nil
}

func (*permissionModuleOutboxStore) Claim(context.Context, time.Time, int, time.Duration) ([]permissionapplication.OutboxClaim, error) {
	return nil, nil
}

func (*permissionModuleOutboxStore) Ack(context.Context, uuid.UUID, uuid.UUID, time.Time) (bool, error) {
	return true, nil
}

func (*permissionModuleOutboxStore) Fail(context.Context, uuid.UUID, uuid.UUID, time.Time, time.Time, string) (bool, error) {
	return true, nil
}

func (*permissionModuleOutboxStore) Backlog(context.Context, time.Time) (permissionapplication.OutboxBacklog, error) {
	return permissionapplication.OutboxBacklog{}, nil
}

type permissionModulePolicyLoader struct{}

func (permissionModulePolicyLoader) LoadPoliciesAtLeast(context.Context, int64) (permissioncasbin.PolicySet, error) {
	return permissioncasbin.PolicySet{}, nil
}

type permissionModuleFailOncePolicyLoader struct {
	err           error
	allowRecovery <-chan struct{}
	calls         atomic.Int64
}

func (l *permissionModuleFailOncePolicyLoader) LoadPoliciesAtLeast(ctx context.Context, _ int64) (permissioncasbin.PolicySet, error) {
	if l.calls.Add(1) == 1 {
		return permissioncasbin.PolicySet{}, l.err
	}
	select {
	case <-ctx.Done():
		return permissioncasbin.PolicySet{}, ctx.Err()
	case <-l.allowRecovery:
		return permissioncasbin.PolicySet{}, nil
	}
}

type permissionModuleUserRoleResolver struct{}

func (permissionModuleUserRoleResolver) RolesForUser(context.Context, uuid.UUID) ([]uuid.UUID, error) {
	return nil, nil
}

func (permissionModuleUserRoleResolver) InvalidateUserRole(uuid.UUID) {}

func (permissionModuleUserRoleResolver) InvalidateAllUserRoles() {}

type permissionModuleUserRoleCacheStats struct{}

func (permissionModuleUserRoleCacheStats) Name() string { return "rbac_user_roles" }

func (permissionModuleUserRoleCacheStats) Stats() localcache.Stats { return localcache.Stats{} }

type permissionModuleLifecycle struct {
	hooks []fx.Hook
}

func (l *permissionModuleLifecycle) Append(hook fx.Hook) {
	l.hooks = append(l.hooks, hook)
}

type permissionModulePolicyInitializer struct {
	initialized bool
	order       *[]string
}

func (i *permissionModulePolicyInitializer) InitializeFailClosed(context.Context) {
	i.initialized = true
	appendPermissionModuleOrder(i.order, "initializer.initialize")
}

type permissionModuleApplicationWatcher struct {
	started   bool
	startErr  error
	stopCalls int
	stopErr   error
	order     *[]string
}

type permissionModuleDispatcher struct {
	started   bool
	startErr  error
	stopCalls int
	stopErr   error
	order     *[]string
}

func (d *permissionModuleDispatcher) Start() error {
	appendPermissionModuleOrder(d.order, "dispatcher.start")
	d.started = d.startErr == nil
	return d.startErr
}

func (d *permissionModuleDispatcher) Stop(context.Context) error {
	appendPermissionModuleOrder(d.order, "dispatcher.stop")
	d.stopCalls++
	d.started = false
	return d.stopErr
}

func (w *permissionModuleApplicationWatcher) Start() error {
	appendPermissionModuleOrder(w.order, "watcher.start")
	if w.startErr != nil {
		return w.startErr
	}
	w.started = true
	return nil
}

func (w *permissionModuleApplicationWatcher) Stop(context.Context) error {
	appendPermissionModuleOrder(w.order, "watcher.stop")
	w.stopCalls++
	w.started = false
	return w.stopErr
}

func appendPermissionModuleOrder(order *[]string, event string) {
	if order != nil {
		*order = append(*order, event)
	}
}
