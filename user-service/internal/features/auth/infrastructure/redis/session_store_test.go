package redis

import (
	"context"
	"encoding/json"
	"errors"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/google/uuid"
	rediscache "github.com/redis/go-redis/v9"
	"go.uber.org/fx"
	"go.uber.org/fx/fxtest"
	"go.uber.org/zap"

	"github.com/aegiscore/common/runtime/config"
	"github.com/aegiscore/common/runtime/localcache"
	"github.com/aegiscore/common/runtime/workerpool"
	commonauth "github.com/aegiscore/common/security/auth"
	authapplication "github.com/aegiscore/user-service/internal/features/auth/application"
	authvalidators "github.com/aegiscore/user-service/internal/features/auth/application/validators"
	authdomain "github.com/aegiscore/user-service/internal/features/auth/domain"
)

var sessionTestUserID = uuid.MustParse("018f6f3e-7c4d-7b2a-9f8a-4f6b1b2c3d4e")

func TestSessionStoreTokenVersionCacheMiss(t *testing.T) {
	redisServer := miniredis.RunT(t)
	store := newTestSessionStore(redisServer)

	_, err := store.GetCachedTokenVersion(context.Background(), sessionTestUserID.String())
	if !errors.Is(err, authdomain.ErrTokenVersionCacheMiss) {
		t.Fatalf("GetCachedTokenVersion err = %v, want cache miss", err)
	}
	if redisServer.Exists(store.tokenVersionKey(sessionTestUserID.String())) {
		t.Fatal("cache miss should not create token version key")
	}
}

func TestSessionStoreCachesAndGetsTokenVersion(t *testing.T) {
	redisServer := miniredis.RunT(t)
	store := newTestSessionStore(redisServer)

	if err := store.CacheTokenVersion(context.Background(), sessionTestUserID.String(), 7); err != nil {
		t.Fatalf("CacheTokenVersion: %v", err)
	}
	version, err := store.GetCachedTokenVersion(context.Background(), sessionTestUserID.String())
	if err != nil {
		t.Fatalf("GetCachedTokenVersion: %v", err)
	}
	if version != 7 {
		t.Fatalf("version = %d, want 7", version)
	}
	got, err := redisServer.Get(store.tokenVersionKey(sessionTestUserID.String()))
	if err != nil {
		t.Fatalf("Get cached token version: %v", err)
	}
	if got != "7" {
		t.Fatalf("cached token version = %q, want 7", got)
	}
}

func TestSessionStoreCacheTokenVersionOverwritesStaleValue(t *testing.T) {
	redisServer := miniredis.RunT(t)
	store := newTestSessionStore(redisServer)
	ctx := context.Background()

	if err := store.CacheTokenVersion(ctx, sessionTestUserID.String(), 7); err != nil {
		t.Fatalf("CacheTokenVersion old: %v", err)
	}
	if err := store.CacheTokenVersion(ctx, sessionTestUserID.String(), 8); err != nil {
		t.Fatalf("CacheTokenVersion new: %v", err)
	}

	version, err := store.GetCachedTokenVersion(ctx, sessionTestUserID.String())
	if err != nil {
		t.Fatalf("GetCachedTokenVersion: %v", err)
	}
	if version != 8 {
		t.Fatalf("version = %d, want 8", version)
	}
	ttl, err := store.redis.TTL(ctx, store.tokenVersionKey(sessionTestUserID.String())).Result()
	if err != nil {
		t.Fatalf("TTL: %v", err)
	}
	if ttl <= 0 || ttl > time.Minute {
		t.Fatalf("TTL = %s, want within explicit %s", ttl, time.Minute)
	}
}

func TestSessionStoreCacheTokenVersionDoesNotOverwriteNewerValue(t *testing.T) {
	redisServer := miniredis.RunT(t)
	store := newTestSessionStore(redisServer)
	ctx := context.Background()

	if err := store.CacheTokenVersion(ctx, sessionTestUserID.String(), 9); err != nil {
		t.Fatalf("CacheTokenVersion new: %v", err)
	}
	if err := store.CacheTokenVersion(ctx, sessionTestUserID.String(), 8); err != nil {
		t.Fatalf("CacheTokenVersion stale: %v", err)
	}

	version, err := store.GetCachedTokenVersion(ctx, sessionTestUserID.String())
	if err != nil {
		t.Fatalf("GetCachedTokenVersion: %v", err)
	}
	if version != 9 {
		t.Fatalf("version = %d, want 9", version)
	}
}

func TestSessionStoreDeleteCachedTokenVersion(t *testing.T) {
	redisServer := miniredis.RunT(t)
	store := newTestSessionStore(redisServer)
	ctx := context.Background()

	if err := store.CacheTokenVersion(ctx, sessionTestUserID.String(), 7); err != nil {
		t.Fatalf("CacheTokenVersion: %v", err)
	}
	if err := store.DeleteCachedTokenVersion(ctx, sessionTestUserID.String()); err != nil {
		t.Fatalf("DeleteCachedTokenVersion: %v", err)
	}

	_, err := store.GetCachedTokenVersion(ctx, sessionTestUserID.String())
	if !errors.Is(err, authdomain.ErrTokenVersionCacheMiss) {
		t.Fatalf("GetCachedTokenVersion err = %v, want cache miss", err)
	}
	if redisServer.Exists(store.tokenVersionKey(sessionTestUserID.String())) {
		t.Fatal("token version cache key still exists")
	}
	if store.tokenVersionKey(sessionTestUserID.String()) != "auth:user:token_version:{"+sessionTestUserID.String()+"}" {
		t.Fatalf("token version key changed: %q", store.tokenVersionKey(sessionTestUserID.String()))
	}
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
			store := newTestSessionStoreWithConfig(redisServer, config.AuthConfig{TokenVersionCacheTTL: tc.ttl})

			err := store.CacheTokenVersion(context.Background(), sessionTestUserID.String(), 7)

			if err != nil {
				t.Fatalf("CacheTokenVersion: %v", err)
			}
			ttl, err := store.redis.TTL(context.Background(), store.tokenVersionKey(sessionTestUserID.String())).Result()
			if err != nil {
				t.Fatalf("TTL: %v", err)
			}
			if ttl <= 0 || ttl > defaultTokenVersionCacheTTL {
				t.Fatalf("TTL = %s, want within default %s", ttl, defaultTokenVersionCacheTTL)
			}
		})
	}
}

