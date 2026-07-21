package redis

import (
	"context"
	"runtime"
	"strconv"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	rediscache "github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
	"go.uber.org/zap"

	runtimeconfig "github.com/aegiscore/common/runtime/config"
	"github.com/aegiscore/common/runtime/workerpool"
	authapplication "github.com/aegiscore/user-service/internal/features/auth/application"
	authdomain "github.com/aegiscore/user-service/internal/features/auth/domain"
)

func TestSessionStoreDeleteAllUserSessions(t *testing.T) {
	redisServer := miniredis.RunT(t)
	store := newTestSessionStore(redisServer)
	ctx := context.Background()
	indexKey := store.userSessionsKey(sessionTestUserID.String())
	for _, sessionID := range []string{"s-1", "s-2"} {
		{
			err := store.CreateSession(ctx, authdomain.AuthSession{UserID: sessionTestUserID.String(), SessionID: sessionID, TokenVersion: 1, ExpiresAt: time.Now().Add(time.Hour)}, time.Hour, defaultMaxActiveSessionsPerUser())
			require.NoError(t, err,
				"CreateSession: %v", err)
		}

	}
	{
		err := store.redis.Set(ctx, store.sessionKey(sessionTestUserID.String(), "expired-session"), "stale", 0).Err()
		require.NoError(t, err,
			"Set expired session: %v", err)
	}
	{

		err := store.redis.ZAdd(ctx, indexKey, rediscache.Z{Score: float64(time.Now().Add(-time.Minute).Unix()), Member: "expired-session"}).Err()
		require.NoError(t, err,
			"ZAdd expired session: %v", err)
	}
	{

		err := store.redis.ZAdd(ctx, indexKey, rediscache.Z{Score: float64(time.Now().Add(time.Hour).Unix()), Member: "missing-session"}).Err()
		require.NoError(t, err,
			"ZAdd missing session: %v", err)
	}
	{

		err := store.DeleteAllUserSessions(ctx, sessionTestUserID.String())
		require.NoError(t, err,
			"DeleteAllUserSessions: %v", err)
	}

	waitForRedisCondition(t, func() bool {
		return !redisServer.Exists(store.sessionKey(sessionTestUserID.String(), "s-1")) &&
			!redisServer.Exists(store.sessionKey(sessionTestUserID.String(), "s-2")) &&
			!redisServer.Exists(indexKey)
	}, "user sessions were not fully deleted")
	require.True(t, redisServer.Exists(store.sessionKey(sessionTestUserID.String(), "expired-session")),
		"expired session key was deleted despite expired index member cleanup")

}

func TestSessionStoreDeleteAllUserSessionsPurgesInBatches(t *testing.T) {
	redisServer := miniredis.RunT(t)
	store := newTestSessionStore(redisServer)
	ctx := context.Background()
	indexKey := store.userSessionsKey(sessionTestUserID.String())
	sessionCount := int(deleteAllUserSessionsBatchSize)*2 + 1
	for i := 0; i < sessionCount; i++ {
		sessionID := "bulk-" + strconv.Itoa(i)
		{
			err := store.redis.Set(ctx, store.sessionKey(sessionTestUserID.String(), sessionID), "{}", time.Hour).Err()
			require.NoError(t, err,
				"Set session %d: %v", i, err)
		}
		{

			err := store.redis.ZAdd(ctx, indexKey, rediscache.Z{Score: float64(time.Now().Add(time.Hour).Unix()), Member: sessionID}).Err()
			require.NoError(t, err,
				"ZAdd session %d: %v", i, err)
		}

	}
	{

		err := store.DeleteAllUserSessions(ctx, sessionTestUserID.String())
		require.NoError(t, err,
			"DeleteAllUserSessions: %v", err)
	}

	waitForRedisCondition(t, func() bool {
		for i := 0; i < sessionCount; i++ {
			if redisServer.Exists(store.sessionKey(sessionTestUserID.String(), "bulk-"+strconv.Itoa(i))) {
				return false
			}
		}
		return !redisServer.Exists(indexKey)
	}, "batched user sessions were not fully deleted")
}

