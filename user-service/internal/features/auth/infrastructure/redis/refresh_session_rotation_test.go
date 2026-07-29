package redis

import (
	"context"
	"encoding/json"
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

func TestSessionStoreRotateSession(t *testing.T) {
	redisServer := miniredis.RunT(t)
	store := newTestSessionStore(redisServer)
	ctx := context.Background()
	indexKey := store.userSessionsKey(sessionTestUserID)
	ttl := time.Hour
	oldSession := authdomain.AuthSession{UserID: sessionTestUserID, SessionID: "s-old", TokenVersion: 1}
	newSession := authdomain.AuthSession{UserID: sessionTestUserID, SessionID: "s-new", TokenVersion: 1, ExpiresAt: time.Now().Add(24 * time.Hour)}
	{
		err := store.CreateSession(ctx, oldSession, ttl, defaultMaxActiveSessionsPerUser())
		require.NoError(t, err,
			"CreateSession: %v", err)
	}
	{

		err := store.redis.ZAdd(ctx, indexKey, rediscache.Z{Score: float64(time.Now().Add(-time.Minute).Unix()), Member: "expired-session"}).Err()
		require.NoError(t, err,
			"ZAdd expired session: %v", err)
	}

	beforeRotate := time.Now()
	{
		err := store.RotateSession(ctx, oldSession, newSession, ttl, defaultMaxActiveSessionsPerUser())
		require.NoError(t, err,
			"RotateSession: %v", err)
	}

	afterRotate := time.Now()
	require.False(t, redisServer.Exists(store.sessionKey(sessionTestUserID, "s-old")),
		"old session key still exists")

	stored, err := store.GetSession(ctx, sessionTestUserID, "s-new")
	require.NoError(t, err,
		"GetSession(new): %v", err)
	require.False(t, stored.UserID != sessionTestUserID || stored.SessionID != "s-new" || stored.TokenVersion != 1,
		"stored = %#v", stored)
	require.False(t, stored.ExpiresAt.Before(beforeRotate.Add(ttl)) || stored.ExpiresAt.After(afterRotate.Add(ttl)),
		"stored ExpiresAt = %s, want derived from ttl %s", stored.ExpiresAt, ttl)
	require.NotEqual(t, newSession.ExpiresAt.Unix(), stored.ExpiresAt.Unix(),
		"stored ExpiresAt used caller-provided mismatched value %s", newSession.ExpiresAt)
	{

		_, err := store.redis.ZScore(ctx, indexKey, "s-old").Result()
		require.ErrorIs(t, err, rediscache.Nil,
			"old session ZScore err = %v, want redis.Nil", err)
	}

	score, err := store.redis.ZScore(ctx, indexKey, "s-new").Result()
	require.NoError(t, err,
		"new session ZScore: %v", err)
	require.Equal(t, stored.ExpiresAt.Unix(), int64(score),
		"new session score = %d, want %d", int64(score), stored.ExpiresAt.Unix())
	{

		_, err := store.redis.ZScore(ctx, indexKey, "expired-session").Result()
		require.ErrorIs(t, err, rediscache.Nil,
			"expired session ZScore err = %v, want redis.Nil", err)
	}

	indexTTL, err := store.redis.TTL(ctx, indexKey).Result()
	require.NoError(t, err,
		"index TTL: %v", err)
	require.False(t, indexTTL <= ttl || indexTTL > ttl+authSessionIndexTTLBuffer,
		"index TTL = %s, want between session ttl and %s", indexTTL, ttl+authSessionIndexTTLBuffer)

}

func TestSessionStoreRotateSessionPrunesOldestWhenLimitExceeded(t *testing.T) {
	redisServer := miniredis.RunT(t)
	store := newTestSessionStore(redisServer)
	ctx := context.Background()
	indexKey := store.userSessionsKey(sessionTestUserID)
	limit := 3
	for i := 0; i < 5; i++ {
		sessionID := "s-" + strconv.Itoa(i)
		session := authdomain.AuthSession{UserID: sessionTestUserID, SessionID: sessionID, TokenVersion: 1}
		data, err := json.Marshal(session)
		require.NoError(t, err,
			"Marshal session %s: %v", sessionID, err)
		{

			err := store.redis.Set(ctx, store.sessionKey(sessionTestUserID, sessionID), data, time.Hour).Err()
			require.NoError(t, err,
				"Set session %s: %v", sessionID, err)
		}
		{

			err := store.redis.ZAdd(ctx, indexKey, rediscache.Z{Score: redisScoreFloat(time.Now().Add(time.Duration(i) * time.Minute)), Member: sessionID}).Err()
			require.NoError(t, err,
				"ZAdd session %s: %v", sessionID, err)
		}

	}
	oldSession := authdomain.AuthSession{UserID: sessionTestUserID, SessionID: "s-4", TokenVersion: 1}
	newSession := authdomain.AuthSession{UserID: sessionTestUserID, SessionID: "s-new", TokenVersion: 1}
	{

		err := store.RotateSession(ctx, oldSession, newSession, time.Hour, limit)
		require.NoError(t, err,
			"RotateSession: %v", err)
	}

	members, err := store.redis.ZRange(ctx, indexKey, 0, -1).Result()
	require.NoError(t, err,
		"ZRange: %v", err)

	wantMembers := []string{"s-2", "s-3", "s-new"}
	require.Equal(t, strings.Join(wantMembers, ","), strings.Join(members, ","),
		"members = %v, want %v", members, wantMembers)

	for _, sessionID := range []string{"s-1", "s-4"} {
		require.False(t, redisServer.Exists(store.sessionKey(sessionTestUserID, sessionID)),
			"pruned or rotated session key %s still exists", sessionID)

	}
	require.True(t, redisServer.Exists(store.sessionKey(sessionTestUserID, "s-new")),
		"new rotated session key does not exist")

}

func TestSessionStoreRotateSessionRejectsMissingOldSession(t *testing.T) {
	redisServer := miniredis.RunT(t)
	store := newTestSessionStore(redisServer)
	ctx := context.Background()
	oldSession := authdomain.AuthSession{UserID: sessionTestUserID, SessionID: "s-missing", TokenVersion: 1}
	newSession := authdomain.AuthSession{UserID: sessionTestUserID, SessionID: "s-new", TokenVersion: 1}

	err := store.RotateSession(ctx, oldSession, newSession, time.Hour, defaultMaxActiveSessionsPerUser())
	require.ErrorIs(t, err, authdomain.ErrAuthSessionNotFound,
		"err = %v, want session not found", err)
	require.False(t, redisServer.Exists(store.sessionKey(sessionTestUserID, "s-new")),
		"new session was created after missing old session")

}

func TestSessionStoreRotateSessionRejectsOldSessionMismatch(t *testing.T) {
	redisServer := miniredis.RunT(t)
	store := newTestSessionStore(redisServer)
	ctx := context.Background()
	storedOldSession := authdomain.AuthSession{UserID: sessionTestUserID, SessionID: "s-old", TokenVersion: 1}
	{
		err := store.CreateSession(ctx, storedOldSession, time.Hour, defaultMaxActiveSessionsPerUser())
		require.NoError(t, err,
			"CreateSession: %v", err)
	}

	oldSession := authdomain.AuthSession{UserID: sessionTestUserID, SessionID: "s-old", TokenVersion: 2}
	newSession := authdomain.AuthSession{UserID: sessionTestUserID, SessionID: "s-new", TokenVersion: 2}

	err := store.RotateSession(ctx, oldSession, newSession, time.Hour, defaultMaxActiveSessionsPerUser())
	require.ErrorIs(t, err, authdomain.ErrAuthSessionMismatch,
		"err = %v, want session mismatch", err)
	require.True(t, redisServer.Exists(store.sessionKey(sessionTestUserID, "s-old")),
		"old session was deleted after mismatch")
	require.False(t, redisServer.Exists(store.sessionKey(sessionTestUserID, "s-new")),
		"new session was created after mismatch")

}

func TestSessionStoreRotateSessionConcurrentAttemptsSucceedOnce(t *testing.T) {
	redisServer := miniredis.RunT(t)
	store := newTestSessionStore(redisServer)
	ctx := context.Background()
	oldSession := authdomain.AuthSession{UserID: sessionTestUserID, SessionID: "s-old", TokenVersion: 1}
	{
		err := store.CreateSession(ctx, oldSession, time.Hour, defaultMaxActiveSessionsPerUser())
		require.NoError(t, err,
			"CreateSession: %v", err)
	}

	const attempts = 8
	start := make(chan struct{})
	var wg sync.WaitGroup
	results := make(chan error, attempts)
	for i := 0; i < attempts; i++ {
		i := i
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			newSession := authdomain.AuthSession{UserID: sessionTestUserID, SessionID: "s-new-" + strconv.Itoa(i), TokenVersion: 1}
			results <- store.RotateSession(ctx, oldSession, newSession, time.Hour, defaultMaxActiveSessionsPerUser())
		}()
	}
	close(start)
	wg.Wait()
	close(results)

	successes := 0
	for err := range results {
		if err == nil {
			successes++
			continue
		}
		require.ErrorIs(t, err, authdomain.ErrAuthSessionNotFound,
			"err = %v, want session not found for failed concurrent rotation", err)

	}
	require.EqualValues(t, 1, successes,
		"successes = %d, want 1", successes)

	members, err := store.redis.ZRange(ctx, store.userSessionsKey(sessionTestUserID), 0, -1).Result()
	require.NoError(t, err,
		"ZRange: %v", err)
	require.EqualValues(t, 1, len(members),
		"members = %v, want exactly one new session", members)

}