func TestSessionStoreTokenVersionCacheUsesExplicitTTL(t *testing.T) {
	redisServer := miniredis.RunT(t)
	explicitTTL := time.Minute
	store := newTestSessionStoreWithConfig(redisServer, config.AuthConfig{TokenVersionCacheTTL: explicitTTL})

	err := store.CacheTokenVersion(context.Background(), sessionTestUserID.String(), 7)

	if err != nil {
		t.Fatalf("CacheTokenVersion: %v", err)
	}
	ttl, err := store.redis.TTL(context.Background(), store.tokenVersionKey(sessionTestUserID.String())).Result()
	if err != nil {
		t.Fatalf("TTL: %v", err)
	}
	if ttl <= 0 || ttl > explicitTTL {
		t.Fatalf("TTL = %s, want within explicit %s", ttl, explicitTTL)
	}
}

func TestSessionStoreTokenVersionInvalidCacheReportsMiss(t *testing.T) {
	redisServer := miniredis.RunT(t)
	store := newTestSessionStore(redisServer)
	ctx := context.Background()
	key := store.tokenVersionKey(sessionTestUserID.String())

	for _, value := range []string{"not-an-int", "0"} {
		if err := store.redis.Set(ctx, key, value, time.Minute).Err(); err != nil {
			t.Fatalf("Set token version cache: %v", err)
		}
		_, err := store.GetCachedTokenVersion(ctx, sessionTestUserID.String())
		if !errors.Is(err, authdomain.ErrTokenVersionCacheMiss) {
			t.Fatalf("GetCachedTokenVersion(%q) err = %v, want cache miss", value, err)
		}
	}
}

func TestTokenVersionValidatorBackfillsMiniredisCacheOnMiss(t *testing.T) {
	redisServer := miniredis.RunT(t)
	store := newTestSessionStore(redisServer)
	users := &tokenVersionRepositoryStub{version: 7}
	validator := newTestTokenVersionValidator(t, users, store)
	ctx := context.Background()

	err := validator.ValidateTokenVersion(ctx, sessionTestUserID.String(), 7)

	if err != nil {
		t.Fatalf("ValidateTokenVersion: %v", err)
	}
	if users.getTokenVersionCalls != 1 || users.gotUserID != sessionTestUserID {
		t.Fatalf("users repo calls=%d userID=%s, want one call for %s", users.getTokenVersionCalls, users.gotUserID, sessionTestUserID)
	}
	version, err := store.GetCachedTokenVersion(ctx, sessionTestUserID.String())
	if err != nil {
		t.Fatalf("GetCachedTokenVersion after backfill: %v", err)
	}
	if version != 7 {
		t.Fatalf("cached version = %d, want 7", version)
	}
	ttl, err := store.redis.TTL(ctx, store.tokenVersionKey(sessionTestUserID.String())).Result()
	if err != nil {
		t.Fatalf("TTL: %v", err)
	}
	if ttl <= 0 || ttl > time.Minute {
		t.Fatalf("TTL = %s, want within explicit %s", ttl, time.Minute)
	}
}

func TestTokenVersionValidatorUsesMiniredisCacheHitWithoutRepositoryLookup(t *testing.T) {
	redisServer := miniredis.RunT(t)
	store := newTestSessionStore(redisServer)
	ctx := context.Background()
	if err := store.CacheTokenVersion(ctx, sessionTestUserID.String(), 8); err != nil {
		t.Fatalf("CacheTokenVersion: %v", err)
	}
	users := &tokenVersionRepositoryStub{err: errors.New("database should not be read")}
	validator := newTestTokenVersionValidator(t, users, store)

	err := validator.ValidateTokenVersion(ctx, sessionTestUserID.String(), 8)

	if err != nil {
		t.Fatalf("ValidateTokenVersion: %v", err)
	}
	if users.getTokenVersionCalls != 0 {
		t.Fatalf("users repo calls = %d, want 0 on cache hit", users.getTokenVersionCalls)
	}
}

func TestTokenVersionValidatorRejectsStaleTokenUsingMiniredisCache(t *testing.T) {
	redisServer := miniredis.RunT(t)
	store := newTestSessionStore(redisServer)
	ctx := context.Background()
	if err := store.CacheTokenVersion(ctx, sessionTestUserID.String(), 9); err != nil {
		t.Fatalf("CacheTokenVersion: %v", err)
	}
	users := &tokenVersionRepositoryStub{err: errors.New("database should not be read")}
	validator := newTestTokenVersionValidator(t, users, store)

	err := validator.ValidateTokenVersion(ctx, sessionTestUserID.String(), 8)

	if !errors.Is(err, commonauth.ErrTokenVersionMismatch) {
		t.Fatalf("err = %v, want token version mismatch", err)
	}
	if users.getTokenVersionCalls != 0 {
		t.Fatalf("users repo calls = %d, want 0 on cache hit mismatch", users.getTokenVersionCalls)
	}
}

func TestTokenVersionCacheRefreshMakesStaleTokenObservable(t *testing.T) {
	redisServer := miniredis.RunT(t)
	store := newTestSessionStore(redisServer)
	ctx := context.Background()
	if err := store.CacheTokenVersion(ctx, sessionTestUserID.String(), 5); err != nil {
		t.Fatalf("CacheTokenVersion old: %v", err)
	}
	if err := store.CreateSession(ctx, authdomain.AuthSession{UserID: sessionTestUserID.String(), SessionID: "s-stale", TokenVersion: 5}, time.Hour, defaultMaxActiveSessionsPerUser()); err != nil {
		t.Fatalf("CreateSession: %v", err)
	}
	if err := store.CacheTokenVersion(ctx, sessionTestUserID.String(), 6); err != nil {
		t.Fatalf("CacheTokenVersion refreshed: %v", err)
	}
	if err := store.DeleteAllUserSessions(ctx, sessionTestUserID.String()); err != nil {
		t.Fatalf("DeleteAllUserSessions: %v", err)
	}
	validator := newTestTokenVersionValidator(t, &tokenVersionRepositoryStub{err: errors.New("database should not be read")}, store)

	err := validator.ValidateTokenVersion(ctx, sessionTestUserID.String(), 5)

	if !errors.Is(err, commonauth.ErrTokenVersionMismatch) {
		t.Fatalf("err = %v, want token version mismatch", err)
	}
	version, err := store.GetCachedTokenVersion(ctx, sessionTestUserID.String())
	if err != nil {
		t.Fatalf("GetCachedTokenVersion: %v", err)
	}
	if version != 6 {
		t.Fatalf("cached version = %d, want 6", version)
	}
	waitForRedisCondition(t, func() bool {
		return !redisServer.Exists(store.sessionKey(sessionTestUserID.String(), "s-stale")) &&
			!redisServer.Exists(store.userSessionsKey(sessionTestUserID.String()))
	}, "user sessions were not deleted during cache refresh flow")
}

