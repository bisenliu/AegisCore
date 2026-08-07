package casbin

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	casbinlib "github.com/casbin/casbin/v3"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"github.com/aegiscore/user-service/internal/shared/rbacbaseline"
)

type loaderFunc func(context.Context, int64) (PolicySet, error)

func (f loaderFunc) LoadPoliciesAtLeast(ctx context.Context, targetRevision int64) (PolicySet, error) {
	return f(ctx, targetRevision)
}

type blockingUserRoleResolver struct {
	started chan struct{}
	release chan struct{}
	roles   []uuid.UUID
}

func (r *blockingUserRoleResolver) RolesForUser(context.Context, uuid.UUID) ([]uuid.UUID, error) {
	close(r.started)
	<-r.release
	return r.roles, nil
}

func (*blockingUserRoleResolver) InvalidateUserRole(uuid.UUID) {}
func (*blockingUserRoleResolver) InvalidateAllUserRoles()      {}

type reloadMetricsRecorder struct {
	succeeded atomic.Int64
	failed    atomic.Int64
	status    atomic.Bool
}

func (m *reloadMetricsRecorder) ReloadSucceeded()      { m.succeeded.Add(1) }
func (m *reloadMetricsRecorder) ReloadFailed()         { m.failed.Add(1) }
func (m *reloadMetricsRecorder) SetLastStatus(ok bool) { m.status.Store(ok) }

type faultInjectedRoleResolver struct {
	mu             sync.RWMutex
	roles          map[uuid.UUID][]uuid.UUID
	started        chan struct{}
	release        chan struct{}
	invalidateUser atomic.Int64
	invalidateAll  atomic.Int64
}

func newFaultInjectedRoleResolver(roles map[uuid.UUID][]uuid.UUID) *faultInjectedRoleResolver {
	return &faultInjectedRoleResolver{roles: roles}
}

func (r *faultInjectedRoleResolver) RolesForUser(_ context.Context, userID uuid.UUID) ([]uuid.UUID, error) {
	if r.started != nil {
		close(r.started)
	}
	if r.release != nil {
		<-r.release
	}
	r.mu.RLock()
	defer r.mu.RUnlock()
	return append([]uuid.UUID(nil), r.roles[userID]...), nil
}

func (r *faultInjectedRoleResolver) InvalidateUserRole(uuid.UUID) { r.invalidateUser.Add(1) }
func (r *faultInjectedRoleResolver) InvalidateAllUserRoles()      { r.invalidateAll.Add(1) }

func requireEventuallyProjection(t *testing.T, engine *Engine, revision int64) {
	t.Helper()
	require.Eventually(t, func() bool {
		status := engine.ProjectionStatus()
		return status.Ready() && status.AppliedRevision == revision && status.TargetRevision == revision
	}, time.Second, time.Millisecond, "projection status: %+v", engine.ProjectionStatus())
}

func requireEventuallyAllowed(t *testing.T, engine *Engine, userID uuid.UUID, path string, method string, want bool) {
	t.Helper()
	require.Eventually(t, func() bool {
		allowed, err := engine.Enforce(context.Background(), userID, path, method)
		return err == nil && allowed == want
	}, time.Second, time.Millisecond, "projection status: %+v", engine.ProjectionStatus())
}

func TestEngineEnforceAllowDenyAndDoesNotReload(t *testing.T) {
	ctrl := gomock.NewController(t)
	userID := uuid.MustParse("018f0000-0000-7000-8000-000000000301")
	roleID := uuid.MustParse("018f0000-0000-7000-8000-000000000302")
	var loads atomic.Int64
	loader := loaderFunc(func(_ context.Context, _ int64) (PolicySet, error) {
		loads.Add(1)
		return policy(1, roleID, "/api/v1/users", "GET"), nil
	})
	roles := NewMockUserRoleResolver(ctrl)
	roles.EXPECT().RolesForUser(gomock.Any(), userID).Return([]uuid.UUID{roleID}, nil).Times(2)
	engine := newStartedEngine(t, loader, NopReloadMetrics(), roles)
	applied, err := engine.ReloadToRevision(context.Background(), 1)
	require.NoError(t, err)
	require.Equal(t, int64(1), applied)

	allowed, err := engine.Enforce(context.Background(), userID, "/api/v1/users", "GET")
	require.NoError(t, err)
	require.True(t, allowed)
	denied, err := engine.Enforce(context.Background(), userID, "/api/v1/users", "POST")
	require.NoError(t, err)
	require.False(t, denied)
	require.Equal(t, int64(1), loads.Load())
}

