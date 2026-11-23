package redis

import (
	"context"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/stretchr/testify/require"
)

func TestLoginLockoutStoreLocksAndClears(t *testing.T) {
	redisServer := miniredis.RunT(t)
	store := newTestSessionStore(redisServer)
	ctx := context.Background()

	for range 4 {
		status, err := store.RegisterLoginFailure(ctx, " Alice ", 5, 15*time.Minute, 15*time.Minute)
		require.NoError(t, err)
		require.False(t, status.Locked)
	}
	status, err := store.RegisterLoginFailure(ctx, "alice", 5, 15*time.Minute, 15*time.Minute)
	require.NoError(t, err)
	require.True(t, status.Locked)

	status, err = store.LoginLockoutStatus(ctx, "ALICE")
	require.NoError(t, err)
	require.True(t, status.Locked)

	require.NoError(t, store.ClearLoginFailures(ctx, "alice"))
	status, err = store.LoginLockoutStatus(ctx, "alice")
	require.NoError(t, err)
	require.False(t, status.Locked)
}

func TestLoginLockoutStorePreservesTTLAcrossFailures(t *testing.T) {
	redisServer := miniredis.RunT(t)
	store := newTestSessionStore(redisServer)
	ctx := context.Background()

	status, err := store.RegisterLoginFailure(ctx, "bob", 5, 15*time.Minute, 30*time.Minute)
	require.NoError(t, err)
	require.False(t, status.Locked)

	key := store.loginLockoutKey("bob")
	ttl1 := redisServer.TTL(key)
	require.True(t, ttl1 > 0 && ttl1 <= 15*time.Minute)

	// 经过 2 分钟后再触发失败，应保留剩余约 13 分钟的窗口，不应抹除 TTL 也不应延长窗口
	redisServer.FastForward(2 * time.Minute)

	status, err = store.RegisterLoginFailure(ctx, "bob", 5, 15*time.Minute, 30*time.Minute)
	require.NoError(t, err)
	require.False(t, status.Locked)

	ttl2 := redisServer.TTL(key)
	require.True(t, ttl2 > 0 && ttl2 <= 13*time.Minute+time.Second, "ttl2=%v should be preserved", ttl2)
}

func TestLoginLockoutStoreExpiresWithoutCleanup(t *testing.T) {
	redisServer := miniredis.RunT(t)
	store := newTestSessionStore(redisServer)
	ctx := context.Background()

	status, err := store.RegisterLoginFailure(ctx, "alice", 1, time.Minute, time.Minute)
	require.NoError(t, err)
	require.True(t, status.Locked)

	redisServer.FastForward(time.Minute + time.Second)
	status, err = store.LoginLockoutStatus(ctx, "alice")
	require.NoError(t, err)
	require.False(t, status.Locked)
}

func TestLoginLockoutStoreFailsClosedWhenRedisClosed(t *testing.T) {
	redisServer := miniredis.RunT(t)
	store := newTestSessionStore(redisServer)
	redisServer.Close()

	_, err := store.LoginLockoutStatus(context.Background(), "alice")
	require.Error(t, err)
}