func TestSessionStoreCreateGetAndDeleteSession(t *testing.T) {
	redisServer := miniredis.RunT(t)
	store := newTestSessionStore(redisServer)
	ctx := context.Background()
	indexKey := store.userSessionsKey(sessionTestUserID.String())
	ttl := time.Hour
	mismatchedExpiresAt := time.Now().Add(24 * time.Hour).Truncate(time.Second)
	session := authdomain.AuthSession{UserID: sessionTestUserID.String(), SessionID: "s-123", TokenVersion: 1, ExpiresAt: mismatchedExpiresAt}
	if err := store.redis.ZAdd(ctx, indexKey, rediscache.Z{Score: float64(time.Now().Add(-time.Minute).Unix()), Member: "expired-session"}).Err(); err != nil {
		t.Fatalf("ZAdd expired session: %v", err)
	}

	beforeCreate := time.Now()
	if err := store.CreateSession(ctx, session, ttl, defaultMaxActiveSessionsPerUser()); err != nil {
		t.Fatalf("CreateSession: %v", err)
	}
	afterCreate := time.Now()
	stored, err := store.GetSession(ctx, sessionTestUserID.String(), "s-123")
	if err != nil {
		t.Fatalf("GetSession: %v", err)
	}
	if stored.UserID != sessionTestUserID.String() || stored.SessionID != "s-123" || stored.TokenVersion != 1 {
		t.Fatalf("stored = %#v", stored)
	}
	if stored.ExpiresAt.Before(beforeCreate.Add(ttl)) || stored.ExpiresAt.After(afterCreate.Add(ttl)) {
		t.Fatalf("stored ExpiresAt = %s, want derived from ttl %s", stored.ExpiresAt, ttl)
	}
	if stored.ExpiresAt.Unix() == mismatchedExpiresAt.Unix() {
		t.Fatalf("stored ExpiresAt used caller-provided mismatched value %s", mismatchedExpiresAt)
	}
	score, err := store.redis.ZScore(ctx, indexKey, "s-123").Result()
	if err != nil {
		t.Fatalf("ZScore: %v", err)
	}
	if int64(score) != stored.ExpiresAt.Unix() {
		t.Fatalf("ZScore = %d, want %d", int64(score), stored.ExpiresAt.Unix())
	}
	sessionTTL, err := store.redis.TTL(ctx, store.sessionKey(sessionTestUserID.String(), "s-123")).Result()
	if err != nil {
		t.Fatalf("session TTL: %v", err)
	}
	if sessionTTL <= 0 || sessionTTL > ttl {
		t.Fatalf("session TTL = %s, want within %s", sessionTTL, ttl)
	}
	indexTTL, err := store.redis.TTL(ctx, indexKey).Result()
	if err != nil {
		t.Fatalf("index TTL: %v", err)
	}
	if indexTTL <= ttl || indexTTL > ttl+authSessionIndexTTLBuffer {
		t.Fatalf("index TTL = %s, want between session ttl and %s", indexTTL, ttl+authSessionIndexTTLBuffer)
	}
	if _, err := store.redis.ZScore(ctx, indexKey, "expired-session").Result(); !errors.Is(err, rediscache.Nil) {
		t.Fatalf("expired session ZScore err = %v, want redis.Nil", err)
	}
	typ, err := store.redis.Type(ctx, indexKey).Result()
	if err != nil {
		t.Fatalf("Type: %v", err)
	}
	if typ != "zset" {
		t.Fatalf("session index type = %q, want zset", typ)
	}

	if err := store.redis.ZAdd(ctx, indexKey, rediscache.Z{Score: float64(time.Now().Add(-time.Minute).Unix()), Member: "expired-on-delete"}).Err(); err != nil {
		t.Fatalf("ZAdd expired-on-delete: %v", err)
	}

	if err := store.DeleteSession(ctx, sessionTestUserID.String(), "s-123"); err != nil {
		t.Fatalf("DeleteSession: %v", err)
	}
	if redisServer.Exists(store.sessionKey(sessionTestUserID.String(), "s-123")) {
		t.Fatal("session key still exists")
	}
	if _, err := store.redis.ZScore(ctx, indexKey, "s-123").Result(); !errors.Is(err, rediscache.Nil) {
		t.Fatalf("deleted session ZScore err = %v, want redis.Nil", err)
	}
	if _, err := store.redis.ZScore(ctx, indexKey, "expired-on-delete").Result(); !errors.Is(err, rediscache.Nil) {
		t.Fatalf("expired-on-delete ZScore err = %v, want redis.Nil", err)
	}
}

func TestSessionStoreCreateSessionPrunesOldestWhenLimitExceeded(t *testing.T) {
	redisServer := miniredis.RunT(t)
	store := newTestSessionStore(redisServer)
	ctx := context.Background()
	limit := 5

	for i := 0; i < 6; i++ {
		sessionID := "s-" + strconv.Itoa(i)
		if err := store.CreateSession(ctx, authdomain.AuthSession{UserID: sessionTestUserID.String(), SessionID: sessionID, TokenVersion: 1}, time.Hour, limit); err != nil {
			t.Fatalf("CreateSession(%s): %v", sessionID, err)
		}
		time.Sleep(time.Millisecond)
	}

	members, err := store.redis.ZRange(ctx, store.userSessionsKey(sessionTestUserID.String()), 0, -1).Result()
	if err != nil {
		t.Fatalf("ZRange: %v", err)
	}
	wantMembers := []string{"s-1", "s-2", "s-3", "s-4", "s-5"}
	if strings.Join(members, ",") != strings.Join(wantMembers, ",") {
		t.Fatalf("members = %v, want %v", members, wantMembers)
	}
	if redisServer.Exists(store.sessionKey(sessionTestUserID.String(), "s-0")) {
		t.Fatal("oldest session key still exists")
	}
	_, err = store.GetSession(ctx, sessionTestUserID.String(), "s-0")
	if !errors.Is(err, authdomain.ErrAuthSessionNotFound) {
		t.Fatalf("GetSession pruned err = %v, want session not found", err)
	}
	for _, sessionID := range wantMembers {
		if !redisServer.Exists(store.sessionKey(sessionTestUserID.String(), sessionID)) {
			t.Fatalf("kept session key %s does not exist", sessionID)
		}
	}
}