func TestEngineSuperAdminWildcard(t *testing.T) {
	ctrl := gomock.NewController(t)
	userID := uuid.MustParse("018f0000-0000-7000-8000-000000000401")
	superAdminRoleID := uuid.MustParse(rbacbaseline.SuperAdminRoleID)
	loader := loaderFunc(func(context.Context, int64) (PolicySet, error) {
		return policy(1, superAdminRoleID, policyWildcard, policyWildcard), nil
	})
	roles := NewMockUserRoleResolver(ctrl)
	roles.EXPECT().RolesForUser(gomock.Any(), userID).Return([]uuid.UUID{superAdminRoleID}, nil)
	engine := newStartedEngine(t, loader, NopReloadMetrics(), roles)
	_, err := engine.ReloadToRevision(context.Background(), 1)
	require.NoError(t, err)
	allowed, err := engine.Enforce(context.Background(), userID, "/api/v1/anything/:id", "DELETE")
	require.NoError(t, err)
	require.True(t, allowed)
}

func TestEngineReloadFailurePreservesProjectionButFailsClosedUntilRecovery(t *testing.T) {
	ctrl := gomock.NewController(t)
	userID := uuid.MustParse("018f0000-0000-7000-8000-000000000501")
	roleID := uuid.MustParse("018f0000-0000-7000-8000-000000000502")
	loadErr := errors.New("reload failed")
	var call atomic.Int64
	loader := loaderFunc(func(_ context.Context, _ int64) (PolicySet, error) {
		switch call.Add(1) {
		case 1:
			return policy(1, roleID, "/api/v1/users", "GET"), nil
		case 2:
			return PolicySet{}, loadErr
		default:
			return policy(2, roleID, "/api/v1/users", "GET"), nil
		}
	})
	roles := NewMockUserRoleResolver(ctrl)
	roles.EXPECT().RolesForUser(gomock.Any(), userID).Return([]uuid.UUID{roleID}, nil)
	metrics := &reloadMetricsRecorder{}
	engine := newStartedEngine(t, loader, metrics, roles)

	_, err := engine.ReloadToRevision(context.Background(), 1)
	require.NoError(t, err)
	_, err = engine.ReloadToRevision(context.Background(), 2)
	require.ErrorIs(t, err, loadErr)
	status := engine.ProjectionStatus()
	require.Equal(t, int64(1), status.AppliedRevision)
	require.Equal(t, int64(2), status.TargetRevision)
	require.False(t, status.Ready())

	allowed, err := engine.Enforce(context.Background(), userID, "/api/v1/users", "GET")
	require.NoError(t, err)
	require.False(t, allowed)

	applied, err := engine.ReloadToRevision(context.Background(), 2)
	require.NoError(t, err)
	require.Equal(t, int64(2), applied)
	require.True(t, engine.ProjectionStatus().Ready())
	allowed, err = engine.Enforce(context.Background(), userID, "/api/v1/users", "GET")
	require.NoError(t, err)
	require.True(t, allowed)
	require.Equal(t, int64(2), metrics.succeeded.Load())
	require.Equal(t, int64(1), metrics.failed.Load())
	require.True(t, metrics.status.Load())
}

