package redis

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/stretchr/testify/require"

	authdomain "github.com/aegiscore/user-service/internal/features/auth/domain"
)

func TestSessionStorePurgeUserSessionsKeyKeepsHashTag(t *testing.T) {
	redisServer := miniredis.RunT(t)
	store := newTestSessionStore(redisServer)

	purgeKey, err := store.purgeUserSessionsKey(sessionTestUserID)
	require.NoError(t, err,
		"purgeUserSessionsKey: %v", err)
	require.True(t, strings.HasPrefix(purgeKey, "auth:user:sessions:{"+sessionTestUserID.String()+"}:purge:"),
		"purge key = %q, want unprefixed purge key prefix", purgeKey)
	require.True(t, strings.Contains(purgeKey, "{"+sessionTestUserID.String()+"}"),
		"purge key = %q, want user hash tag", purgeKey)

}

func TestSessionStorePurgeUserSessionsKeyUsesAppNamePrefix(t *testing.T) {
	redisServer := miniredis.RunT(t)
	store := newTestSessionStoreWithAppName(t, redisServer, " aegiscore-user-service ")

	purgeKey, err := store.purgeUserSessionsKey(sessionTestUserID)
	require.NoError(t, err,
		"purgeUserSessionsKey: %v", err)
	require.True(t, strings.HasPrefix(purgeKey, "aegiscore-user-service:auth:user:sessions:{"+sessionTestUserID.String()+"}:purge:"),
		"purge key = %q, want app-name-prefixed purge key", purgeKey)
	require.True(t, strings.Contains(purgeKey, "{"+sessionTestUserID.String()+"}"),
		"purge key = %q, want user hash tag", purgeKey)

}

func TestSessionStoreUserSessionsIndexTTLIsNotShortened(t *testing.T) {
	redisServer := miniredis.RunT(t)
	store := newTestSessionStore(redisServer)
	ctx := context.Background()
	indexKey := store.userSessionsKey(sessionTestUserID)
	longTTL := 2 * time.Hour
	shortTTL := time.Hour
	{

		err := store.CreateSession(ctx, authdomain.AuthSession{UserID: sessionTestUserID, SessionID: "long", TokenVersion: 1}, longTTL, defaultMaxActiveSessionsPerUser())
		require.NoError(t, err,
			"CreateSession(long): %v", err)
	}

	longIndexTTL, err := store.redis.TTL(ctx, indexKey).Result()
	require.NoError(t, err,
		"long index TTL: %v", err)
	require.False(t, longIndexTTL <= longTTL || longIndexTTL > longTTL+authSessionIndexTTLBuffer,
		"long index TTL = %s, want between session ttl and %s", longIndexTTL, longTTL+authSessionIndexTTLBuffer)
	{

		err := store.CreateSession(ctx, authdomain.AuthSession{UserID: sessionTestUserID, SessionID: "short", TokenVersion: 1}, shortTTL, defaultMaxActiveSessionsPerUser())
		require.NoError(t, err,
			"CreateSession(short): %v", err)
	}

	afterShortIndexTTL, err := store.redis.TTL(ctx, indexKey).Result()
	require.NoError(t, err,
		"after short index TTL: %v", err)
	require.False(t, afterShortIndexTTL <= shortTTL+authSessionIndexTTLBuffer,
		"index TTL was shortened to %s after short session", afterShortIndexTTL)
	require.False(t, afterShortIndexTTL > longTTL+authSessionIndexTTLBuffer,
		"index TTL = %s, want at most %s", afterShortIndexTTL, longTTL+authSessionIndexTTLBuffer)

}