func TestSessionStoreCreateSessionAllowsUnlimitedWhenLimitDisabled(t *testing.T) {
	redisServer := miniredis.RunT(t)
	store := newTestSessionStore(redisServer)
	ctx := context.Background()
	limit := 0

	for i := 0; i < 6; i++ {
		sessionID := "s-" + strconv.Itoa(i)
		if err := store.CreateSession(ctx, authdomain.AuthSession{UserID: sessionTestUserID.String(), SessionID: sessionID, TokenVersion: 1}, time.Hour, limit); err != nil {
			t.Fatalf("CreateSession(%s): %v", sessionID, err)
		}
	}

	count, err := store.redis.ZCard(ctx, store.userSessionsKey(sessionTestUserID.String())).Result()
	if err != nil {
		t.Fatalf("ZCard: %v", err)
	}
	if count != 6 {
		t.Fatalf("session count = %d, want 6", count)
	}
	for i := 0; i < 6; i++ {
		sessionID := "s-" + strconv.Itoa(i)
		if !redisServer.Exists(store.sessionKey(sessionTestUserID.String(), sessionID)) {
			t.Fatalf("session key %s does not exist", sessionID)
		}
	}
}

func TestSessionStoreCreateSessionCleansExpiredIndexBeforePruning(t *testing.T) {
	redisServer := miniredis.RunT(t)
	store := newTestSessionStore(redisServer)
	ctx := context.Background()
	indexKey := store.userSessionsKey(sessionTestUserID.String())
	limit := 1
	if err := store.redis.Set(ctx, store.sessionKey(sessionTestUserID.String(), "expired-session"), "stale", time.Hour).Err(); err != nil {
		t.Fatalf("Set expired session: %v", err)
	}
	if err := store.redis.ZAdd(ctx, indexKey, rediscache.Z{Score: redisScoreFloat(time.Now().Add(-time.Minute)), Member: "expired-session"}).Err(); err != nil {
		t.Fatalf("ZAdd expired session: %v", err)
	}

	if err := store.CreateSession(ctx, authdomain.AuthSession{UserID: sessionTestUserID.String(), SessionID: "s-new", TokenVersion: 1}, time.Hour, limit); err != nil {
		t.Fatalf("CreateSession: %v", err)
	}

	members, err := store.redis.ZRange(ctx, indexKey, 0, -1).Result()
	if err != nil {
		t.Fatalf("ZRange: %v", err)
	}
	if len(members) != 1 || members[0] != "s-new" {
		t.Fatalf("members = %v, want only s-new", members)
	}
	if !redisServer.Exists(store.sessionKey(sessionTestUserID.String(), "expired-session")) {
		t.Fatal("expired payload key should be left for its own TTL")
	}
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
			errs <- store.CreateSession(ctx, authdomain.AuthSession{UserID: sessionTestUserID.String(), SessionID: sessionID, TokenVersion: 1}, time.Hour, limit)
		}()
	}
	close(start)
	wg.Wait()
	close(errs)
	for err := range errs {
		if err != nil {
			t.Fatalf("CreateSession concurrent: %v", err)
		}
	}
	count, err := store.redis.ZCard(ctx, store.userSessionsKey(sessionTestUserID.String())).Result()
	if err != nil {
		t.Fatalf("ZCard: %v", err)
	}
	if count != int64(limit) {
		t.Fatalf("session count = %d, want %d", count, limit)
	}
}

func TestSessionStoreRotateSession(t *testing.T) {
	redisServer := miniredis.RunT(t)
	store := newTestSessionStore(redisServer)
	ctx := context.Background()
	indexKey := store.userSessionsKey(sessionTestUserID.String())
	ttl := time.Hour
	oldSession := authdomain.AuthSession{UserID: sessionTestUserID.String(), SessionID: "s-old", TokenVersion: 1}
	newSession := authdomain.AuthSession{UserID: sessionTestUserID.String(), SessionID: "s-new", TokenVersion: 1, ExpiresAt: time.Now().Add(24 * time.Hour)}
	if err := store.CreateSession(ctx, oldSession, ttl, defaultMaxActiveSessionsPerUser()); err != nil {
		t.Fatalf("CreateSession: %v", err)
	}
	if err := store.redis.ZAdd(ctx, indexKey, rediscache.Z{Score: float64(time.Now().Add(-time.Minute).Unix()), Member: "expired-session"}).Err(); err != nil {
		t.Fatalf("ZAdd expired session: %v", err)
	}

	beforeRotate := time.Now()
	if err := store.RotateSession(ctx, oldSession, newSession, ttl, defaultMaxActiveSessionsPerUser()); err != nil {
		t.Fatalf("RotateSession: %v", err)
	}
	afterRotate := time.Now()

	if redisServer.Exists(store.sessionKey(sessionTestUserID.String(), "s-old")) {
		t.Fatal("old session key still exists")
	}
	stored, err := store.GetSession(ctx, sessionTestUserID.String(), "s-new")
	if err != nil {
		t.Fatalf("GetSession(new): %v", err)
	}
	if stored.UserID != sessionTestUserID.String() || stored.SessionID != "s-new" || stored.TokenVersion != 1 {
		t.Fatalf("stored = %#v", stored)
	}
	if stored.ExpiresAt.Before(beforeRotate.Add(ttl)) || stored.ExpiresAt.After(afterRotate.Add(ttl)) {
		t.Fatalf("stored ExpiresAt = %s, want derived from ttl %s", stored.ExpiresAt, ttl)
	}
	if stored.ExpiresAt.Unix() == newSession.ExpiresAt.Unix() {
		t.Fatalf("stored ExpiresAt used caller-provided mismatched value %s", newSession.ExpiresAt)
	}
	if _, err := store.redis.ZScore(ctx, indexKey, "s-old").Result(); !errors.Is(err, rediscache.Nil) {
		t.Fatalf("old session ZScore err = %v, want redis.Nil", err)
	}
	score, err := store.redis.ZScore(ctx, indexKey, "s-new").Result()
	if err != nil {
		t.Fatalf("new session ZScore: %v", err)
	}
	if int64(score) != stored.ExpiresAt.Unix() {
		t.Fatalf("new session score = %d, want %d", int64(score), stored.ExpiresAt.Unix())
	}
	if _, err := store.redis.ZScore(ctx, indexKey, "expired-session").Result(); !errors.Is(err, rediscache.Nil) {
		t.Fatalf("expired session ZScore err = %v, want redis.Nil", err)
	}
	indexTTL, err := store.redis.TTL(ctx, indexKey).Result()
	if err != nil {
		t.Fatalf("index TTL: %v", err)
	}
	if indexTTL <= ttl || indexTTL > ttl+authSessionIndexTTLBuffer {
		t.Fatalf("index TTL = %s, want between session ttl and %s", indexTTL, ttl+authSessionIndexTTLBuffer)
	}
}