func TestEngineCandidateSwapIsHigherOnlyAndEqualIsIdempotent(t *testing.T) {
	engine := newStartedEngine(t, nil, NopReloadMetrics(), nil)
	newer := mustEnforcer(t, policy(2, uuid.New(), "/new", "GET"))
	older := mustEnforcer(t, policy(1, uuid.New(), "/old", "GET"))
	equal := mustEnforcer(t, policy(2, uuid.New(), "/equal", "GET"))
	olderBuilt := make(chan struct{})
	applyOlder := make(chan struct{})
	olderDone := make(chan struct{})

	engine.targetRevision = 2
	go func() {
		close(olderBuilt)
		<-applyOlder
		engine.applyCandidate(nil, PolicySet{Revision: 1}, older, false)
		close(olderDone)
	}()
	<-olderBuilt
	require.True(t, engine.applyCandidate(nil, PolicySet{Revision: 2}, newer, false))
	close(applyOlder)
	<-olderDone
	require.True(t, engine.applyCandidate(nil, PolicySet{Revision: 2}, equal, false))

	engine.mu.RLock()
	require.Same(t, newer, engine.enforcer)
	engine.mu.RUnlock()
	require.Equal(t, int64(2), engine.AppliedRevision())
}

func TestEngineRefreshReplacesStaleSnapshotAtEqualRevision(t *testing.T) {
	userID := uuid.MustParse("018f0000-0000-7000-8000-000000000561")
	roleID := uuid.MustParse("018f0000-0000-7000-8000-000000000562")
	var calls atomic.Int64
	loader := loaderFunc(func(context.Context, int64) (PolicySet, error) {
		if calls.Add(1) == 1 {
			return policy(2, roleID, "/api/v1/stale", "GET"), nil
		}
		return policy(2, roleID, "/api/v1/current", "GET"), nil
	})
	resolver := newFaultInjectedRoleResolver(map[uuid.UUID][]uuid.UUID{userID: {roleID}})
	engine := newStartedEngine(t, loader, NopReloadMetrics(), resolver)

	_, err := engine.ReloadToRevision(context.Background(), 2)
	require.NoError(t, err)
	_, err = engine.ReloadToRevision(context.Background(), 2)
	require.NoError(t, err)
	require.Equal(t, int64(1), calls.Load(), "普通 reload 对相同 revision 保持幂等")
	requireEventuallyAllowed(t, engine, userID, "/api/v1/stale", "GET", true)

	applied, err := engine.RefreshToRevision(context.Background(), 2)
	require.NoError(t, err)
	require.Equal(t, int64(2), applied)
	require.Equal(t, int64(2), calls.Load())
	requireEventuallyAllowed(t, engine, userID, "/api/v1/current", "GET", true)
	requireEventuallyAllowed(t, engine, userID, "/api/v1/stale", "GET", false)
}

func TestEngineForceJoiningInFlightReloadReadsSnapshotAgain(t *testing.T) {
	userID := uuid.MustParse("018f0000-0000-7000-8000-000000000571")
	roleID := uuid.MustParse("018f0000-0000-7000-8000-000000000572")
	ordinaryLoadStarted := make(chan struct{})
	releaseOrdinaryLoad := make(chan struct{})
	var calls atomic.Int64
	loader := loaderFunc(func(ctx context.Context, target int64) (PolicySet, error) {
		switch calls.Add(1) {
		case 1:
			return policy(2, roleID, "/api/v1/initial", "GET"), nil
		case 2:
			close(ordinaryLoadStarted)
			select {
			case <-releaseOrdinaryLoad:
			case <-ctx.Done():
				return PolicySet{}, ctx.Err()
			}
			return policy(target, roleID, "/api/v1/before-force", "GET"), nil
		default:
			return policy(target, roleID, "/api/v1/after-force", "GET"), nil
		}
	})
	resolver := newFaultInjectedRoleResolver(map[uuid.UUID][]uuid.UUID{userID: {roleID}})
	engine := newStartedEngine(t, loader, NopReloadMetrics(), resolver)
	_, err := engine.ReloadToRevision(context.Background(), 2)
	require.NoError(t, err)

	ordinaryDone := make(chan error, 1)
	go func() {
		_, reloadErr := engine.ReloadToRevision(context.Background(), 3)
		ordinaryDone <- reloadErr
	}()
	<-ordinaryLoadStarted
	forceDone := make(chan error, 1)
	go func() {
		_, refreshErr := engine.RefreshToRevision(context.Background(), 3)
		forceDone <- refreshErr
	}()
	require.Eventually(t, func() bool {
		engine.mu.RLock()
		defer engine.mu.RUnlock()
		return engine.flight != nil && engine.flight.force
	}, time.Second, time.Millisecond)
	close(releaseOrdinaryLoad)

	require.NoError(t, <-ordinaryDone)
	require.NoError(t, <-forceDone)
	require.Equal(t, int64(3), calls.Load())
	requireEventuallyAllowed(t, engine, userID, "/api/v1/after-force", "GET", true)
	requireEventuallyAllowed(t, engine, userID, "/api/v1/before-force", "GET", false)
}

