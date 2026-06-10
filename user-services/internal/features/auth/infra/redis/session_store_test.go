package redis

import (
	"context"
	"encoding/json"
	"errors"
	"strconv"
	"sync"
	"testing"
	"time"

	"github.com/aegiscore/common/runtime/config"
	authapp "github.com/aegiscore/user-services/internal/features/auth/app"
	authdomain "github.com/aegiscore/user-services/internal/features/auth/domain"
	"github.com/alicebob/miniredis/v2"
	"github.com/google/uuid"
	rediscache "github.com/redis/go-redis/v9"
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
	redisServer := miniredis.RunT(t)
	store := newTestSessionStoreWithConfig(redisServer, config.AuthConfig{})

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
	validator := authapp.NewTokenVersionValidator(users, store)
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
	validator := authapp.NewTokenVersionValidator(users, store)

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
	validator := authapp.NewTokenVersionValidator(users, store)

	err := validator.ValidateTokenVersion(ctx, sessionTestUserID.String(), 8)

	if !errors.Is(err, authdomain.ErrTokenVersionMismatch) {
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
	if err := store.CreateSession(ctx, authdomain.AuthSession{UserID: sessionTestUserID.String(), SessionID: "s-stale", TokenVersion: 5}, time.Hour); err != nil {
		t.Fatalf("CreateSession: %v", err)
	}
	if err := store.CacheTokenVersion(ctx, sessionTestUserID.String(), 6); err != nil {
		t.Fatalf("CacheTokenVersion refreshed: %v", err)
	}
	if err := store.DeleteAllUserSessions(ctx, sessionTestUserID.String()); err != nil {
		t.Fatalf("DeleteAllUserSessions: %v", err)
	}
	validator := authapp.NewTokenVersionValidator(&tokenVersionRepositoryStub{err: errors.New("database should not be read")}, store)

	err := validator.ValidateTokenVersion(ctx, sessionTestUserID.String(), 5)

	if !errors.Is(err, authdomain.ErrTokenVersionMismatch) {
		t.Fatalf("err = %v, want token version mismatch", err)
	}
	version, err := store.GetCachedTokenVersion(ctx, sessionTestUserID.String())
	if err != nil {
		t.Fatalf("GetCachedTokenVersion: %v", err)
	}
	if version != 6 {
		t.Fatalf("cached version = %d, want 6", version)
	}
	if redisServer.Exists(store.sessionKey(sessionTestUserID.String(), "s-stale")) || redisServer.Exists(store.userSessionsKey(sessionTestUserID.String())) {
		t.Fatal("user sessions were not deleted during cache refresh flow")
	}
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
	if err := store.CreateSession(ctx, session, ttl); err != nil {
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

func TestSessionStoreRotateSession(t *testing.T) {
	redisServer := miniredis.RunT(t)
	store := newTestSessionStore(redisServer)
	ctx := context.Background()
	indexKey := store.userSessionsKey(sessionTestUserID.String())
	ttl := time.Hour
	oldSession := authdomain.AuthSession{UserID: sessionTestUserID.String(), SessionID: "s-old", TokenVersion: 1}
	newSession := authdomain.AuthSession{UserID: sessionTestUserID.String(), SessionID: "s-new", TokenVersion: 1, ExpiresAt: time.Now().Add(24 * time.Hour)}
	if err := store.CreateSession(ctx, oldSession, ttl); err != nil {
		t.Fatalf("CreateSession: %v", err)
	}
	if err := store.redis.ZAdd(ctx, indexKey, rediscache.Z{Score: float64(time.Now().Add(-time.Minute).Unix()), Member: "expired-session"}).Err(); err != nil {
		t.Fatalf("ZAdd expired session: %v", err)
	}

	beforeRotate := time.Now()
	if err := store.RotateSession(ctx, oldSession, newSession, ttl); err != nil {
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

func TestSessionStoreRotateSessionRejectsMissingOldSession(t *testing.T) {
	redisServer := miniredis.RunT(t)
	store := newTestSessionStore(redisServer)
	ctx := context.Background()
	oldSession := authdomain.AuthSession{UserID: sessionTestUserID.String(), SessionID: "s-missing", TokenVersion: 1}
	newSession := authdomain.AuthSession{UserID: sessionTestUserID.String(), SessionID: "s-new", TokenVersion: 1}

	err := store.RotateSession(ctx, oldSession, newSession, time.Hour)

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
	if err := store.CreateSession(ctx, storedOldSession, time.Hour); err != nil {
		t.Fatalf("CreateSession: %v", err)
	}
	oldSession := authdomain.AuthSession{UserID: sessionTestUserID.String(), SessionID: "s-old", TokenVersion: 2}
	newSession := authdomain.AuthSession{UserID: sessionTestUserID.String(), SessionID: "s-new", TokenVersion: 2}

	err := store.RotateSession(ctx, oldSession, newSession, time.Hour)

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
	if err := store.CreateSession(ctx, oldSession, time.Hour); err != nil {
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
			results <- store.RotateSession(ctx, oldSession, newSession, time.Hour)
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
		if err := store.CreateSession(ctx, authdomain.AuthSession{UserID: sessionTestUserID.String(), SessionID: sessionID, TokenVersion: 1, ExpiresAt: time.Now().Add(time.Hour)}, time.Hour); err != nil {
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
	if redisServer.Exists(store.sessionKey(sessionTestUserID.String(), "s-1")) || redisServer.Exists(store.sessionKey(sessionTestUserID.String(), "s-2")) || redisServer.Exists(indexKey) {
		t.Fatal("user sessions were not fully deleted")
	}
	if !redisServer.Exists(store.sessionKey(sessionTestUserID.String(), "expired-session")) {
		t.Fatal("expired session key was deleted despite expired index member cleanup")
	}
}

func TestSessionStoreUserSessionsIndexTTLIsNotShortened(t *testing.T) {
	redisServer := miniredis.RunT(t)
	store := newTestSessionStore(redisServer)
	ctx := context.Background()
	indexKey := store.userSessionsKey(sessionTestUserID.String())
	longTTL := 2 * time.Hour
	shortTTL := time.Hour

	if err := store.CreateSession(ctx, authdomain.AuthSession{UserID: sessionTestUserID.String(), SessionID: "long", TokenVersion: 1}, longTTL); err != nil {
		t.Fatalf("CreateSession(long): %v", err)
	}
	longIndexTTL, err := store.redis.TTL(ctx, indexKey).Result()
	if err != nil {
		t.Fatalf("long index TTL: %v", err)
	}
	if longIndexTTL <= longTTL || longIndexTTL > longTTL+authSessionIndexTTLBuffer {
		t.Fatalf("long index TTL = %s, want between session ttl and %s", longIndexTTL, longTTL+authSessionIndexTTLBuffer)
	}

	if err := store.CreateSession(ctx, authdomain.AuthSession{UserID: sessionTestUserID.String(), SessionID: "short", TokenVersion: 1}, shortTTL); err != nil {
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
	store := newTestSessionStoreWithAppName(redisServer, " aegiscore-user-services ")
	ctx := context.Background()
	session := authdomain.AuthSession{UserID: sessionTestUserID.String(), SessionID: "s-prefixed", TokenVersion: 7, ExpiresAt: time.Now().Add(time.Hour)}

	if err := store.CreateSession(ctx, session, time.Hour); err != nil {
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
	store := newTestSessionStoreWithAppName(redisServer, "   ")
	ctx := context.Background()
	session := authdomain.AuthSession{UserID: sessionTestUserID.String(), SessionID: "s-empty-prefix", TokenVersion: 7, ExpiresAt: time.Now().Add(time.Hour)}

	if err := store.CreateSession(ctx, session, time.Hour); err != nil {
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

func newTestSessionStore(redisServer *miniredis.Miniredis) *sessionStore {
	return newTestSessionStoreWithConfig(redisServer, config.AuthConfig{TokenVersionCacheTTL: time.Minute})
}

func newTestSessionStoreWithConfig(redisServer *miniredis.Miniredis, authCfg config.AuthConfig) *sessionStore {
	client := rediscache.NewClient(&rediscache.Options{Addr: redisServer.Addr()})
	return &sessionStore{redis: client, keys: authdomain.NewRedisKeyBuilder(&config.Config{}), tokenVersionCacheTTL: authCfg.TokenVersionCacheTTL}
}

func newTestSessionStoreWithAppName(redisServer *miniredis.Miniredis, appName string) *sessionStore {
	client := rediscache.NewClient(&rediscache.Options{Addr: redisServer.Addr()})
	store := NewSessionStore(SessionStoreParams{
		Redis: client,
		Keys:  authdomain.NewRedisKeyBuilder(&config.Config{App: config.AppConfig{Name: appName}}),
		Cfg: &config.Config{
			App:  config.AppConfig{Name: appName},
			Auth: config.AuthConfig{TokenVersionCacheTTL: time.Minute},
		},
	})
	return store
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