func TestSessionStoreRotateSessionPrunesOldestWhenLimitExceeded(t *testing.T) {
	redisServer := miniredis.RunT(t)
	store := newTestSessionStore(redisServer)
	ctx := context.Background()
	indexKey := store.userSessionsKey(sessionTestUserID.String())
	limit := 3
	for i := 0; i < 5; i++ {
		sessionID := "s-" + strconv.Itoa(i)
		session := authdomain.AuthSession{UserID: sessionTestUserID.String(), SessionID: sessionID, TokenVersion: 1}
		data, err := json.Marshal(session)
		if err != nil {
			t.Fatalf("Marshal session %s: %v", sessionID, err)
		}
		if err := store.redis.Set(ctx, store.sessionKey(sessionTestUserID.String(), sessionID), data, time.Hour).Err(); err != nil {
			t.Fatalf("Set session %s: %v", sessionID, err)
		}
		if err := store.redis.ZAdd(ctx, indexKey, rediscache.Z{Score: redisScoreFloat(time.Now().Add(time.Duration(i) * time.Minute)), Member: sessionID}).Err(); err != nil {
			t.Fatalf("ZAdd session %s: %v", sessionID, err)
		}
	}
	oldSession := authdomain.AuthSession{UserID: sessionTestUserID.String(), SessionID: "s-4", TokenVersion: 1}
	newSession := authdomain.AuthSession{UserID: sessionTestUserID.String(), SessionID: "s-new", TokenVersion: 1}

	if err := store.RotateSession(ctx, oldSession, newSession, time.Hour, limit); err != nil {
		t.Fatalf("RotateSession: %v", err)
	}

	members, err := store.redis.ZRange(ctx, indexKey, 0, -1).Result()
	if err != nil {
		t.Fatalf("ZRange: %v", err)
	}
	wantMembers := []string{"s-2", "s-3", "s-new"}
	if strings.Join(members, ",") != strings.Join(wantMembers, ",") {
		t.Fatalf("members = %v, want %v", members, wantMembers)
	}
	for _, sessionID := range []string{"s-1", "s-4"} {
		if redisServer.Exists(store.sessionKey(sessionTestUserID.String(), sessionID)) {
			t.Fatalf("pruned or rotated session key %s still exists", sessionID)
		}
	}
	if !redisServer.Exists(store.sessionKey(sessionTestUserID.String(), "s-new")) {
		t.Fatal("new rotated session key does not exist")
	}
}

func TestSessionStoreRotateSessionRejectsMissingOldSession(t *testing.T) {
	redisServer := miniredis.RunT(t)
	store := newTestSessionStore(redisServer)
	ctx := context.Background()
	oldSession := authdomain.AuthSession{UserID: sessionTestUserID.String(), SessionID: "s-missing", TokenVersion: 1}
	newSession := authdomain.AuthSession{UserID: sessionTestUserID.String(), SessionID: "s-new", TokenVersion: 1}

	err := store.RotateSession(ctx, oldSession, newSession, time.Hour, defaultMaxActiveSessionsPerUser())

	if !errors.Is(err, authdomain.ErrAuthSessionNotFound) {
		t.Fatalf("err = %v, want session not found", err)
	}
	if redisServer.Exists(store.sessionKey(sessionTestUserID.String(), "s-new")) {
		t.Fatal("new session was created after missing old session")
	}
}

func TestSessionStoreRotateSessionRejectsOldSessionMismatch(t *testing.T) {
	redisServer := miniredis.RunT(t)
	store := newTestSessionStore(redisServer)
	ctx := context.Background()
	storedOldSession := authdomain.AuthSession{UserID: sessionTestUserID.String(), SessionID: "s-old", TokenVersion: 1}
	if err := store.CreateSession(ctx, storedOldSession, time.Hour, defaultMaxActiveSessionsPerUser()); err != nil {
		t.Fatalf("CreateSession: %v", err)
	}
	oldSession := authdomain.AuthSession{UserID: sessionTestUserID.String(), SessionID: "s-old", TokenVersion: 2}
	newSession := authdomain.AuthSession{UserID: sessionTestUserID.String(), SessionID: "s-new", TokenVersion: 2}

	err := store.RotateSession(ctx, oldSession, newSession, time.Hour, defaultMaxActiveSessionsPerUser())

	if !errors.Is(err, authdomain.ErrAuthSessionMismatch) {
		t.Fatalf("err = %v, want session mismatch", err)
	}
	if !redisServer.Exists(store.sessionKey(sessionTestUserID.String(), "s-old")) {
		t.Fatal("old session was deleted after mismatch")
	}
	if redisServer.Exists(store.sessionKey(sessionTestUserID.String(), "s-new")) {
		t.Fatal("new session was created after mismatch")
	}
}

func TestSessionStoreRotateSessionConcurrentAttemptsSucceedOnce(t *testing.T) {
	redisServer := miniredis.RunT(t)
	store := newTestSessionStore(redisServer)
	ctx := context.Background()
	oldSession := authdomain.AuthSession{UserID: sessionTestUserID.String(), SessionID: "s-old", TokenVersion: 1}
	if err := store.CreateSession(ctx, oldSession, time.Hour, defaultMaxActiveSessionsPerUser()); err != nil {
		t.Fatalf("CreateSession: %v", err)
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
			newSession := authdomain.AuthSession{UserID: sessionTestUserID.String(), SessionID: "s-new-" + strconv.Itoa(i), TokenVersion: 1}
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
		if !errors.Is(err, authdomain.ErrAuthSessionNotFound) {
			t.Fatalf("err = %v, want session not found for failed concurrent rotation", err)
		}
	}
	if successes != 1 {
		t.Fatalf("successes = %d, want 1", successes)
	}
	members, err := store.redis.ZRange(ctx, store.userSessionsKey(sessionTestUserID.String()), 0, -1).Result()
	if err != nil {
		t.Fatalf("ZRange: %v", err)
	}
	if len(members) != 1 {
		t.Fatalf("members = %v, want exactly one new session", members)
	}
}