func TestEngineFaultInjectionOutOfOrderReloadKeepsLatestProjection(t *testing.T) {
	userID := uuid.MustParse("018f0000-0000-7000-8000-000000000551")
	oldRoleID := uuid.MustParse("018f0000-0000-7000-8000-000000000552")
	newRoleID := uuid.MustParse("018f0000-0000-7000-8000-000000000553")
	started := make(chan struct{})
	release := make(chan struct{})
	var calls atomic.Int64
	loader := loaderFunc(func(ctx context.Context, target int64) (PolicySet, error) {
		call := calls.Add(1)
		if call == 1 {
			close(started)
			select {
			case <-release:
			case <-ctx.Done():
				return PolicySet{}, ctx.Err()
			}
			return policy(1, oldRoleID, "/api/v1/old", "GET"), nil
		}
		return policy(target, newRoleID, "/api/v1/new", "GET"), nil
	})
	resolver := newFaultInjectedRoleResolver(map[uuid.UUID][]uuid.UUID{userID: {newRoleID}})
	engine := newStartedEngine(t, loader, NopReloadMetrics(), resolver)

	first := make(chan error, 1)
	go func() {
		_, err := engine.ReloadToRevision(context.Background(), 1)
		first <- err
	}()
	<-started
	second := make(chan error, 1)
	go func() {
		_, err := engine.ReloadToRevision(context.Background(), 2)
		second <- err
	}()
	require.Eventually(t, func() bool { return engine.ProjectionStatus().TargetRevision == 2 }, time.Second, time.Millisecond)
	close(release)

	require.NoError(t, <-first)
	require.NoError(t, <-second)
	requireEventuallyProjection(t, engine, 2)
	requireEventuallyAllowed(t, engine, userID, "/api/v1/new", "GET", true)
	requireEventuallyAllowed(t, engine, userID, "/api/v1/old", "GET", false)
	require.Equal(t, int64(2), calls.Load())
}

func TestEngineCanceledHigherTargetIsObservedAndFailsClosed(t *testing.T) {
	roleID := uuid.New()
	engine := newStartedEngine(t, loaderFunc(func(context.Context, int64) (PolicySet, error) {
		return policy(1, roleID, "/api/v1/users", "GET"), nil
	}), NopReloadMetrics(), nil)
	_, err := engine.ReloadToRevision(context.Background(), 1)
	require.NoError(t, err)

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	applied, err := engine.ReloadToRevision(ctx, 3)
	require.ErrorIs(t, err, context.Canceled)
	require.Equal(t, int64(1), applied)
	status := engine.ProjectionStatus()
	require.Equal(t, int64(3), status.TargetRevision)
	require.False(t, status.Ready())
}

