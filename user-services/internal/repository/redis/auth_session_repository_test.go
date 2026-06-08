package redis

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/aegiscore/common/runtime/config"
	"github.com/aegiscore/user-services/internal/repository"
	"github.com/aegiscore/user-services/internal/service"
	"github.com/alicebob/miniredis/v2"
	"github.com/google/uuid"
	rediscache "github.com/redis/go-redis/v9"
)

var sessionTestUserID = uuid.MustParse("018f6f3e-7c4d-7b2a-9f8a-4f6b1b2c3d4e")

func TestAuthSessionRepositoryTokenVersionCacheMiss(t *testing.T) {
	redisServer := miniredis.RunT(t)
	store := newTestAuthSessionRepository(redisServer)

	_, err := store.GetCachedTokenVersion(context.Background(), sessionTestUserID.String())
	if !errors.Is(err, repository.ErrTokenVersionCacheMiss) {
		t.Fatalf("GetCachedTokenVersion err = %v, want cache miss", err)
	}
	if redisServer.Exists(store.tokenVersionKey(sessionTestUserID.String())) {
		t.Fatal("cache miss should not create token version key")
	}
}

func TestAuthSessionRepositoryCachesAndGetsTokenVersion(t *testing.T) {
	redisServer := miniredis.RunT(t)
	store := newTestAuthSessionRepository(redisServer)

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

func TestAuthSessionRepositoryCacheTokenVersionOverwritesStaleValue(t *testing.T) {
	redisServer := miniredis.RunT(t)
	store := newTestAuthSessionRepository(redisServer)
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

func TestAuthSessionRepositoryTokenVersionCacheUsesDefaultTTL(t *testing.T) {
	redisServer := miniredis.RunT(t)
	store := newTestAuthSessionRepositoryWithConfig(redisServer, config.AuthConfig{})

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

func TestAuthSessionRepositoryTokenVersionCacheUsesExplicitTTL(t *testing.T) {
	redisServer := miniredis.RunT(t)
	explicitTTL := time.Minute
	store := newTestAuthSessionRepositoryWithConfig(redisServer, config.AuthConfig{TokenVersionCacheTTL: explicitTTL})

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

func TestAuthSessionRepositoryTokenVersionInvalidCacheReportsMiss(t *testing.T) {
	redisServer := miniredis.RunT(t)
	store := newTestAuthSessionRepository(redisServer)
	ctx := context.Background()
	key := store.tokenVersionKey(sessionTestUserID.String())

	for _, value := range []string{"not-an-int", "0"} {
		if err := store.redis.Set(ctx, key, value, time.Minute).Err(); err != nil {
			t.Fatalf("Set token version cache: %v", err)
		}
		_, err := store.GetCachedTokenVersion(ctx, sessionTestUserID.String())
		if !errors.Is(err, repository.ErrTokenVersionCacheMiss) {
			t.Fatalf("GetCachedTokenVersion(%q) err = %v, want cache miss", value, err)
		}
	}
}

func TestAuthSessionRepositoryCreateGetAndDeleteSession(t *testing.T) {
	redisServer := miniredis.RunT(t)
	store := newTestAuthSessionRepository(redisServer)
	ctx := context.Background()
	indexKey := store.userSessionsKey(sessionTestUserID.String())
	ttl := time.Hour
	mismatchedExpiresAt := time.Now().Add(24 * time.Hour).Truncate(time.Second)
	session := repository.AuthSession{UserID: sessionTestUserID.String(), SessionID: "s-123", TokenVersion: 1, ExpiresAt: mismatchedExpiresAt}
	if err := store.redis.ZAdd(ctx, indexKey, rediscache.Z{Score: float64(time.Now().Add(-time.Minute).Unix()), Member: "expired-session"}).Err(); err != nil {
		t.Fatalf("ZAdd expired session: %v", err)
	}

	beforeCreate := time.Now()
	if err := store.CreateSession(ctx, session, ttl); err != nil {
		t.Fatalf("CreateSession: %v", err)
	}
	afterCreate := time.Now()
	stored, err := store.GetSession(ctx, "s-123")
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
	sessionTTL, err := store.redis.TTL(ctx, store.sessionKey("s-123")).Result()
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
	if redisServer.Exists(store.sessionKey("s-123")) {
		t.Fatal("session key still exists")
	}
	if _, err := store.redis.ZScore(ctx, indexKey, "s-123").Result(); !errors.Is(err, rediscache.Nil) {
		t.Fatalf("deleted session ZScore err = %v, want redis.Nil", err)
	}
	if _, err := store.redis.ZScore(ctx, indexKey, "expired-on-delete").Result(); !errors.Is(err, rediscache.Nil) {
		t.Fatalf("expired-on-delete ZScore err = %v, want redis.Nil", err)
	}
}

func TestAuthSessionRepositoryDeleteAllUserSessions(t *testing.T) {
	redisServer := miniredis.RunT(t)
	store := newTestAuthSessionRepository(redisServer)
	ctx := context.Background()
	indexKey := store.userSessionsKey(sessionTestUserID.String())
	for _, sessionID := range []string{"s-1", "s-2"} {
		if err := store.CreateSession(ctx, repository.AuthSession{UserID: sessionTestUserID.String(), SessionID: sessionID, TokenVersion: 1, ExpiresAt: time.Now().Add(time.Hour)}, time.Hour); err != nil {
			t.Fatalf("CreateSession: %v", err)
		}
	}
	if err := store.redis.Set(ctx, store.sessionKey("expired-session"), "stale", 0).Err(); err != nil {
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
	if redisServer.Exists(store.sessionKey("s-1")) || redisServer.Exists(store.sessionKey("s-2")) || redisServer.Exists(indexKey) {
		t.Fatal("user sessions were not fully deleted")
	}
	if !redisServer.Exists(store.sessionKey("expired-session")) {
		t.Fatal("expired session key was deleted despite expired index member cleanup")
	}
}

func TestAuthSessionRepositoryUserSessionsIndexTTLIsNotShortened(t *testing.T) {
	redisServer := miniredis.RunT(t)
	store := newTestAuthSessionRepository(redisServer)
	ctx := context.Background()
	indexKey := store.userSessionsKey(sessionTestUserID.String())
	longTTL := 2 * time.Hour
	shortTTL := time.Hour

	if err := store.CreateSession(ctx, repository.AuthSession{UserID: sessionTestUserID.String(), SessionID: "long", TokenVersion: 1}, longTTL); err != nil {
		t.Fatalf("CreateSession(long): %v", err)
	}
	longIndexTTL, err := store.redis.TTL(ctx, indexKey).Result()
	if err != nil {
		t.Fatalf("long index TTL: %v", err)
	}
	if longIndexTTL <= longTTL || longIndexTTL > longTTL+authSessionIndexTTLBuffer {
		t.Fatalf("long index TTL = %s, want between session ttl and %s", longIndexTTL, longTTL+authSessionIndexTTLBuffer)
	}

	if err := store.CreateSession(ctx, repository.AuthSession{UserID: sessionTestUserID.String(), SessionID: "short", TokenVersion: 1}, shortTTL); err != nil {
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

func TestAuthSessionRepositoryKeysUseAppNamePrefix(t *testing.T) {
	redisServer := miniredis.RunT(t)
	store := newTestAuthSessionRepositoryWithAppName(redisServer, " aegiscore-user-services ")
	ctx := context.Background()
	session := repository.AuthSession{UserID: sessionTestUserID.String(), SessionID: "s-prefixed", TokenVersion: 7, ExpiresAt: time.Now().Add(time.Hour)}

	if err := store.CreateSession(ctx, session, time.Hour); err != nil {
		t.Fatalf("CreateSession: %v", err)
	}
	if err := store.CacheTokenVersion(ctx, sessionTestUserID.String(), 7); err != nil {
		t.Fatalf("CacheTokenVersion: %v", err)
	}

	if !redisServer.Exists("aegiscore-user-services:auth:session:s-prefixed") {
		t.Fatal("prefixed session key does not exist")
	}
	if !redisServer.Exists("aegiscore-user-services:auth:user:" + sessionTestUserID.String() + ":sessions") {
		t.Fatal("prefixed user sessions key does not exist")
	}
	if !redisServer.Exists("aegiscore-user-services:auth:user:" + sessionTestUserID.String() + ":token_version") {
		t.Fatal("prefixed token version key does not exist")
	}
	if redisServer.Exists("auth:session:s-prefixed") || redisServer.Exists("auth:user:"+sessionTestUserID.String()+":sessions") || redisServer.Exists("auth:user:"+sessionTestUserID.String()+":token_version") {
		t.Fatal("unprefixed Redis keys should not exist when app.name is set")
	}
}

func TestAuthSessionRepositoryKeysRemainUnprefixedWhenAppNameEmpty(t *testing.T) {
	redisServer := miniredis.RunT(t)
	store := newTestAuthSessionRepositoryWithAppName(redisServer, "   ")
	ctx := context.Background()
	session := repository.AuthSession{UserID: sessionTestUserID.String(), SessionID: "s-empty-prefix", TokenVersion: 7, ExpiresAt: time.Now().Add(time.Hour)}

	if err := store.CreateSession(ctx, session, time.Hour); err != nil {
		t.Fatalf("CreateSession: %v", err)
	}
	if err := store.CacheTokenVersion(ctx, sessionTestUserID.String(), 7); err != nil {
		t.Fatalf("CacheTokenVersion: %v", err)
	}

	if !redisServer.Exists("auth:session:s-empty-prefix") {
		t.Fatal("unprefixed session key does not exist")
	}
	if !redisServer.Exists("auth:user:" + sessionTestUserID.String() + ":sessions") {
		t.Fatal("unprefixed user sessions key does not exist")
	}
	if !redisServer.Exists("auth:user:" + sessionTestUserID.String() + ":token_version") {
		t.Fatal("unprefixed token version key does not exist")
	}
	if redisServer.Exists("aegiscore-user-services:auth:session:s-empty-prefix") || redisServer.Exists("aegiscore-user-services:auth:user:"+sessionTestUserID.String()+":sessions") || redisServer.Exists("aegiscore-user-services:auth:user:"+sessionTestUserID.String()+":token_version") {
		t.Fatal("default service-name Redis keys should not exist when app.name is empty")
	}
}

func newTestAuthSessionRepository(redisServer *miniredis.Miniredis) *authSessionRepository {
	return newTestAuthSessionRepositoryWithConfig(redisServer, config.AuthConfig{TokenVersionCacheTTL: time.Minute})
}

func newTestAuthSessionRepositoryWithConfig(redisServer *miniredis.Miniredis, authCfg config.AuthConfig) *authSessionRepository {
	client := rediscache.NewClient(&rediscache.Options{Addr: redisServer.Addr()})
	return &authSessionRepository{redis: client, keys: service.NewRedisKeyBuilder(&config.Config{}), tokenVersionCacheTTL: authCfg.TokenVersionCacheTTL}
}

func newTestAuthSessionRepositoryWithAppName(redisServer *miniredis.Miniredis, appName string) *authSessionRepository {
	client := rediscache.NewClient(&rediscache.Options{Addr: redisServer.Addr()})
	store := NewAuthSessionRepository(AuthSessionRepositoryParams{
		Redis: client,
		Keys:  service.NewRedisKeyBuilder(&config.Config{App: config.AppConfig{Name: appName}}),
		Cfg: &config.Config{
			App:  config.AppConfig{Name: appName},
			Auth: config.AuthConfig{TokenVersionCacheTTL: time.Minute},
		},
	})
	return store.(*authSessionRepository)
}