func TestSessionStoreDeleteAllUserSessionsDoesNotDeleteNewSessionsAfterDetach(t *testing.T) {
	redisServer := miniredis.RunT(t)
	store := newTestSessionStore(redisServer)
	ctx := context.Background()
	oldSessionID := "old-before-detach"
	newSessionID := "new-after-detach"
	{
		err := store.CreateSession(ctx, authdomain.AuthSession{UserID: sessionTestUserID.String(), SessionID: oldSessionID, TokenVersion: 1}, time.Hour, defaultMaxActiveSessionsPerUser())
		require.NoError(t, err,
			"CreateSession old: %v", err)
	}
	{

		err := store.DeleteAllUserSessions(ctx, sessionTestUserID.String())
		require.NoError(t, err,
			"DeleteAllUserSessions: %v", err)
	}
	{

		err := store.CreateSession(ctx, authdomain.AuthSession{UserID: sessionTestUserID.String(), SessionID: newSessionID, TokenVersion: 2}, time.Hour, defaultMaxActiveSessionsPerUser())
		require.NoError(t, err,
			"CreateSession new: %v", err)
	}

	waitForRedisCondition(t, func() bool {
		return !redisServer.Exists(store.sessionKey(sessionTestUserID.String(), oldSessionID))
	}, "detached old session was not purged")
	require.True(t, redisServer.Exists(store.sessionKey(sessionTestUserID.String(), newSessionID)),
		"new session created after detach was deleted")

	members, err := store.redis.ZRange(ctx, store.userSessionsKey(sessionTestUserID.String()), 0, -1).Result()
	require.NoError(t, err,
		"ZRange new index: %v", err)
	require.False(t, len(members) != 1 || members[0] != newSessionID,
		"members = %v, want only new session", members)

}

func TestSessionStoreDeleteAllUserSessionsReturnsSubmitError(t *testing.T) {
	redisServer := miniredis.RunT(t)
	store := newTestSessionStore(redisServer)
	ctrl := gomock.NewController(t)
	metrics := NewMockMetrics(ctrl)
	store.metrics = metrics
	store.purgePool = rejectingPurgeTaskPool{err: workerpool.ErrQueueFull}
	ctx := context.Background()
	metrics.EXPECT().SessionPurgeSubmitFailed(gomock.Any())
	{
		err := store.CreateSession(ctx, authdomain.AuthSession{UserID: sessionTestUserID.String(), SessionID: "s-rejected", TokenVersion: 1}, time.Hour, defaultMaxActiveSessionsPerUser())
		require.NoError(t, err,
			"CreateSession: %v", err)
	}

	err := store.DeleteAllUserSessions(ctx, sessionTestUserID.String())
	require.ErrorIs(t, err, workerpool.ErrQueueFull,
		"DeleteAllUserSessions err = %v, want ErrQueueFull", err)
	require.False(t, err == nil || !strings.Contains(err.Error(), "submit delete user auth sessions purge"),
		"DeleteAllUserSessions err = %v, want submit context", err)

}

func TestSessionStoreDeleteAllUserSessionsPurgeFailureIsObservable(t *testing.T) {
	redisServer := miniredis.RunT(t)
	store := newTestSessionStore(redisServer)
	pool := &recordingPurgeTaskPool{beforeRun: redisServer.Close}
	store.purgePool = pool
	ctx := context.Background()
	{
		err := store.CreateSession(ctx, authdomain.AuthSession{UserID: sessionTestUserID.String(), SessionID: "s-fails", TokenVersion: 1}, time.Hour, defaultMaxActiveSessionsPerUser())
		require.NoError(t, err,
			"CreateSession: %v", err)
	}
	{

		err := store.DeleteAllUserSessions(ctx, sessionTestUserID.String())
		require.NoError(t, err,
			"DeleteAllUserSessions: %v", err)
	}

	require.EqualValues(t, 1, pool.Stats().Failed,
		"Failed stats = %d, want 1", pool.Stats().Failed)
	require.False(t, pool.err == nil || !strings.Contains(pool.err.Error(), "read detached user sessions"),
		"purge err = %v, want read detached user sessions", pool.err)
	require.Equal(t, "auth.redis.purge_detached_user_sessions", pool.taskName,
		"task name = %q, want auth.redis.purge_detached_user_sessions", pool.taskName)

}

func TestSessionStorePurgePoolStopsBeforeRedisClose(t *testing.T) {
	redisServer := miniredis.RunT(t)
	client := rediscache.NewClient(&rediscache.Options{Addr: redisServer.Addr()})
	stopOrder := make([]string, 0, 2)

	pool, err := NewSessionPurgePool(zap.NewNop())
	require.NoError(t, err,
		"NewSessionPurgePool: %v", err)

	store := NewSessionStore(SessionStoreOptions{
		Redis:                client,
		Keys:                 mustKeyCatalog(""),
		TokenVersionCacheTTL: time.Minute,
		PurgePool:            pool,
		Metrics:              authapplication.NopMetrics(),
	})
	require.NotNil(t, store.purgePool,
		"purgePool = nil")

	stopCtx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	stopOrder = append(stopOrder, "purge_pool")
	require.NoError(t, pool.Stop(stopCtx))
	stopOrder = append(stopOrder, "redis")
	require.NoError(t, client.Close())
	require.Equal(t, "purge_pool,redis", strings.Join(stopOrder, ","),
		"stop order = %v, want purge_pool before redis", stopOrder)

}