func TestEngineEnforceReturnsRoleResolverError(t *testing.T) {
	ctrl := gomock.NewController(t)
	userID := uuid.MustParse("018f0000-0000-7000-8000-000000000601")
	roleID := uuid.MustParse("018f0000-0000-7000-8000-000000000602")
	resolveErr := errors.New("resolve roles failed")
	loader := loaderFunc(func(context.Context, int64) (PolicySet, error) {
		return policy(1, roleID, "/api/v1/users", "GET"), nil
	})
	roles := NewMockUserRoleResolver(ctrl)
	roles.EXPECT().RolesForUser(gomock.Any(), userID).Return(nil, resolveErr)
	engine := newStartedEngine(t, loader, NopReloadMetrics(), roles)
	_, err := engine.ReloadToRevision(context.Background(), 1)
	require.NoError(t, err)

	allowed, err := engine.Enforce(context.Background(), userID, "/api/v1/users", "GET")
	require.ErrorIs(t, err, resolveErr)
	require.False(t, allowed)
}

func TestEngineEnforceFailsClosedWhenProjectionBecomesStaleDuringRoleResolution(t *testing.T) {
	userID := uuid.New()
	roleID := uuid.New()
	resolver := &blockingUserRoleResolver{started: make(chan struct{}), release: make(chan struct{}), roles: []uuid.UUID{roleID}}
	engine := newStartedEngine(t, loaderFunc(func(context.Context, int64) (PolicySet, error) {
		return policy(1, roleID, "/api/v1/users", "GET"), nil
	}), NopReloadMetrics(), resolver)
	_, err := engine.ReloadToRevision(context.Background(), 1)
	require.NoError(t, err)

	result := make(chan bool, 1)
	go func() {
		allowed, enforceErr := engine.Enforce(context.Background(), userID, "/api/v1/users", "GET")
		require.NoError(t, enforceErr)
		result <- allowed
	}()
	<-resolver.started
	engine.ObserveTargetRevision(2)
	close(resolver.release)
	require.False(t, <-result)
}

func TestEngineWaiterCancellationDoesNotCancelSharedReload(t *testing.T) {
	started := make(chan struct{})
	release := make(chan struct{})
	loaderCanceled := make(chan struct{}, 1)
	loader := loaderFunc(func(ctx context.Context, _ int64) (PolicySet, error) {
		close(started)
		select {
		case <-release:
			return PolicySet{Revision: 2}, nil
		case <-ctx.Done():
			loaderCanceled <- struct{}{}
			return PolicySet{}, ctx.Err()
		}
	})
	engine := newStartedEngine(t, loader, NopReloadMetrics(), nil)
	leaderCtx, cancelLeader := context.WithCancel(context.Background())
	leaderResult := make(chan error, 1)
	go func() {
		_, err := engine.ReloadToRevision(leaderCtx, 1)
		leaderResult <- err
	}()
	<-started
	waiterResult := make(chan error, 1)
	go func() {
		_, err := engine.ReloadToRevision(context.Background(), 2)
		waiterResult <- err
	}()
	require.Eventually(t, func() bool { return engine.ProjectionStatus().TargetRevision == 2 }, time.Second, time.Millisecond)
	cancelLeader()
	require.ErrorIs(t, <-leaderResult, context.Canceled)
	select {
	case <-loaderCanceled:
		t.Fatal("one waiter canceled the shared reload")
	default:
	}
	close(release)
	require.NoError(t, <-waiterResult)
	require.Equal(t, int64(2), engine.AppliedRevision())
}

