package redis

import (
	"context"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/google/uuid"
	rediscache "github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/require"

	"github.com/aegiscore/common/runtime/localcache"
	"github.com/aegiscore/common/runtime/workerpool"
	commonauth "github.com/aegiscore/common/security/auth"
	serviceconfig "github.com/aegiscore/user-service/internal/config"
	authapplication "github.com/aegiscore/user-service/internal/features/auth/application"
	authvalidators "github.com/aegiscore/user-service/internal/features/auth/application/validators"
)

var sessionTestUserID = uuid.MustParse("018f6f3e-7c4d-7b2a-9f8a-4f6b1b2c3d4e")

func newTestSessionStore(redisServer *miniredis.Miniredis) *SessionStore {
	return newTestSessionStoreWithConfig(redisServer, serviceconfig.AuthConfig{TokenVersionCacheTTL: time.Minute})
}

func newTestSessionStoreWithConfig(redisServer *miniredis.Miniredis, authCfg serviceconfig.AuthConfig) *SessionStore {
	client := rediscache.NewClient(&rediscache.Options{Addr: redisServer.Addr()})
	return &SessionStore{redis: client, keys: mustKeyCatalog(""), tokenVersionCacheTTL: authCfg.TokenVersionCacheTTL, purgePool: directPurgeTaskPool{}, metrics: authapplication.NopMetrics()}
}

func defaultMaxActiveSessionsPerUser() int {
	return 5
}

func newTestTokenVersionValidator(t testing.TB, users authapplication.UserTokenVersionStore, tokenCache authapplication.TokenVersionCache) commonauth.TokenVersionValidator {
	t.Helper()
	cache, err := localcache.NewLoadingCache[string, int64](localcache.Config{
		Name:        "auth_token_version_test",
		Capacity:    100,
		TTL:         time.Minute,
		LoadTimeout: time.Second,
	}, func(ctx context.Context, userID string) (int64, error) {
		return authvalidators.Current(ctx, users, tokenCache, userID)
	})
	require.NoError(t, err,
		"New localcache: %v", err)

	t.Cleanup(cache.Close)
	return authvalidators.NewCachingValidator(cache)
}

func newTestSessionStoreWithAppName(t testing.TB, redisServer *miniredis.Miniredis, appName string) *SessionStore {
	t.Helper()
	client := rediscache.NewClient(&rediscache.Options{Addr: redisServer.Addr()})
	keys, err := NewKeyCatalog(appName)
	require.NoError(t, err,
		"NewKeyCatalog: %v", err)
	store := NewSessionStore(SessionStoreOptions{
		Redis:                client,
		Keys:                 keys,
		TokenVersionCacheTTL: time.Minute,
		PurgePool:            directPurgeTaskPool{},
		Metrics:              authapplication.NopMetrics(),
	})

	t.Cleanup(func() {
		_ = client.Close()
	})
	return store
}

func waitForRedisCondition(t *testing.T, condition func() bool, message string) {
	t.Helper()
	require.Eventually(t, condition, 2*time.Second, 10*time.Millisecond, message)
}

type rejectingPurgeTaskPool struct {
	err error
}

func (p rejectingPurgeTaskPool) Submit(context.Context, workerpool.Task) error {
	return p.err
}

func (p rejectingPurgeTaskPool) Stats() workerpool.Stats {
	return workerpool.Stats{}
}

type directPurgeTaskPool struct{}

func (directPurgeTaskPool) Submit(ctx context.Context, task workerpool.Task) error {
	if task.Run == nil {
		return workerpool.ErrInvalidTask
	}
	if ctx == nil {
		ctx = context.Background()
	}
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}
	return task.Run(ctx)
}

func (directPurgeTaskPool) Stats() workerpool.Stats {
	return workerpool.Stats{}
}

type recordingPurgeTaskPool struct {
	beforeRun func()
	taskName  string
	err       error
	failed    int64
}

func (p *recordingPurgeTaskPool) Submit(ctx context.Context, task workerpool.Task) error {
	p.taskName = task.Name
	if p.beforeRun != nil {
		p.beforeRun()
	}
	p.err = task.Run(ctx)
	if p.err != nil {
		p.failed++
	}
	return nil
}

func (p *recordingPurgeTaskPool) Stats() workerpool.Stats {
	return workerpool.Stats{Failed: p.failed}
}