func TestSessionStoreDeleteAllUserSessions(t *testing.T) {
	redisServer := miniredis.RunT(t)
	store := newTestSessionStore(redisServer)
	ctx := context.Background()
	indexKey := store.userSessionsKey(sessionTestUserID.String())
	for _, sessionID := range []string{"s-1", "s-2"} {
		if err := store.CreateSession(ctx, authdomain.AuthSession{UserID: sessionTestUserID.String(), SessionID: sessionID, TokenVersion: 1, ExpiresAt: time.Now().Add(time.Hour)}, time.Hour, defaultMaxActiveSessionsPerUser()); err != nil {
			t.Fatalf("CreateSession: %v", err)
		}
	}
	if err := store.redis.Set(ctx, store.sessionKey(sessionTestUserID.String(), "expired-session"), "stale", 0).Err(); err != nil {
		t.Fatalf("Set expired session: %v", err)
	}
	if err := store.redis.ZAdd(ctx, indexKey, rediscache.Z{Score: float64(time.Now().Add(-time.Minute).Unix()), Member: "expired-session"}).Err(); err != nil {
		t.Fatalf("ZAdd expired session: %v", err)
	}
	if err := store.redis.ZAdd(ctx, indexKey, rediscache.Z{Score: float64(time.Now().Add(time.Hour).Unix()), Member: "missing-session"}).Err(); err != nil {
		t.Fatalf("ZAdd missing session: %v", err)
	}
	if err := store.DeleteAllUserSessions(ctx, sessionTestUserID.String()); err != nil {
		t.Fatalf("DeleteAllUserSessions: %v", err)
	}
	waitForRedisCondition(t, func() bool {
		return !redisServer.Exists(store.sessionKey(sessionTestUserID.String(), "s-1")) &&
			!redisServer.Exists(store.sessionKey(sessionTestUserID.String(), "s-2")) &&
			!redisServer.Exists(indexKey)
	}, "user sessions were not fully deleted")
	if !redisServer.Exists(store.sessionKey(sessionTestUserID.String(), "expired-session")) {
		t.Fatal("expired session key was deleted despite expired index member cleanup")
	}
}

func TestSessionStoreDeleteAllUserSessionsPurgesInBatches(t *testing.T) {
	redisServer := miniredis.RunT(t)
	store := newTestSessionStore(redisServer)
	ctx := context.Background()
	indexKey := store.userSessionsKey(sessionTestUserID.String())
	sessionCount := int(deleteAllUserSessionsBatchSize)*2 + 1
	for i := 0; i < sessionCount; i++ {
		sessionID := "bulk-" + strconv.Itoa(i)
		if err := store.redis.Set(ctx, store.sessionKey(sessionTestUserID.String(), sessionID), "{}", time.Hour).Err(); err != nil {
			t.Fatalf("Set session %d: %v", i, err)
		}
		if err := store.redis.ZAdd(ctx, indexKey, rediscache.Z{Score: float64(time.Now().Add(time.Hour).Unix()), Member: sessionID}).Err(); err != nil {
			t.Fatalf("ZAdd session %d: %v", i, err)
		}
	}

	if err := store.DeleteAllUserSessions(ctx, sessionTestUserID.String()); err != nil {
		t.Fatalf("DeleteAllUserSessions: %v", err)
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
	if err := store.CreateSession(ctx, authdomain.AuthSession{UserID: sessionTestUserID.String(), SessionID: oldSessionID, TokenVersion: 1}, time.Hour, defaultMaxActiveSessionsPerUser()); err != nil {
		t.Fatalf("CreateSession old: %v", err)
	}

	if err := store.DeleteAllUserSessions(ctx, sessionTestUserID.String()); err != nil {
		t.Fatalf("DeleteAllUserSessions: %v", err)
	}
	if err := store.CreateSession(ctx, authdomain.AuthSession{UserID: sessionTestUserID.String(), SessionID: newSessionID, TokenVersion: 2}, time.Hour, defaultMaxActiveSessionsPerUser()); err != nil {
		t.Fatalf("CreateSession new: %v", err)
	}

	waitForRedisCondition(t, func() bool {
		return !redisServer.Exists(store.sessionKey(sessionTestUserID.String(), oldSessionID))
	}, "detached old session was not purged")
	if !redisServer.Exists(store.sessionKey(sessionTestUserID.String(), newSessionID)) {
		t.Fatal("new session created after detach was deleted")
	}
	members, err := store.redis.ZRange(ctx, store.userSessionsKey(sessionTestUserID.String()), 0, -1).Result()
	if err != nil {
		t.Fatalf("ZRange new index: %v", err)
	}
	if len(members) != 1 || members[0] != newSessionID {
		t.Fatalf("members = %v, want only new session", members)
	}
}

func TestSessionStoreDeleteAllUserSessionsReturnsSubmitError(t *testing.T) {
	redisServer := miniredis.RunT(t)
	store := newTestSessionStore(redisServer)
	metrics := &sessionStoreMetricsSpy{}
	store.metrics = metrics
	store.purgePool = rejectingPurgeTaskPool{err: workerpool.ErrQueueFull}
	ctx := context.Background()
	if err := store.CreateSession(ctx, authdomain.AuthSession{UserID: sessionTestUserID.String(), SessionID: "s-rejected", TokenVersion: 1}, time.Hour, defaultMaxActiveSessionsPerUser()); err != nil {
		t.Fatalf("CreateSession: %v", err)
	}

	err := store.DeleteAllUserSessions(ctx, sessionTestUserID.String())

	if !errors.Is(err, workerpool.ErrQueueFull) {
		t.Fatalf("DeleteAllUserSessions err = %v, want ErrQueueFull", err)
	}
	if err == nil || !strings.Contains(err.Error(), "submit delete user auth sessions purge") {
		t.Fatalf("DeleteAllUserSessions err = %v, want submit context", err)
	}
	if metrics.sessionPurgeFailures != 1 {
		t.Fatalf("session purge submit metrics = %d, want 1", metrics.sessionPurgeFailures)
	}
}

func TestSessionStoreDeleteAllUserSessionsPurgeFailureIsObservable(t *testing.T) {
	redisServer := miniredis.RunT(t)
	store := newTestSessionStore(redisServer)
	pool := &recordingPurgeTaskPool{beforeRun: redisServer.Close}
	store.purgePool = pool
	ctx := context.Background()
	if err := store.CreateSession(ctx, authdomain.AuthSession{UserID: sessionTestUserID.String(), SessionID: "s-fails", TokenVersion: 1}, time.Hour, defaultMaxActiveSessionsPerUser()); err != nil {
		t.Fatalf("CreateSession: %v", err)
	}
	if err := store.DeleteAllUserSessions(ctx, sessionTestUserID.String()); err != nil {
		t.Fatalf("DeleteAllUserSessions: %v", err)
	}

	if pool.Stats().Failed != 1 {
		t.Fatalf("Failed stats = %d, want 1", pool.Stats().Failed)
	}
	if pool.err == nil || !strings.Contains(pool.err.Error(), "read detached user sessions") {
		t.Fatalf("purge err = %v, want read detached user sessions", pool.err)
	}
	if pool.taskName != "auth.redis.purge_detached_user_sessions" {
		t.Fatalf("task name = %q, want auth.redis.purge_detached_user_sessions", pool.taskName)
	}
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
	if err != nil {
		t.Fatalf("NewSessionPurgePool: %v", err)
	}
	store, err := NewSessionStore(SessionStoreParams{
		Redis:     client,
		Cfg:       &config.Config{},
		PurgePool: pool,
	})
	if err != nil {
		t.Fatalf("NewSessionStore: %v", err)
	}
	if store.purgePool == nil {
		t.Fatal("purgePool = nil")
	}
	if len(lifecycle.hooks) != 2 {
		t.Fatalf("lifecycle hooks = %d, want redis and purge pool hooks", len(lifecycle.hooks))
	}
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
		if err := hook.OnStop(stopCtx); err != nil {
			t.Fatalf("OnStop hook %d: %v", i, err)
		}
	}
	if strings.Join(stopOrder, ",") != "purge_pool,redis" {
		t.Fatalf("stop order = %v, want purge_pool before redis", stopOrder)
	}
}