func TestEngineConcurrentReloadAndRefreshWaitersConvergeToHighestTarget(t *testing.T) {
	const latest = int64(25)
	started := make(chan struct{})
	release := make(chan struct{})
	var calls atomic.Int64
	loader := loaderFunc(func(ctx context.Context, target int64) (PolicySet, error) {
		if calls.Add(1) == 1 {
			require.Equal(t, int64(3), target)
			close(started)
			select {
			case <-release:
			case <-ctx.Done():
				return PolicySet{}, ctx.Err()
			}
			return PolicySet{Revision: target}, nil
		}
		require.Equal(t, latest, target)
		return PolicySet{Revision: latest}, nil
	})
	engine := newStartedEngine(t, loader, NopReloadMetrics(), nil)

	results := make(chan error, 4)
	go func() {
		applied, err := engine.ReloadToRevision(context.Background(), 3)
		if err == nil && applied < 3 {
			err = fmt.Errorf("reload target 3 returned applied %d", applied)
		}
		results <- err
	}()
	<-started

	for _, call := range []struct {
		target int64
		force  bool
	}{
		{target: 20, force: true},
		{target: 12, force: false},
		{target: latest, force: true},
	} {
		go func(target int64, force bool) {
			var (
				applied int64
				err     error
			)
			if force {
				applied, err = engine.RefreshToRevision(context.Background(), target)
			} else {
				applied, err = engine.ReloadToRevision(context.Background(), target)
			}
			if err == nil && applied < target {
				err = fmt.Errorf("target %d returned applied %d", target, applied)
			}
			results <- err
		}(call.target, call.force)
	}

	require.Eventually(t, func() bool {
		engine.mu.RLock()
		defer engine.mu.RUnlock()
		return engine.targetRevision == latest && engine.flight != nil && engine.flight.force
	}, time.Second, time.Millisecond)
	close(release)
	for range 4 {
		require.NoError(t, <-results)
	}
	require.Equal(t, latest, engine.AppliedRevision())
	require.Equal(t, int64(2), calls.Load(), "force refresh joining an in-flight ordinary reload must read a fresh snapshot")
}

func TestEngineAllWaitersCancelCancelsSharedReloadAndFailsClosed(t *testing.T) {
	started := make(chan struct{})
	loaderCanceled := make(chan struct{})
	var cancelOnce sync.Once
	loader := loaderFunc(func(ctx context.Context, _ int64) (PolicySet, error) {
		close(started)
		<-ctx.Done()
		cancelOnce.Do(func() { close(loaderCanceled) })
		return PolicySet{}, ctx.Err()
	})
	engine := newStartedEngine(t, loader, NopReloadMetrics(), nil)
	firstCtx, cancelFirst := context.WithCancel(context.Background())
	secondCtx, cancelSecond := context.WithCancel(context.Background())
	firstResult := make(chan error, 1)
	secondResult := make(chan error, 1)

	go func() {
		_, err := engine.ReloadToRevision(firstCtx, 1)
		firstResult <- err
	}()
	<-started
	go func() {
		_, err := engine.ReloadToRevision(secondCtx, 2)
		secondResult <- err
	}()
	require.Eventually(t, func() bool { return engine.ProjectionStatus().TargetRevision == 2 }, time.Second, time.Millisecond)

	cancelFirst()
	require.ErrorIs(t, <-firstResult, context.Canceled)
	select {
	case <-loaderCanceled:
		t.Fatal("shared reload was canceled while another waiter still needed it")
	default:
	}
	cancelSecond()
	require.ErrorIs(t, <-secondResult, context.Canceled)
	select {
	case <-loaderCanceled:
	case <-time.After(time.Second):
		t.Fatal("shared reload was not canceled after all waiters canceled")
	}

	status := engine.ProjectionStatus()
	require.Equal(t, int64(0), status.AppliedRevision)
	require.Equal(t, int64(2), status.TargetRevision)
	require.False(t, status.Ready())
	require.ErrorIs(t, status.LastError, context.Canceled)
}

