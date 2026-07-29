package redis

import (
	"context"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	rediscache "github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/require"

	authdomain "github.com/aegiscore/user-service/internal/features/auth/domain"
)

func TestSessionStoreCreateGetAndDeleteSession(t *testing.T) {
	redisServer := miniredis.RunT(t)
	store := newTestSessionStore(redisServer)
	ctx := context.Background()
	indexKey := store.userSessionsKey(sessionTestUserID)
	ttl := time.Hour
	mismatchedExpiresAt := time.Now().Add(24 * time.Hour).Truncate(time.Second)
	session := authdomain.AuthSession{UserID: sessionTestUserID, SessionID: "s-123", TokenVersion: 1, ExpiresAt: mismatchedExpiresAt}
	{
		err := store.redis.ZAdd(ctx, indexKey, rediscache.Z{Score: float64(time.Now().Add(-time.Minute).Unix()), Member: "expired-session"}).Err()
		require.NoError(t, err,
			"ZAdd expired session: %v", err)
	}

	beforeCreate := time.Now()
	{
		err := store.CreateSession(ctx, session, ttl, defaultMaxActiveSessionsPerUser())
		require.NoError(t, err,
			"CreateSession: %v", err)
	}

	afterCreate := time.Now()
	stored, err := store.GetSession(ctx, sessionTestUserID, "s-123")
	require.NoError(t, err,
		"GetSession: %v", err)
	require.False(t, stored.UserID != sessionTestUserID || stored.SessionID != "s-123" || stored.TokenVersion != 1,
		"stored = %#v", stored)
	require.False(t, stored.ExpiresAt.Before(beforeCreate.Add(ttl)) || stored.ExpiresAt.After(afterCreate.Add(ttl)),
		"stored ExpiresAt = %s, want derived from ttl %s", stored.ExpiresAt, ttl)
	require.NotEqual(t, mismatchedExpiresAt.Unix(), stored.ExpiresAt.Unix(),
		"stored ExpiresAt used caller-provided mismatched value %s", mismatchedExpiresAt)

	score, err := store.redis.ZScore(ctx, indexKey, "s-123").Result()
	require.NoError(t, err,
		"ZScore: %v", err)
	require.Equal(t, stored.ExpiresAt.Unix(), int64(score),
		"ZScore = %d, want %d", int64(score), stored.ExpiresAt.Unix())

	sessionTTL, err := store.redis.TTL(ctx, store.sessionKey(sessionTestUserID, "s-123")).Result()
	require.NoError(t, err,
		"session TTL: %v", err)
	require.False(t, sessionTTL <= 0 || sessionTTL > ttl,
		"session TTL = %s, want within %s", sessionTTL, ttl)

	indexTTL, err := store.redis.TTL(ctx, indexKey).Result()
	require.NoError(t, err,
		"index TTL: %v", err)
	require.False(t, indexTTL <= ttl || indexTTL > ttl+authSessionIndexTTLBuffer,
		"index TTL = %s, want between session ttl and %s", indexTTL, ttl+authSessionIndexTTLBuffer)
	{

		_, err := store.redis.ZScore(ctx, indexKey, "expired-session").Result()
		require.ErrorIs(t, err, rediscache.Nil,
			"expired session ZScore err = %v, want redis.Nil", err)
	}

	typ, err := store.redis.Type(ctx, indexKey).Result()
	require.NoError(t, err,
		"Type: %v", err)
	require.Equal(t, "zset", typ,
		"session index type = %q, want zset", typ)
	{

		err := store.redis.ZAdd(ctx, indexKey, rediscache.Z{Score: float64(time.Now().Add(-time.Minute).Unix()), Member: "expired-on-delete"}).Err()
		require.NoError(t, err,
			"ZAdd expired-on-delete: %v", err)
	}
	{

		err := store.DeleteSession(ctx, sessionTestUserID, "s-123")
		require.NoError(t, err,
			"DeleteSession: %v", err)
	}

	require.False(t, redisServer.Exists(store.sessionKey(sessionTestUserID, "s-123")),
		"session key still exists")
	{

		_, err := store.redis.ZScore(ctx, indexKey, "s-123").Result()
		require.ErrorIs(t, err, rediscache.Nil,
			"deleted session ZScore err = %v, want redis.Nil", err)
	}
	{

		_, err := store.redis.ZScore(ctx, indexKey, "expired-on-delete").Result()
		require.ErrorIs(t, err, rediscache.Nil,
			"expired-on-delete ZScore err = %v, want redis.Nil", err)
	}

}