func TestSessionStoreConsumesNamedPurgePool(t *testing.T) {
	redisServer := miniredis.RunT(t)
	client := rediscache.NewClient(&rediscache.Options{Addr: redisServer.Addr()})
	var store *SessionStore
	app := fxtest.New(t,
		fx.Provide(
			func() *config.Config {
				return &config.Config{Auth: config.AuthConfig{TokenVersionCacheTTL: time.Minute}}
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
	if store == nil {
		t.Fatal("store = nil")
	}
	if store.purgePool == nil {
		t.Fatal("purgePool = nil")
	}
}

func TestSessionStorePurgeUserSessionsKeyKeepsHashTag(t *testing.T) {
	redisServer := miniredis.RunT(t)
	store := newTestSessionStore(redisServer)

	purgeKey, err := store.purgeUserSessionsKey(sessionTestUserID.String())
	if err != nil {
		t.Fatalf("purgeUserSessionsKey: %v", err)
	}

	if !strings.HasPrefix(purgeKey, "auth:user:sessions:{"+sessionTestUserID.String()+"}:purge:") {
		t.Fatalf("purge key = %q, want unprefixed purge key prefix", purgeKey)
	}
	if !strings.Contains(purgeKey, "{"+sessionTestUserID.String()+"}") {
		t.Fatalf("purge key = %q, want user hash tag", purgeKey)
	}
}

func TestSessionStorePurgeUserSessionsKeyUsesAppNamePrefix(t *testing.T) {
	redisServer := miniredis.RunT(t)
	store := newTestSessionStoreWithAppName(t, redisServer, " aegiscore-user-services ")

	purgeKey, err := store.purgeUserSessionsKey(sessionTestUserID.String())
	if err != nil {
		t.Fatalf("purgeUserSessionsKey: %v", err)
	}

	if !strings.HasPrefix(purgeKey, "aegiscore-user-services:auth:user:sessions:{"+sessionTestUserID.String()+"}:purge:") {
		t.Fatalf("purge key = %q, want app-name-prefixed purge key", purgeKey)
	}
	if !strings.Contains(purgeKey, "{"+sessionTestUserID.String()+"}") {
		t.Fatalf("purge key = %q, want user hash tag", purgeKey)
	}
}

func TestSessionStoreUserSessionsIndexTTLIsNotShortened(t *testing.T) {
	redisServer := miniredis.RunT(t)
	store := newTestSessionStore(redisServer)
	ctx := context.Background()
	indexKey := store.userSessionsKey(sessionTestUserID.String())
	longTTL := 2 * time.Hour
	shortTTL := time.Hour

	if err := store.CreateSession(ctx, authdomain.AuthSession{UserID: sessionTestUserID.String(), SessionID: "long", TokenVersion: 1}, longTTL, defaultMaxActiveSessionsPerUser()); err != nil {
		t.Fatalf("CreateSession(long): %v", err)
	}
	longIndexTTL, err := store.redis.TTL(ctx, indexKey).Result()
	if err != nil {
		t.Fatalf("long index TTL: %v", err)
	}
	if longIndexTTL <= longTTL || longIndexTTL > longTTL+authSessionIndexTTLBuffer {
		t.Fatalf("long index TTL = %s, want between session ttl and %s", longIndexTTL, longTTL+authSessionIndexTTLBuffer)
	}

	if err := store.CreateSession(ctx, authdomain.AuthSession{UserID: sessionTestUserID.String(), SessionID: "short", TokenVersion: 1}, shortTTL, defaultMaxActiveSessionsPerUser()); err != nil {
		t.Fatalf("CreateSession(short): %v", err)
	}
	afterShortIndexTTL, err := store.redis.TTL(ctx, indexKey).Result()
	if err != nil {
		t.Fatalf("after short index TTL: %v", err)
	}
	if afterShortIndexTTL <= shortTTL+authSessionIndexTTLBuffer {
		t.Fatalf("index TTL was shortened to %s after short session", afterShortIndexTTL)
	}
	if afterShortIndexTTL > longTTL+authSessionIndexTTLBuffer {
		t.Fatalf("index TTL = %s, want at most %s", afterShortIndexTTL, longTTL+authSessionIndexTTLBuffer)
	}
}

func TestSessionStoreKeysUseAppNamePrefixWithNewFormat(t *testing.T) {
	redisServer := miniredis.RunT(t)
	store := newTestSessionStoreWithAppName(t, redisServer, " aegiscore-user-services ")
	ctx := context.Background()
	session := authdomain.AuthSession{UserID: sessionTestUserID.String(), SessionID: "s-prefixed", TokenVersion: 7, ExpiresAt: time.Now().Add(time.Hour)}

	if err := store.CreateSession(ctx, session, time.Hour, defaultMaxActiveSessionsPerUser()); err != nil {
		t.Fatalf("CreateSession: %v", err)
	}
	if err := store.CacheTokenVersion(ctx, sessionTestUserID.String(), 7); err != nil {
		t.Fatalf("CacheTokenVersion: %v", err)
	}

	if !redisServer.Exists("aegiscore-user-services:auth:session:{" + sessionTestUserID.String() + "}:s-prefixed") {
		t.Fatal("prefixed new session key does not exist")
	}
	if !redisServer.Exists("aegiscore-user-services:auth:user:sessions:{" + sessionTestUserID.String() + "}") {
		t.Fatal("prefixed new user sessions key does not exist")
	}
	if !redisServer.Exists("aegiscore-user-services:auth:user:token_version:{" + sessionTestUserID.String() + "}") {
		t.Fatal("prefixed new token version key does not exist")
	}
	if redisServer.Exists("auth:session:{"+sessionTestUserID.String()+"}:s-prefixed") || redisServer.Exists("auth:user:sessions:{"+sessionTestUserID.String()+"}") || redisServer.Exists("auth:user:token_version:{"+sessionTestUserID.String()+"}") {
		t.Fatal("unprefixed Redis keys should not exist when app.name is set")
	}
}

func TestSessionStoreKeysRemainUnprefixedWhenAppNameEmpty(t *testing.T) {
	redisServer := miniredis.RunT(t)
	store := newTestSessionStoreWithAppName(t, redisServer, "   ")
	ctx := context.Background()
	session := authdomain.AuthSession{UserID: sessionTestUserID.String(), SessionID: "s-empty-prefix", TokenVersion: 7, ExpiresAt: time.Now().Add(time.Hour)}

	if err := store.CreateSession(ctx, session, time.Hour, defaultMaxActiveSessionsPerUser()); err != nil {
		t.Fatalf("CreateSession: %v", err)
	}
	if err := store.CacheTokenVersion(ctx, sessionTestUserID.String(), 7); err != nil {
		t.Fatalf("CacheTokenVersion: %v", err)
	}

	if !redisServer.Exists("auth:session:{" + sessionTestUserID.String() + "}:s-empty-prefix") {
		t.Fatal("unprefixed new session key does not exist")
	}
	if !redisServer.Exists("auth:user:sessions:{" + sessionTestUserID.String() + "}") {
		t.Fatal("unprefixed new user sessions key does not exist")
	}
	if !redisServer.Exists("auth:user:token_version:{" + sessionTestUserID.String() + "}") {
		t.Fatal("unprefixed new token version key does not exist")
	}
	if redisServer.Exists("aegiscore-user-services:auth:session:{"+sessionTestUserID.String()+"}:s-empty-prefix") || redisServer.Exists("aegiscore-user-services:auth:user:sessions:{"+sessionTestUserID.String()+"}") || redisServer.Exists("aegiscore-user-services:auth:user:token_version:{"+sessionTestUserID.String()+"}") {
		t.Fatal("default service-name Redis keys should not exist when app.name is empty")
	}
}

func TestSessionStoreIgnoresLegacyKeys(t *testing.T) {
	redisServer := miniredis.RunT(t)
	store := newTestSessionStore(redisServer)
	ctx := context.Background()
	legacySession := authdomain.AuthSession{UserID: sessionTestUserID.String(), SessionID: "s-legacy", TokenVersion: 7, ExpiresAt: time.Now().Add(time.Hour)}
	data, err := json.Marshal(legacySession)
	if err != nil {
		t.Fatalf("Marshal legacy session: %v", err)
	}
	if err := store.redis.Set(ctx, "auth:session:s-legacy", data, time.Hour).Err(); err != nil {
		t.Fatalf("Set legacy session: %v", err)
	}
	if err := store.redis.Set(ctx, "auth:user:"+sessionTestUserID.String()+":token_version", "7", time.Hour).Err(); err != nil {
		t.Fatalf("Set legacy token version: %v", err)
	}

	_, err = store.GetSession(ctx, sessionTestUserID.String(), "s-legacy")
	if !errors.Is(err, authdomain.ErrAuthSessionNotFound) {
		t.Fatalf("GetSession err = %v, want session not found", err)
	}
	_, err = store.GetCachedTokenVersion(ctx, sessionTestUserID.String())
	if !errors.Is(err, authdomain.ErrTokenVersionCacheMiss) {
		t.Fatalf("GetCachedTokenVersion err = %v, want cache miss", err)
	}
}

func newTestSessionStore(redisServer *miniredis.Miniredis) *SessionStore {
	return newTestSessionStoreWithConfig(redisServer, config.AuthConfig{TokenVersionCacheTTL: time.Minute})
}

func newTestSessionStoreWithConfig(redisServer *miniredis.Miniredis, authCfg config.AuthConfig) *SessionStore {
	client := rediscache.NewClient(&rediscache.Options{Addr: redisServer.Addr()})
	return &SessionStore{redis: client, keys: MustKeyCatalog(""), tokenVersionCacheTTL: authCfg.TokenVersionCacheTTL, purgePool: directPurgeTaskPool{}, metrics: authapplication.NopMetrics()}
}

func defaultMaxActiveSessionsPerUser() int {
	return 5
}

func newTestTokenVersionValidator(t testing.TB, users authapplication.UserTokenVersionStore, sessions authapplication.AuthSessionStore) commonauth.TokenVersionValidator {
	t.Helper()
	cache, err := localcache.New[string, int64](localcache.Config[string]{
		Name:        "auth_token_version_test",
		Capacity:    100,
		TTL:         time.Minute,
		LoadTimeout: time.Second,
		KeyString:   func(key string) string { return key },
	}, func(ctx context.Context, userID string) (int64, error) {
		return authvalidators.Current(ctx, users, sessions, userID)
	}, nil)
	if err != nil {
		t.Fatalf("New localcache: %v", err)
	}
	t.Cleanup(cache.Close)
	return authvalidators.NewCachingValidator(cache)
}

func newTestSessionStoreWithAppName(t testing.TB, redisServer *miniredis.Miniredis, appName string) *SessionStore {
	t.Helper()
	client := rediscache.NewClient(&rediscache.Options{Addr: redisServer.Addr()})
	store, err := NewSessionStore(SessionStoreParams{
		Redis: client,
		Cfg: &config.Config{
			App:  config.AppConfig{Name: appName},
			Auth: config.AuthConfig{TokenVersionCacheTTL: time.Minute},
		},
		PurgePool: directPurgeTaskPool{},
	})
	if err != nil {
		t.Fatalf("NewSessionStore: %v", err)
	}
	t.Cleanup(func() {
		_ = client.Close()
	})
	return store
}

func waitForRedisCondition(t *testing.T, condition func() bool, message string) {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if condition() {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal(message)
}

type tokenVersionRepositoryStub struct {
	version              int64
	newVersion           int64
	err                  error
	incrementErr         error
	gotUserID            uuid.UUID
	incrementedUserID    uuid.UUID
	getTokenVersionCalls int
	incrementCalls       int
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

type sessionStoreMetricsSpy struct {
	authapplication.Metrics
	sessionPurgeFailures int
}

func (m *sessionStoreMetricsSpy) SessionPurgeSubmitFailed(context.Context) {
	m.sessionPurgeFailures++
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

type lifecycleRecorder struct {
	hooks []fx.Hook
}

func (r *lifecycleRecorder) Append(hook fx.Hook) {
	r.hooks = append(r.hooks, hook)
}

func (r *tokenVersionRepositoryStub) GetTokenVersion(_ context.Context, userID uuid.UUID) (int64, error) {
	r.getTokenVersionCalls++
	r.gotUserID = userID
	if r.err != nil {
		return 0, r.err
	}
	return r.version, nil
}

func (r *tokenVersionRepositoryStub) IncrementTokenVersion(_ context.Context, userID uuid.UUID) (int64, error) {
	r.incrementCalls++
	r.incrementedUserID = userID
	if r.incrementErr != nil {
		return 0, r.incrementErr
	}
	return r.newVersion, nil
}
