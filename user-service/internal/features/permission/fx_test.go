package permission

import (
	"context"
	"errors"
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
	settings := serviceconfig.RBACSettings{AppName: "aegiscore-user-service-module-test"}
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
			fx.Annotate(permissionModuleUserRoleCacheCloser{}, fx.As(new(userRoleResolverLifecycle)), fx.ResultTags(`name:"permission_user_role_resolver_lifecycle"`)),
			fx.Annotate(permissionModuleUserRoleCacheCloser{}, fx.As(new(permissioncasbin.UserRoleCacheCloser)), fx.ResultTags(`name:"permission_user_role_cache_closer"`)),
			fx.Annotate(permissionModuleUserRoleCacheCloser{}, fx.As(new(localcache.StatsSource)), fx.ResultTags(`name:"permission_rbac_user_roles_cache"`)),
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
	require.True(t, watcherStatus.Running())
	require.True(t, runtime.WatcherStatus.Running())
	dispatcherStatus, err := runtime.DispatcherStatus.Status(context.Background())
	require.NoError(t, err)
	require.True(t, dispatcherStatus.Running)
	require.Same(t, runtime.Dispatcher, runtime.DispatcherStatus)
	app.RequireStop()
	require.False(t, watcherStatus.Running())
	require.False(t, runtime.WatcherStatus.Running())
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
	require.NotNil(t, runtime.UserRoles)
}

func TestPermissionModuleStopsWatcherWhenLaterStartHookFails(t *testing.T) {
	redisServer := miniredis.RunT(t)
	redisClient := rediscmd.NewClient(&rediscmd.Options{Addr: redisServer.Addr()})
	provider := newPermissionModuleMetricsProvider(t, false)
	settings := serviceconfig.RBACSettings{AppName: "aegiscore-user-service-module-test"}
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
			fx.Annotate(permissionModuleUserRoleCacheCloser{}, fx.As(new(userRoleResolverLifecycle)), fx.ResultTags(`name:"permission_user_role_resolver_lifecycle"`)),
			fx.Annotate(permissionModuleUserRoleCacheCloser{}, fx.As(new(permissioncasbin.UserRoleCacheCloser)), fx.ResultTags(`name:"permission_user_role_cache_closer"`)),
			fx.Annotate(permissionModuleUserRoleCacheCloser{}, fx.As(new(localcache.StatsSource)), fx.ResultTags(`name:"permission_rbac_user_roles_cache"`)),
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
	require.False(t, watcherStatus.Running())
	require.NoError(t, redisClient.Ping(context.Background()).Err())
	require.NoError(t, redisClient.Close())
}

func TestPermissionModuleStartsFailClosedWhenInitialPolicyLoadFails(t *testing.T) {
	redisServer := miniredis.RunT(t)
	redisClient := rediscmd.NewClient(&rediscmd.Options{Addr: redisServer.Addr()})
	t.Cleanup(func() { _ = redisClient.Close() })
	provider := newPermissionModuleMetricsProvider(t, false)
	settings := serviceconfig.RBACSettings{AppName: "aegiscore-user-service-module-test"}
	settings.OutboxDispatcher = serviceconfig.DefaultOutboxDispatcherConfig()
	loadErr := errors.New("initial policy load failed")
	loader := &permissionModuleFailOncePolicyLoader{err: loadErr}
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
			fx.Annotate(permissionModuleUserRoleCacheCloser{}, fx.As(new(userRoleResolverLifecycle)), fx.ResultTags(`name:"permission_user_role_resolver_lifecycle"`)),
			fx.Annotate(permissionModuleUserRoleCacheCloser{}, fx.As(new(permissioncasbin.UserRoleCacheCloser)), fx.ResultTags(`name:"permission_user_role_cache_closer"`)),
			fx.Annotate(permissionModuleUserRoleCacheCloser{}, fx.As(new(localcache.StatsSource)), fx.ResultTags(`name:"permission_rbac_user_roles_cache"`)),
			fx.Annotate(&permissionModuleStore{}, fx.As(new(permissionapplication.PermissionStore))),
			fx.Annotate(&permissionModuleOutboxStore{}, fx.As(new(permissionapplication.OutboxStore))),
			fx.Annotate(permissionModuleRevisionSource{}, fx.As(new(permissionapplication.LatestPolicyRevisionSource))),
		),
		Module,
		fx.Populate(&authorizer, &policyHealth, &watcherStatus),
	)

	app.RequireStart()
	require.True(t, watcherStatus.Running())
	require.ErrorIs(t, policyHealth.ProjectionStatus().LastError, loadErr)
	allowed, err := authorizer.Enforce(context.Background(), uuid.NewString(), "/api/v1/users", "GET")
	require.NoError(t, err)
	require.False(t, allowed)
	app.RequireStop()
	require.False(t, watcherStatus.Running())
}