func TestSessionStoreCreateSessionPrunesOldestWhenLimitExceeded(t *testing.T) {
	redisServer := miniredis.RunT(t)
	store := newTestSessionStore(redisServer)
	ctx := context.Background()
	limit := 5
	baseScoreTime := time.Now().Add(time.Hour)

	for i := 0; i < 6; i++ {
		sessionID := "s-" + strconv.Itoa(i)
		{
			err := store.CreateSession(ctx, authdomain.AuthSession{UserID: sessionTestUserID, SessionID: sessionID, TokenVersion: 1}, time.Hour, limit)
			require.NoError(t, err,
				"CreateSession(%s): %v", sessionID, err)
		}
		if i < limit {
			score := redisScoreFloat(baseScoreTime.Add(time.Duration(i-limit) * time.Second))
			err := store.redis.ZAdd(ctx, store.userSessionsKey(sessionTestUserID), rediscache.Z{Score: score, Member: sessionID}).Err()
			require.NoError(t, err,
				"ZAdd deterministic score for %s: %v", sessionID, err)
		}
	}

	members, err := store.redis.ZRange(ctx, store.userSessionsKey(sessionTestUserID), 0, -1).Result()
	require.NoError(t, err,
		"ZRange: %v", err)

	wantMembers := []string{"s-1", "s-2", "s-3", "s-4", "s-5"}
	require.Equal(t, strings.Join(wantMembers, ","), strings.Join(members, ","),
		"members = %v, want %v", members, wantMembers)
	require.False(t, redisServer.Exists(store.sessionKey(sessionTestUserID, "s-0")),
		"oldest session key still exists")

	_, err = store.GetSession(ctx, sessionTestUserID, "s-0")
	require.ErrorIs(t, err, authdomain.ErrAuthSessionNotFound,
		"GetSession pruned err = %v, want session not found", err)

	for _, sessionID := range wantMembers {
		require.True(t, redisServer.Exists(store.sessionKey(sessionTestUserID, sessionID)),
			"kept session key %s does not exist", sessionID)

	}
}

func TestSessionStoreCreateSessionAllowsUnlimitedWhenLimitDisabled(t *testing.T) {
	redisServer := miniredis.RunT(t)
	store := newTestSessionStore(redisServer)
	ctx := context.Background()
	limit := 0

	for i := 0; i < 6; i++ {
		sessionID := "s-" + strconv.Itoa(i)
		{
			err := store.CreateSession(ctx, authdomain.AuthSession{UserID: sessionTestUserID, SessionID: sessionID, TokenVersion: 1}, time.Hour, limit)
			require.NoError(t, err,
				"CreateSession(%s): %v", sessionID, err)
		}

	}

	count, err := store.redis.ZCard(ctx, store.userSessionsKey(sessionTestUserID)).Result()
	require.NoError(t, err,
		"ZCard: %v", err)
	require.EqualValues(t, 6, count,
		"session count = %d, want 6", count)

	for i := 0; i < 6; i++ {
		sessionID := "s-" + strconv.Itoa(i)
		require.True(t, redisServer.Exists(store.sessionKey(sessionTestUserID, sessionID)),
			"session key %s does not exist", sessionID)

	}
}

func TestSessionStoreCreateSessionCleansExpiredIndexBeforePruning(t *testing.T) {
	redisServer := miniredis.RunT(t)
	store := newTestSessionStore(redisServer)
	ctx := context.Background()
	indexKey := store.userSessionsKey(sessionTestUserID)
	limit := 1
	{
		err := store.redis.Set(ctx, store.sessionKey(sessionTestUserID, "expired-session"), "stale", time.Hour).Err()
		require.NoError(t, err,
			"Set expired session: %v", err)
	}
	{

		err := store.redis.ZAdd(ctx, indexKey, rediscache.Z{Score: redisScoreFloat(time.Now().Add(-time.Minute)), Member: "expired-session"}).Err()
		require.NoError(t, err,
			"ZAdd expired session: %v", err)
	}
	{

		err := store.CreateSession(ctx, authdomain.AuthSession{UserID: sessionTestUserID, SessionID: "s-new", TokenVersion: 1}, time.Hour, limit)
		require.NoError(t, err,
			"CreateSession: %v", err)
	}

	members, err := store.redis.ZRange(ctx, indexKey, 0, -1).Result()
	require.NoError(t, err,
		"ZRange: %v", err)
	require.False(t, len(members) != 1 || members[0] != "s-new",
		"members = %v, want only s-new", members)
	require.True(t, redisServer.Exists(store.sessionKey(sessionTestUserID, "expired-session")),
		"expired payload key should be left for its own TTL")

}

func TestSessionStoreCreateSessionConcurrentAttemptsRespectLimit(t *testing.T) {
	redisServer := miniredis.RunT(t)
	store := newTestSessionStore(redisServer)
	ctx := context.Background()
	limit := 5
	const attempts = 12
	start := make(chan struct{})
	var wg sync.WaitGroup
	errs := make(chan error, attempts)

	for i := 0; i < attempts; i++ {
		i := i
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			sessionID := "s-concurrent-" + strconv.Itoa(i)
			errs <- store.CreateSession(ctx, authdomain.AuthSession{UserID: sessionTestUserID, SessionID: sessionID, TokenVersion: 1}, time.Hour, limit)
		}()
	}
	close(start)
	wg.Wait()
	close(errs)
	for err := range errs {
		require.NoError(t, err,
			"CreateSession concurrent: %v", err)

	}
	count, err := store.redis.ZCard(ctx, store.userSessionsKey(sessionTestUserID)).Result()
	require.NoError(t, err,
		"ZCard: %v", err)
	require.Equal(t, int64(limit), count,
		"session count = %d, want %d", count, limit)

}