func TestSessionStoreConsumesPurgePool(t *testing.T) {
	redisServer := miniredis.RunT(t)
	client := rediscache.NewClient(&rediscache.Options{Addr: redisServer.Addr()})
	t.Cleanup(func() { _ = client.Close() })
	pool, err := NewSessionPurgePool(zap.NewNop())
	require.NoError(t, err,
		"NewSessionPurgePool: %v", err)
	store := NewSessionStore(SessionStoreOptions{
		Redis:                client,
		Keys:                 mustKeyCatalog(""),
		TokenVersionCacheTTL: time.Minute,
		PurgePool:            pool,
		Metrics:              authapplication.NopMetrics(),
	})
	stopCtx, cancel := context.WithTimeout(context.Background(), time.Second)
	t.Cleanup(func() {
		defer cancel()
		_ = pool.Stop(stopCtx)
	})
	require.NotNil(t, store,
		"store = nil")
	require.NotNil(t, store.purgePool,
		"purgePool = nil")

}

func TestSessionPurgePoolStopDrainsAndIsIdempotent(t *testing.T) {
	baselineGoroutines := runtime.NumGoroutine()
	pool, err := NewSessionPurgePool(zap.NewNop())
	require.NoError(t, err,
		"NewSessionPurgePool: %v", err)

	started := make(chan struct{})
	release := make(chan struct{})
	var completed atomic.Bool
	err = pool.Submit(context.Background(), workerpool.Task{Name: "drain", Run: func(context.Context) error {
		close(started)
		<-release
		completed.Store(true)
		return nil
	}})
	require.NoError(t, err,
		"Submit: %v", err)
	<-started

	stopDone := make(chan error, 1)
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		stopDone <- pool.Stop(ctx)
	}()
	require.Never(t, func() bool {
		select {
		case <-stopDone:
			return true
		default:
			return false
		}
	}, 50*time.Millisecond, 10*time.Millisecond)

	close(release)
	select {
	case err := <-stopDone:
		require.NoError(t, err)
	case <-time.After(time.Second):
		t.Fatal("session purge pool stop blocked after task release")
	}
	require.True(t, completed.Load(),
		"task was not drained before Stop returned")
	require.NoError(t, pool.Stop(context.Background()))
	require.True(t, pool.Stats().Closed,
		"pool was not marked closed")
	require.Zero(t, pool.Stats().Running,
		"pool still has running tasks")
	require.Eventually(t, func() bool {
		return runtime.NumGoroutine() <= baselineGoroutines+2
	}, time.Second, 10*time.Millisecond, "purge pool goroutines did not settle")
}

func TestSessionPurgePoolStopRespectsCallerTimeout(t *testing.T) {
	pool, err := NewSessionPurgePool(zap.NewNop())
	require.NoError(t, err,
		"NewSessionPurgePool: %v", err)

	started := make(chan struct{})
	err = pool.Submit(context.Background(), workerpool.Task{Name: "timeout", Run: func(ctx context.Context) error {
		close(started)
		<-ctx.Done()
		return nil
	}})
	require.NoError(t, err,
		"Submit: %v", err)
	<-started

	stopCtx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()
	stopDone := make(chan error, 1)
	go func() { stopDone <- pool.Stop(stopCtx) }()
	select {
	case err = <-stopDone:
	case <-time.After(time.Second):
		t.Fatal("session purge pool stop did not honor caller timeout")
	}
	require.ErrorIs(t, err, context.DeadlineExceeded,
		"Stop err = %v, want DeadlineExceeded", err)
	require.Eventually(t, func() bool {
		return pool.Stats().Closed && pool.Stats().Running == 0
	}, time.Second, 10*time.Millisecond, "pool did not settle after timeout")
	require.NoError(t, pool.Stop(context.Background()))
}

func TestSessionPurgePoolStopTimeoutMatchesRuntimeWorkerDrainAllowance(t *testing.T) {
	require.Equal(t, runtimeconfig.DefaultLifecycleWorkerDrainAllowance, deleteAllUserSessionsPurgeStopTimeout)
}
