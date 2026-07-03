package scheduler

import (
	"context"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	redis "github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/require"
)

func TestRedisLockerAcquireUnlockAndReacquire(t *testing.T) {
	server, client := newMiniRedisClient(t)
	locker := newTestRedisLocker(t, client)

	lock, ok, err := locker.Acquire(context.Background(), "nightly", time.Minute, 0)
	require.NoError(t, err)
	require.True(t, ok)
	require.True(t, server.Exists("aegiscore:cron:nightly"), "lock key was not created")

	secondLock, ok, err := locker.Acquire(context.Background(), "nightly", time.Minute, 0)
	require.NoError(t, err)
	require.False(t, ok)
	require.Nil(t, secondLock)

	require.NoError(t, lock.Unlock(context.Background()))
	require.False(t, server.Exists("aegiscore:cron:nightly"), "lock key still exists after unlock")

	_, ok, err = locker.Acquire(context.Background(), "nightly", time.Minute, 0)
	require.NoError(t, err)
	require.True(t, ok)
}

func TestRedisLockUnlockRejectsLostOwnership(t *testing.T) {
	_, client := newMiniRedisClient(t)
	locker := newTestRedisLocker(t, client)

	lock, ok, err := locker.Acquire(context.Background(), "nightly", time.Minute, 0)
	require.NoError(t, err)
	require.True(t, ok)
	require.NoError(t, client.Set(context.Background(), "aegiscore:cron:nightly", "other-owner", time.Minute).Err())

	err = lock.Unlock(context.Background())
	require.ErrorIs(t, err, ErrLockNotOwned)
}

func TestRedisLockRenewExtendsTTL(t *testing.T) {
	server, client := newMiniRedisClient(t)
	locker := newTestRedisLocker(t, client)

	lock, ok, err := locker.Acquire(context.Background(), "nightly", 2*time.Second, 0)
	require.NoError(t, err)
	require.True(t, ok)

	server.FastForward(1500 * time.Millisecond)
	require.NoError(t, lock.Renew(context.Background(), 5*time.Second))

	ttl := server.TTL("aegiscore:cron:nightly")
	require.Greater(t, ttl, 3*time.Second)
}

func TestRedisLockerWaitTimeoutReturnsBusy(t *testing.T) {
	_, client := newMiniRedisClient(t)
	locker := newTestRedisLocker(t, client)

	firstLock, ok, err := locker.Acquire(context.Background(), "nightly", time.Minute, 0)
	require.NoError(t, err)
	require.True(t, ok)
	defer func() {
		_ = firstLock.Unlock(context.Background())
	}()

	startedAt := time.Now()
	lock, ok, err := locker.Acquire(context.Background(), "nightly", time.Minute, 25*time.Millisecond)
	require.NoError(t, err)
	require.False(t, ok)
	require.Nil(t, lock)
	require.GreaterOrEqual(t, time.Since(startedAt), 20*time.Millisecond)
}

func TestRedisLockerMaxAttemptsLimitsRetries(t *testing.T) {
	_, client := newMiniRedisClient(t)
	locker, err := NewRedisLocker(client, RedisLockerOptions{
		Namespace: "aegiscore",
		Scope:     []string{"cron"},
		Retry: RetryPolicy{
			InitialInterval: time.Millisecond,
			MaxInterval:     time.Millisecond,
			MaxAttempts:     1,
		},
	})
	require.NoError(t, err)

	firstLock, ok, err := locker.Acquire(context.Background(), "nightly", time.Minute, 0)
	require.NoError(t, err)
	require.True(t, ok)
	defer func() {
		_ = firstLock.Unlock(context.Background())
	}()

	startedAt := time.Now()
	lock, ok, err := locker.Acquire(context.Background(), "nightly", time.Minute, time.Second)
	require.NoError(t, err)
	require.False(t, ok)
	require.Nil(t, lock)
	require.LessOrEqual(t, time.Since(startedAt), 100*time.Millisecond)
}

func TestRetryPolicyDefaultsAndBackoff(t *testing.T) {
	policy, err := normalizeRetryPolicy(RetryPolicy{})
	require.NoError(t, err)
	require.Equal(t, 50*time.Millisecond, policy.InitialInterval)
	require.Equal(t, time.Second, policy.MaxInterval)

	delay := policy.InitialInterval
	delay = nextRetryDelay(delay, policy.MaxInterval)
	require.Equal(t, 100*time.Millisecond, delay)
	delay = nextRetryDelay(800*time.Millisecond, policy.MaxInterval)
	require.Equal(t, time.Second, delay)
	delay = nextRetryDelay(time.Second, policy.MaxInterval)
	require.Equal(t, time.Second, delay)
}

func TestRetryPolicyRejectsInvalidIntervals(t *testing.T) {
	_, err := normalizeRetryPolicy(RetryPolicy{
		InitialInterval: time.Second,
		MaxInterval:     time.Millisecond,
	})
	require.ErrorIs(t, err, ErrInvalidLock)
}

func newMiniRedisClient(t *testing.T) (*miniredis.Miniredis, *redis.Client) {
	t.Helper()
	server := miniredis.RunT(t)
	client := redis.NewClient(&redis.Options{Addr: server.Addr()})
	t.Cleanup(func() {
		require.NoError(t, client.Close())
	})
	return server, client
}

func newTestRedisLocker(t *testing.T, client redis.UniversalClient) *RedisLocker {
	t.Helper()
	locker, err := NewRedisLocker(client, RedisLockerOptions{
		Namespace: "aegiscore",
		Scope:     []string{"cron"},
		Retry: RetryPolicy{
			InitialInterval: 5 * time.Millisecond,
			MaxInterval:     5 * time.Millisecond,
		},
	})
	require.NoError(t, err)
	return locker
}