func TestStopRBACLifecycleJoinsDispatcherWatcherAndCloserErrors(t *testing.T) {
	dispatcherErr := errors.New("dispatcher stop failed")
	watcherErr := errors.New("watcher stop failed")
	closeErr := errors.New("cache close failed")
	closer := &permissionModuleErrCloser{err: closeErr}

	stopDone := make(chan error, 1)
	go func() {
		stopDone <- stopRBACLifecycle(context.Background(), func(context.Context) error { return dispatcherErr }, func(context.Context) error { return watcherErr }, closer)
	}()
	var err error
	select {
	case err = <-stopDone:
	case <-time.After(time.Second):
		t.Fatal("RBAC lifecycle stop blocked")
	}
	require.ErrorIs(t, err, watcherErr)
	require.ErrorIs(t, err, dispatcherErr)
	require.ErrorIs(t, err, closeErr)
	require.True(t, closer.closed)
}

func TestRegisterRBACLifecycleStopsWhenUserRolesStartFails(t *testing.T) {
	startErr := errors.New("user roles start failed")
	lifecycle := &permissionModuleLifecycle{}
	engine := &permissionModulePolicyInitializer{}
	watcher := &permissionModuleApplicationWatcher{}
	userRoles := &permissionModuleUserRoleLifecycle{startErr: startErr}

	registerRBACLifecycle(RegisterRBACLifecycleParams{
		Lifecycle: lifecycle,
		Runtime:   &PermissionRuntime{Initializer: engine, Watcher: watcher, Dispatcher: &permissionModuleDispatcher{}, UserRoles: userRoles},
	})
	require.Len(t, lifecycle.hooks, 1)

	err := lifecycle.hooks[0].OnStart(context.Background())
	require.ErrorIs(t, err, startErr)
	require.Equal(t, 1, userRoles.startCalls)
	require.False(t, engine.initialized)
	require.False(t, watcher.started)
}

func TestRegisterRBACLifecycleStopClosesUserRolesAfterWatcherError(t *testing.T) {
	watcherErr := errors.New("watcher stop failed")
	lifecycle := &permissionModuleLifecycle{}
	userRoles := &permissionModuleUserRoleLifecycle{closeErr: errors.New("user roles close failed")}
	watcher := &permissionModuleApplicationWatcher{stopErr: watcherErr}
	dispatcher := &permissionModuleDispatcher{stopErr: errors.New("dispatcher stop failed")}

	registerRBACLifecycle(RegisterRBACLifecycleParams{
		Lifecycle: lifecycle,
		Runtime:   &PermissionRuntime{Initializer: &permissionModulePolicyInitializer{}, Watcher: watcher, Dispatcher: dispatcher, UserRoles: userRoles},
	})
	require.Len(t, lifecycle.hooks, 1)

	err := lifecycle.hooks[0].OnStop(context.Background())
	require.ErrorIs(t, err, dispatcher.stopErr)
	require.ErrorIs(t, err, watcherErr)
	require.ErrorIs(t, err, userRoles.closeErr)
	require.Equal(t, 1, watcher.stopCalls)
	require.Equal(t, 1, dispatcher.stopCalls)
	require.Equal(t, 1, userRoles.closeCalls)
}

func TestRegisterRBACLifecycleOrdersStartAndStop(t *testing.T) {
	var order []string
	lifecycle := &permissionModuleLifecycle{}
	initializer := &permissionModulePolicyInitializer{order: &order}
	watcher := &permissionModuleApplicationWatcher{order: &order}
	dispatcher := &permissionModuleDispatcher{order: &order}
	userRoles := &permissionModuleUserRoleLifecycle{order: &order}
	registerRBACLifecycle(RegisterRBACLifecycleParams{
		Lifecycle: lifecycle,
		Runtime:   &PermissionRuntime{Initializer: initializer, Watcher: watcher, Dispatcher: dispatcher, UserRoles: userRoles},
	})

	require.NoError(t, lifecycle.hooks[0].OnStart(context.Background()))
	require.Equal(t, []string{"resolver.start", "initializer.initialize", "watcher.start", "dispatcher.start"}, order)
	order = nil
	require.NoError(t, lifecycle.hooks[0].OnStop(context.Background()))
	require.Equal(t, []string{"dispatcher.stop", "watcher.stop", "resolver.close"}, order)
}