func TestSessionStoreKeysUseAppNamePrefixWithNewFormat(t *testing.T) {
	redisServer := miniredis.RunT(t)
	store := newTestSessionStoreWithAppName(t, redisServer, " aegiscore-user-service ")
	ctx := context.Background()
	session := authdomain.AuthSession{UserID: sessionTestUserID, SessionID: "s-prefixed", TokenVersion: 7, ExpiresAt: time.Now().Add(time.Hour)}
	{

		err := store.CreateSession(ctx, session, time.Hour, defaultMaxActiveSessionsPerUser())
		require.NoError(t, err,
			"CreateSession: %v", err)
	}
	{

		err := store.CacheTokenVersion(ctx, sessionTestUserID, 7)
		require.NoError(t, err,
			"CacheTokenVersion: %v", err)
	}

	require.True(t, redisServer.Exists("aegiscore-user-service:auth:session:{"+sessionTestUserID.String()+"}:s-prefixed"),
		"prefixed new session key does not exist")
	require.True(t, redisServer.Exists("aegiscore-user-service:auth:user:sessions:{"+sessionTestUserID.String()+"}"),
		"prefixed new user sessions key does not exist")
	require.True(t, redisServer.Exists("aegiscore-user-service:auth:user:token_version:{"+sessionTestUserID.String()+"}"),
		"prefixed new token version key does not exist")
	require.False(t, redisServer.Exists("auth:session:{"+sessionTestUserID.String()+"}:s-prefixed") || redisServer.Exists("auth:user:sessions:{"+sessionTestUserID.String()+"}") || redisServer.Exists("auth:user:token_version:{"+sessionTestUserID.String()+"}"),
		"unprefixed Redis keys should not exist when app.name is set")

}

func TestSessionStoreKeysRemainUnprefixedWhenAppNameEmpty(t *testing.T) {
	redisServer := miniredis.RunT(t)
	store := newTestSessionStoreWithAppName(t, redisServer, "   ")
	ctx := context.Background()
	session := authdomain.AuthSession{UserID: sessionTestUserID, SessionID: "s-empty-prefix", TokenVersion: 7, ExpiresAt: time.Now().Add(time.Hour)}
	{

		err := store.CreateSession(ctx, session, time.Hour, defaultMaxActiveSessionsPerUser())
		require.NoError(t, err,
			"CreateSession: %v", err)
	}
	{

		err := store.CacheTokenVersion(ctx, sessionTestUserID, 7)
		require.NoError(t, err,
			"CacheTokenVersion: %v", err)
	}

	require.True(t, redisServer.Exists("auth:session:{"+sessionTestUserID.String()+"}:s-empty-prefix"),
		"unprefixed new session key does not exist")
	require.True(t, redisServer.Exists("auth:user:sessions:{"+sessionTestUserID.String()+"}"),
		"unprefixed new user sessions key does not exist")
	require.True(t, redisServer.Exists("auth:user:token_version:{"+sessionTestUserID.String()+"}"),
		"unprefixed new token version key does not exist")
	require.False(t, redisServer.Exists("aegiscore-user-service:auth:session:{"+sessionTestUserID.String()+"}:s-empty-prefix") || redisServer.Exists("aegiscore-user-service:auth:user:sessions:{"+sessionTestUserID.String()+"}") || redisServer.Exists("aegiscore-user-service:auth:user:token_version:{"+sessionTestUserID.String()+"}"),
		"default service-name Redis keys should not exist when app.name is empty")

}

func TestSessionStoreIgnoresLegacyKeys(t *testing.T) {
	redisServer := miniredis.RunT(t)
	store := newTestSessionStore(redisServer)
	ctx := context.Background()
	legacySession := authdomain.AuthSession{UserID: sessionTestUserID, SessionID: "s-legacy", TokenVersion: 7, ExpiresAt: time.Now().Add(time.Hour)}
	data, err := json.Marshal(newAuthSessionPayload(legacySession))
	require.NoError(t, err,
		"Marshal legacy session: %v", err)
	{

		err := store.redis.Set(ctx, "auth:session:s-legacy", data, time.Hour).Err()
		require.NoError(t, err,
			"Set legacy session: %v", err)
	}
	{

		err := store.redis.Set(ctx, "auth:user:"+sessionTestUserID.String()+":token_version", "7", time.Hour).Err()
		require.NoError(t, err,
			"Set legacy token version: %v", err)
	}

	_, err = store.GetSession(ctx, sessionTestUserID, "s-legacy")
	require.ErrorIs(t, err, authdomain.ErrAuthSessionNotFound,
		"GetSession err = %v, want session not found", err)

	_, err = store.GetCachedTokenVersion(ctx, sessionTestUserID)
	require.ErrorIs(t, err, authdomain.ErrTokenVersionCacheMiss,
		"GetCachedTokenVersion err = %v, want cache miss", err)

}
