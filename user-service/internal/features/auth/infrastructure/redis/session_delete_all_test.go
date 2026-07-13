package redis

import (
	"context"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	rediscache "github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/require"
	"go.uber.org/fx"
	"go.uber.org/fx/fxtest"
	"go.uber.org/mock/gomock"
	"go.uber.org/zap"

	"github.com/aegiscore/common/runtime/workerpool"
	serviceconfig "github.com/aegiscore/user-service/internal/config"
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

func TestSessionStorePurgePoolStopHookPrecedesRedisStopHook(t *testing.T) {
	redisServer := miniredis.RunT(t)
	client := rediscache.NewClient(&rediscache.Options{Addr: redisServer.Addr()})
	lifecycle := &lifecycleRecorder{}
	stopOrder := make([]string, 0, 2)
	lifecycle.Append(fx.Hook{OnStop: func(context.Context) error {
		stopOrder = append(stopOrder, "redis")
		return client.Close()
	}})

	pool, err := NewSessionPurgePool(SessionPurgePoolParams{
		Lifecycle: lifecycle,
		Redis:     client,
		Log:       zap.NewNop(),
	})
	require.NoError(t, err,
		"NewSessionPurgePool: %v", err)

	store, err := NewSessionStore(SessionStoreParams{
		Redis:     client,
		Cfg:       &serviceconfig.Config{},
		PurgePool: pool,
	})
	require.NoError(t, err,
		"NewSessionStore: %v", err)
	require.NotNil(t, store.purgePool,
		"purgePool = nil")
	require.EqualValues(t, 2, len(lifecycle.hooks),
		"lifecycle hooks = %d, want redis and purge pool hooks", len(lifecycle.hooks))

	purgeStop := lifecycle.hooks[1].OnStop
	lifecycle.hooks[1].OnStop = func(ctx context.Context) error {
		stopOrder = append(stopOrder, "purge_pool")
		return purgeStop(ctx)
	}

	stopCtx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	for i := len(lifecycle.hooks) - 1; i >= 0; i-- {
		hook := lifecycle.hooks[i]
		if hook.OnStop == nil {
			continue
		}
		{
			err := hook.OnStop(stopCtx)
			require.NoError(t, err,
				"OnStop hook %d: %v", i, err)
		}

	}
	require.Equal(t, "purge_pool,redis", strings.Join(stopOrder, ","),
		"stop order = %v, want purge_pool before redis", stopOrder)

}

func TestSessionStoreConsumesNamedPurgePool(t *testing.T) {
	redisServer := miniredis.RunT(t)
	client := rediscache.NewClient(&rediscache.Options{Addr: redisServer.Addr()})
	var store *SessionStore
	app := fxtest.New(t,
		fx.Provide(
			func() *serviceconfig.Config {
				return &serviceconfig.Config{Auth: serviceconfig.AuthConfig{TokenVersionCacheTTL: time.Minute}}
			},
			func() *zap.Logger {
				return zap.NewNop()
			},
			fx.Annotate(
				func() *rediscache.Client {
					return client
				},
				fx.ResultTags(`name:"cache_redis"`),
			),
			fx.Annotate(
				NewSessionPurgePool,
				fx.As(new(PurgeTaskPool)),
				fx.ResultTags(`name:"auth_session_purge_pool"`),
			),
			NewSessionStore,
		),
		fx.Populate(&store),
	)
	app.RequireStart()
	app.RequireStop()
	_ = client.Close()
	require.NotNil(t, store,
		"store = nil")
	require.NotNil(t, store.purgePool,
		"purgePool = nil")

}