func TestRegisterRBACLifecycleRollsBackWhenDispatcherStartFails(t *testing.T) {
	startErr := errors.New("dispatcher start failed")
	watcherErr := errors.New("watcher rollback failed")
	resolverErr := errors.New("resolver rollback failed")
	var order []string
	lifecycle := &permissionModuleLifecycle{}
	watcher := &permissionModuleApplicationWatcher{stopErr: watcherErr, order: &order}
	dispatcher := &permissionModuleDispatcher{startErr: startErr, order: &order}
	userRoles := &permissionModuleUserRoleLifecycle{closeErr: resolverErr, order: &order}
	registerRBACLifecycle(RegisterRBACLifecycleParams{
		Lifecycle: lifecycle,
		Runtime: &PermissionRuntime{
			Initializer: &permissionModulePolicyInitializer{order: &order},
			Watcher:     watcher,
			Dispatcher:  dispatcher,
			UserRoles:   userRoles,
		},
	})

	err := lifecycle.hooks[0].OnStart(context.Background())
	require.ErrorIs(t, err, startErr)
	require.ErrorIs(t, err, watcherErr)
	require.ErrorIs(t, err, resolverErr)
	require.Equal(t, []string{"resolver.start", "initializer.initialize", "watcher.start", "dispatcher.start", "watcher.stop", "resolver.close"}, order)
}

func TestUserRoleResolverHolderFailsClosedAndClosesIdempotently(t *testing.T) {
	holder := &userRoleResolverHolder{params: permissioncasbin.UserRoleResolverParams{Settings: serviceconfig.RBACSettings{UserRoleCache: serviceconfig.FeatureCacheConfig{Enabled: true, Size: 10, TTL: time.Minute, LoadTimeout: time.Second}}}}
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
			serviceconfig.RBACSettings{AppName: "aegiscore-user-service-module-test"},
			zap.NewNop(),
			fx.Annotate(redisClient, fx.As(new(rediscmd.UniversalClient)), fx.ResultTags(`name:"cache_redis"`)),
		),
		fx.Replace(
			fx.Annotate(permissionModulePolicyLoader{}, fx.As(new(permissioncasbin.Loader))),
			fx.Annotate(permissionModuleUserRoleResolver{}, fx.As(new(permissioncasbin.UserRoleResolver))),
			fx.Annotate(permissionModuleUserRoleCacheCloser{}, fx.As(new(userRoleResolverLifecycle)), fx.ResultTags(`name:"permission_user_role_resolver_lifecycle"`)),
			fx.Annotate(permissionModuleUserRoleCacheCloser{}, fx.As(new(permissioncasbin.UserRoleCacheCloser)), fx.ResultTags(`name:"permission_user_role_cache_closer"`)),
			fx.Annotate(permissionModuleUserRoleCacheCloser{}, fx.As(new(localcache.StatsSource)), fx.ResultTags(`name:"permission_rbac_user_roles_cache"`)),
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
	err  error
	done bool
}

func (l *permissionModuleFailOncePolicyLoader) LoadPoliciesAtLeast(context.Context, int64) (permissioncasbin.PolicySet, error) {
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

func (c *permissionModuleErrCloser) Start(context.Context) error { return nil }

func (c *permissionModuleErrCloser) Close() error {
	c.closed = true
	return c.err
}

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

func (w *permissionModuleApplicationWatcher) Start() {
	appendPermissionModuleOrder(w.order, "watcher.start")
	w.started = true
}

func (w *permissionModuleApplicationWatcher) Stop(context.Context) error {
	appendPermissionModuleOrder(w.order, "watcher.stop")
	w.stopCalls++
	w.started = false
	return w.stopErr
}

type permissionModuleUserRoleLifecycle struct {
	startCalls int
	closeCalls int
	startErr   error
	closeErr   error
	order      *[]string
}

func (l *permissionModuleUserRoleLifecycle) Start(context.Context) error {
	appendPermissionModuleOrder(l.order, "resolver.start")
	l.startCalls++
	return l.startErr
}

func (l *permissionModuleUserRoleLifecycle) Close() error {
	appendPermissionModuleOrder(l.order, "resolver.close")
	l.closeCalls++
	return l.closeErr
}

func appendPermissionModuleOrder(order *[]string, event string) {
	if order != nil {
		*order = append(*order, event)
	}
}