func TestEngineLifecycleRootCancellationCancelsSharedReload(t *testing.T) {
	started := make(chan struct{})
	loaderCanceled := make(chan struct{})
	var cancelOnce sync.Once
	loader := loaderFunc(func(ctx context.Context, _ int64) (PolicySet, error) {
		close(started)
		<-ctx.Done()
		cancelOnce.Do(func() { close(loaderCanceled) })
		return PolicySet{}, ctx.Err()
	})
	engine := newStartedEngine(t, loader, NopReloadMetrics(), nil)
	lifecycleCtx, cancelLifecycle := context.WithCancel(context.Background())
	require.NoError(t, engine.Start(lifecycleCtx))
	result := make(chan error, 1)

	go func() {
		_, err := engine.ReloadToRevision(context.Background(), 1)
		result <- err
	}()
	<-started
	cancelLifecycle()

	select {
	case <-loaderCanceled:
	case <-time.After(time.Second):
		t.Fatal("engine lifecycle root did not cancel shared reload")
	}
	require.ErrorIs(t, <-result, context.Canceled)
	status := engine.ProjectionStatus()
	require.Equal(t, int64(0), status.AppliedRevision)
	require.Equal(t, int64(1), status.TargetRevision)
	require.False(t, status.Ready())
	require.ErrorIs(t, status.LastError, context.Canceled)
}

func TestEngineNewWaiterStartsFreshFlightAfterSoleWaiterCancels(t *testing.T) {
	firstStarted := make(chan struct{})
	firstCanceled := make(chan struct{})
	var calls atomic.Int64
	loader := loaderFunc(func(ctx context.Context, target int64) (PolicySet, error) {
		if calls.Add(1) == 1 {
			close(firstStarted)
			<-ctx.Done()
			close(firstCanceled)
			return PolicySet{}, ctx.Err()
		}
		return PolicySet{Revision: target}, nil
	})
	engine := newStartedEngine(t, loader, NopReloadMetrics(), nil)
	ctx, cancel := context.WithCancel(context.Background())
	firstResult := make(chan error, 1)
	go func() {
		_, err := engine.ReloadToRevision(ctx, 1)
		firstResult <- err
	}()
	<-firstStarted
	cancel()
	require.ErrorIs(t, <-firstResult, context.Canceled)

	applied, err := engine.ReloadToRevision(context.Background(), 2)
	require.NoError(t, err)
	require.Equal(t, int64(2), applied)
	<-firstCanceled
	require.Equal(t, int64(2), calls.Load())
	require.True(t, engine.ProjectionStatus().Ready())
}

func TestEngineCoalescesOneHundredConcurrentTargetsToLatest(t *testing.T) {
	const latest = int64(137)
	started := make(chan struct{})
	release := make(chan struct{})
	var calls atomic.Int64
	loader := loaderFunc(func(ctx context.Context, _ int64) (PolicySet, error) {
		if calls.Add(1) == 1 {
			close(started)
		}
		select {
		case <-release:
			return PolicySet{Revision: latest}, nil
		case <-ctx.Done():
			return PolicySet{}, ctx.Err()
		}
	})
	engine := newStartedEngine(t, loader, NopReloadMetrics(), nil)

	results := make(chan error, 100)
	var wg sync.WaitGroup
	for target := int64(1); target <= 100; target++ {
		wg.Add(1)
		go func(target int64) {
			defer wg.Done()
			applied, err := engine.ReloadToRevision(context.Background(), target)
			if err == nil && applied < target {
				err = fmt.Errorf("target %d returned applied %d", target, applied)
			}
			results <- err
		}(target)
	}
	<-started
	require.Eventually(t, func() bool { return engine.ProjectionStatus().TargetRevision == 100 }, time.Second, time.Millisecond)
	close(release)
	wg.Wait()
	close(results)
	for err := range results {
		require.NoError(t, err)
	}
	require.Equal(t, latest, engine.AppliedRevision())
	require.Equal(t, int64(1), calls.Load(), "coalescing should load latest once, not every intermediate revision")
}

