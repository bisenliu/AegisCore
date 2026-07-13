package redis

import (
	"context"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/stretchr/testify/require"

	serviceconfig "github.com/aegiscore/user-service/internal/config"
	authdomain "github.com/aegiscore/user-service/internal/features/auth/domain"
)

func TestSessionStoreTokenVersionCacheMiss(t *testing.T) {
	redisServer := miniredis.RunT(t)
	store := newTestSessionStore(redisServer)

	_, err := store.GetCachedTokenVersion(context.Background(), sessionTestUserID.String())
	require.ErrorIs(t, err, authdomain.ErrTokenVersionCacheMiss,
		"GetCachedTokenVersion err = %v, want cache miss", err)
	require.False(t, redisServer.Exists(store.tokenVersionKey(sessionTestUserID.String())),
		"cache miss should not create token version key")

}

func TestSessionStoreCachesAndGetsTokenVersion(t *testing.T) {
	redisServer := miniredis.RunT(t)
	store := newTestSessionStore(redisServer)
	{

		err := store.CacheTokenVersion(context.Background(), sessionTestUserID.String(), 7)
		require.NoError(t, err,
			"CacheTokenVersion: %v", err)
	}

	version, err := store.GetCachedTokenVersion(context.Background(), sessionTestUserID.String())
	require.NoError(t, err,
		"GetCachedTokenVersion: %v", err)
	require.EqualValues(t, 7, version,
		"version = %d, want 7", version)

	got, err := redisServer.Get(store.tokenVersionKey(sessionTestUserID.String()))
	require.NoError(t, err,
		"Get cached token version: %v", err)
	require.Equal(t, "7", got,
		"cached token version = %q, want 7", got)

}

func TestSessionStoreCacheTokenVersionOverwritesStaleValue(t *testing.T) {
	redisServer := miniredis.RunT(t)
	store := newTestSessionStore(redisServer)
	ctx := context.Background()
	{

		err := store.CacheTokenVersion(ctx, sessionTestUserID.String(), 7)
		require.NoError(t, err,
			"CacheTokenVersion old: %v", err)
	}
	{

		err := store.CacheTokenVersion(ctx, sessionTestUserID.String(), 8)
		require.NoError(t, err,
			"CacheTokenVersion new: %v", err)
	}

	version, err := store.GetCachedTokenVersion(ctx, sessionTestUserID.String())
	require.NoError(t, err,
		"GetCachedTokenVersion: %v", err)
	require.EqualValues(t, 8, version,
		"version = %d, want 8", version)

	ttl, err := store.redis.TTL(ctx, store.tokenVersionKey(sessionTestUserID.String())).Result()
	require.NoError(t, err,
		"TTL: %v", err)
	require.False(t, ttl <= 0 || ttl > time.Minute,
		"TTL = %s, want within explicit %s", ttl, time.Minute)

}

func TestSessionStoreCacheTokenVersionDoesNotOverwriteNewerValue(t *testing.T) {
	redisServer := miniredis.RunT(t)
	store := newTestSessionStore(redisServer)
	ctx := context.Background()
	{

		err := store.CacheTokenVersion(ctx, sessionTestUserID.String(), 9)
		require.NoError(t, err,
			"CacheTokenVersion new: %v", err)
	}
	{

		err := store.CacheTokenVersion(ctx, sessionTestUserID.String(), 8)
		require.NoError(t, err,
			"CacheTokenVersion stale: %v", err)
	}

	version, err := store.GetCachedTokenVersion(ctx, sessionTestUserID.String())
	require.NoError(t, err,
		"GetCachedTokenVersion: %v", err)
	require.EqualValues(t, 9, version,
		"version = %d, want 9", version)

}

func TestSessionStoreDeleteCachedTokenVersion(t *testing.T) {
	redisServer := miniredis.RunT(t)
	store := newTestSessionStore(redisServer)
	ctx := context.Background()
	{

		err := store.CacheTokenVersion(ctx, sessionTestUserID.String(), 7)
		require.NoError(t, err,
			"CacheTokenVersion: %v", err)
	}
	{

		err := store.DeleteCachedTokenVersion(ctx, sessionTestUserID.String())
		require.NoError(t, err,
			"DeleteCachedTokenVersion: %v", err)
	}

	_, err := store.GetCachedTokenVersion(ctx, sessionTestUserID.String())
	require.ErrorIs(t, err, authdomain.ErrTokenVersionCacheMiss,
		"GetCachedTokenVersion err = %v, want cache miss", err)
	require.False(t, redisServer.Exists(store.tokenVersionKey(sessionTestUserID.String())),
		"token version cache key still exists")
	require.Equal(t, "auth:user:token_version:{"+sessionTestUserID.String()+"}", store.tokenVersionKey(sessionTestUserID.String()),
		"token version key changed: %q", store.tokenVersionKey(sessionTestUserID.String()))

}

func TestSessionStoreTokenVersionCacheUsesDefaultTTL(t *testing.T) {
	for _, tc := range []struct {
		name string
		ttl  time.Duration
	}{
		{name: "zero", ttl: 0},
		{name: "negative", ttl: -time.Second},
	} {
		t.Run(tc.name, func(t *testing.T) {
			redisServer := miniredis.RunT(t)
			store := newTestSessionStoreWithConfig(redisServer, serviceconfig.AuthConfig{TokenVersionCacheTTL: tc.ttl})

			err := store.CacheTokenVersion(context.Background(), sessionTestUserID.String(), 7)
			require.NoError(t, err,
				"CacheTokenVersion: %v", err)

			ttl, err := store.redis.TTL(context.Background(), store.tokenVersionKey(sessionTestUserID.String())).Result()
			require.NoError(t, err,
				"TTL: %v", err)
			require.False(t, ttl <= 0 || ttl > defaultTokenVersionCacheTTL,
				"TTL = %s, want within default %s", ttl, defaultTokenVersionCacheTTL)

		})
	}
}

func TestSessionStoreTokenVersionCacheUsesExplicitTTL(t *testing.T) {
	redisServer := miniredis.RunT(t)
	explicitTTL := time.Minute
	store := newTestSessionStoreWithConfig(redisServer, serviceconfig.AuthConfig{TokenVersionCacheTTL: explicitTTL})

	err := store.CacheTokenVersion(context.Background(), sessionTestUserID.String(), 7)
	require.NoError(t, err,
		"CacheTokenVersion: %v", err)

	ttl, err := store.redis.TTL(context.Background(), store.tokenVersionKey(sessionTestUserID.String())).Result()
	require.NoError(t, err,
		"TTL: %v", err)
	require.False(t, ttl <= 0 || ttl > explicitTTL,
		"TTL = %s, want within explicit %s", ttl, explicitTTL)

}

func TestSessionStoreTokenVersionInvalidCacheReportsMiss(t *testing.T) {
	redisServer := miniredis.RunT(t)
	store := newTestSessionStore(redisServer)
	ctx := context.Background()
	key := store.tokenVersionKey(sessionTestUserID.String())

	for _, value := range []string{"not-an-int", "0"} {
		{
			err := store.redis.Set(ctx, key, value, time.Minute).Err()
			require.NoError(t, err,
				"Set token version cache: %v", err)
		}

		_, err := store.GetCachedTokenVersion(ctx, sessionTestUserID.String())
		require.ErrorIs(t, err, authdomain.ErrTokenVersionCacheMiss,
			"GetCachedTokenVersion(%q) err = %v, want cache miss", value, err)

	}
}