func TestEngineFaultInjectionOneHundredConcurrentWritesConvergeToAuthorizationProjection(t *testing.T) {
	const latest = int64(100)
	userID := uuid.MustParse("018f0000-0000-7000-8000-000000000571")
	roleID := uuid.MustParse("018f0000-0000-7000-8000-000000000572")
	started := make(chan struct{})
	release := make(chan struct{})
	var calls atomic.Int64
	loader := loaderFunc(func(ctx context.Context, _ int64) (PolicySet, error) {
		if calls.Add(1) == 1 {
			close(started)
		}
		select {
		case <-release:
			return policy(latest, roleID, "/api/v1/users", "GET"), nil
		case <-ctx.Done():
			return PolicySet{}, ctx.Err()
		}
	})
	resolver := newFaultInjectedRoleResolver(map[uuid.UUID][]uuid.UUID{userID: {roleID}})
	engine := newStartedEngine(t, loader, NopReloadMetrics(), resolver)

	results := make(chan error, 100)
	var wg sync.WaitGroup
	for revision := int64(1); revision <= latest; revision++ {
		wg.Add(1)
		go func(revision int64) {
			defer wg.Done()
			applied, err := engine.ReloadToRevision(context.Background(), revision)
			if err == nil && applied < revision {
				err = fmt.Errorf("database revision %d returned applied revision %d", revision, applied)
			}
			results <- err
		}(revision)
	}
	<-started
	require.Eventually(t, func() bool { return engine.ProjectionStatus().TargetRevision == latest }, time.Second, time.Millisecond)
	close(release)
	wg.Wait()
	close(results)
	for err := range results {
		require.NoError(t, err)
	}
	requireEventuallyProjection(t, engine, latest)
	requireEventuallyAllowed(t, engine, userID, "/api/v1/users", "GET", true)
	requireEventuallyAllowed(t, engine, userID, "/api/v1/users", "DELETE", false)
	require.Equal(t, int64(1), calls.Load(), "100 concurrent writes should coalesce to one latest projection load")
}

func TestEngineInitializeUsesTargetZeroAndCallerContext(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	loader := loaderFunc(func(loaderCtx context.Context, target int64) (PolicySet, error) {
		require.Equal(t, int64(0), target)
		cancel()
		<-loaderCtx.Done()
		return PolicySet{}, loaderCtx.Err()
	})
	engine := newStartedEngine(t, loader, NopReloadMetrics(), nil)
	engine.InitializeFailClosed(ctx)
	require.Eventually(t, func() bool { return errors.Is(engine.LastError(), context.Canceled) }, time.Second, time.Millisecond)
	require.False(t, engine.ProjectionStatus().Ready())
}

func TestEngineInvalidatesUserRoleResolver(t *testing.T) {
	ctrl := gomock.NewController(t)
	userID := uuid.MustParse("018f0000-0000-7000-8000-000000000505")
	roles := NewMockUserRoleResolver(ctrl)
	gomock.InOrder(
		roles.EXPECT().InvalidateUserRole(userID),
		roles.EXPECT().InvalidateAllUserRoles(),
	)
	engine := NewEngine(nil, NopReloadMetrics(), roles)
	engine.InvalidateUserRole(userID)
	engine.InvalidateAllUserRoles()
}

func newStartedEngine(t *testing.T, loader Loader, metrics ReloadMetrics, roles UserRoleResolver) *Engine {
	t.Helper()
	engine := NewEngine(loader, metrics, roles)
	require.NoError(t, engine.Start(context.Background()))
	t.Cleanup(func() { require.NoError(t, engine.Stop(context.Background())) })
	return engine
}

func policy(revision int64, roleID uuid.UUID, path, method string) PolicySet {
	return PolicySet{
		Revision:        revision,
		PermissionRules: []PermissionRule{{RoleID: roleID, PathTemplate: path, HTTPMethod: method}},
	}
}

func mustEnforcer(t *testing.T, policySet PolicySet) *casbinlib.Enforcer {
	t.Helper()
	engine := NewEngine(loaderFunc(func(context.Context, int64) (PolicySet, error) {
		return policySet, nil
	}), NopReloadMetrics(), nil)
	_, enforcer, err := engine.buildEnforcer(context.Background(), policySet.Revision)
	require.NoError(t, err)
	return enforcer
}
