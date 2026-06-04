package redis

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/aegiscore/common/config"
	"github.com/aegiscore/user-services/internal/domain"
	"github.com/aegiscore/user-services/internal/repository"
	"github.com/alicebob/miniredis/v2"
	"github.com/google/uuid"
	rediscache "github.com/redis/go-redis/v9"
)

var sessionTestUserID = uuid.MustParse("018f6f3e-7c4d-7b2a-9f8a-4f6b1b2c3d4e")

func TestAuthSessionRepositoryTokenVersionCacheMissReadsRepository(t *testing.T) {
	redisServer := miniredis.RunT(t)
	store := newTestAuthSessionRepository(redisServer, &tokenVersionRepoStub{version: 7})

	version, err := store.GetCurrentTokenVersion(context.Background(), sessionTestUserID.String())
	if err != nil {
		t.Fatalf("GetCurrentTokenVersion: %v", err)
	}
	if version != 7 {
		t.Fatalf("version = %d, want 7", version)
	}
	got, err := redisServer.Get(tokenVersionKey(sessionTestUserID.String()))
	if err != nil {
		t.Fatalf("Get cached token version: %v", err)
	}
	if got != "7" {
		t.Fatalf("cached token version = %q, want 7", got)
	}
}

func TestAuthSessionRepositoryTokenVersionCacheUsesDefaultTTL(t *testing.T) {
	redisServer := miniredis.RunT(t)
	store := newTestAuthSessionRepositoryWithConfig(redisServer, &tokenVersionRepoStub{version: 7}, config.AuthConfig{})

	_, err := store.GetCurrentTokenVersion(context.Background(), sessionTestUserID.String())

	if err != nil {
		t.Fatalf("GetCurrentTokenVersion: %v", err)
	}
	ttl, err := store.redis.TTL(context.Background(), tokenVersionKey(sessionTestUserID.String())).Result()
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
	store := newTestAuthSessionRepositoryWithConfig(redisServer, &tokenVersionRepoStub{version: 7}, config.AuthConfig{TokenVersionCacheTTL: explicitTTL})

	_, err := store.GetCurrentTokenVersion(context.Background(), sessionTestUserID.String())

	if err != nil {
		t.Fatalf("GetCurrentTokenVersion: %v", err)
	}
	ttl, err := store.redis.TTL(context.Background(), tokenVersionKey(sessionTestUserID.String())).Result()
	if err != nil {
		t.Fatalf("TTL: %v", err)
	}
	if ttl <= 0 || ttl > explicitTTL {
		t.Fatalf("TTL = %s, want within explicit %s", ttl, explicitTTL)
	}
}

func TestAuthSessionRepositoryTokenVersionInvalidCacheReadsRepository(t *testing.T) {
	redisServer := miniredis.RunT(t)
	repo := &tokenVersionRepoStub{version: 9}
	store := newTestAuthSessionRepository(redisServer, repo)
	ctx := context.Background()
	key := tokenVersionKey(sessionTestUserID.String())

	for _, value := range []string{"not-an-int", "0"} {
		if err := store.redis.Set(ctx, key, value, time.Minute).Err(); err != nil {
			t.Fatalf("Set token version cache: %v", err)
		}
		version, err := store.GetCurrentTokenVersion(ctx, sessionTestUserID.String())
		if err != nil {
			t.Fatalf("GetCurrentTokenVersion(%q): %v", value, err)
		}
		if version != repo.version {
			t.Fatalf("version = %d, want %d", version, repo.version)
		}
	}
	if repo.getTokenVersionCalls != 2 {
		t.Fatalf("GetTokenVersion calls = %d, want 2", repo.getTokenVersionCalls)
	}
}

func TestAuthSessionRepositoryCreateGetAndDeleteSession(t *testing.T) {
	redisServer := miniredis.RunT(t)
	store := newTestAuthSessionRepository(redisServer, &tokenVersionRepoStub{version: 1})
	ctx := context.Background()
	indexKey := userSessionsKey(sessionTestUserID.String())
	expiresAt := time.Now().Add(time.Hour).Truncate(time.Second)
	session := repository.AuthSession{UserID: sessionTestUserID.String(), SessionID: "s-123", TokenVersion: 1, ExpiresAt: expiresAt}
	if err := store.redis.ZAdd(ctx, indexKey, rediscache.Z{Score: float64(time.Now().Add(-time.Minute).Unix()), Member: "expired-session"}).Err(); err != nil {
		t.Fatalf("ZAdd expired session: %v", err)
	}

	if err := store.CreateSession(ctx, session, time.Hour); err != nil {
		t.Fatalf("CreateSession: %v", err)
	}
	stored, err := store.GetSession(ctx, "s-123")
	if err != nil {
		t.Fatalf("GetSession: %v", err)
	}
	if stored.UserID != sessionTestUserID.String() || stored.SessionID != "s-123" || stored.TokenVersion != 1 {
		t.Fatalf("stored = %#v", stored)
	}
	score, err := store.redis.ZScore(ctx, indexKey, "s-123").Result()
	if err != nil {
		t.Fatalf("ZScore: %v", err)
	}
	if int64(score) != expiresAt.Unix() {
		t.Fatalf("ZScore = %d, want %d", int64(score), expiresAt.Unix())
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
	if redisServer.Exists(sessionKey("s-123")) {
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
	store := newTestAuthSessionRepository(redisServer, &tokenVersionRepoStub{version: 1})
	ctx := context.Background()
	indexKey := userSessionsKey(sessionTestUserID.String())
	for _, sessionID := range []string{"s-1", "s-2"} {
		if err := store.CreateSession(ctx, repository.AuthSession{UserID: sessionTestUserID.String(), SessionID: sessionID, TokenVersion: 1, ExpiresAt: time.Now().Add(time.Hour)}, time.Hour); err != nil {
			t.Fatalf("CreateSession: %v", err)
		}
	}
	if err := store.redis.Set(ctx, sessionKey("expired-session"), "stale", 0).Err(); err != nil {
		t.Fatalf("Set expired session: %v", err)
	}
	if err := store.redis.ZAdd(ctx, indexKey, rediscache.Z{Score: float64(time.Now().Add(-time.Minute).Unix()), Member: "expired-session"}).Err(); err != nil {
		t.Fatalf("ZAdd expired session: %v", err)
	}
	if err := store.DeleteAllUserSessions(ctx, sessionTestUserID.String()); err != nil {
		t.Fatalf("DeleteAllUserSessions: %v", err)
	}
	if redisServer.Exists(sessionKey("s-1")) || redisServer.Exists(sessionKey("s-2")) || redisServer.Exists(indexKey) {
		t.Fatal("user sessions were not fully deleted")
	}
	if !redisServer.Exists(sessionKey("expired-session")) {
		t.Fatal("expired session key was deleted despite expired index member cleanup")
	}
}

func newTestAuthSessionRepository(redisServer *miniredis.Miniredis, repo repository.UserRepository) *authSessionRepository {
	return newTestAuthSessionRepositoryWithConfig(redisServer, repo, config.AuthConfig{TokenVersionCacheTTL: time.Minute})
}

func newTestAuthSessionRepositoryWithConfig(redisServer *miniredis.Miniredis, repo repository.UserRepository, authCfg config.AuthConfig) *authSessionRepository {
	client := rediscache.NewClient(&rediscache.Options{Addr: redisServer.Addr()})
	return &authSessionRepository{redis: client, repo: repo, tokenVersionCacheTTL: authCfg.TokenVersionCacheTTL}
}

type tokenVersionRepoStub struct {
	version              int64
	getTokenVersionCalls int
}

func (r *tokenVersionRepoStub) Create(context.Context, repository.CreateUserInput) (*domain.User, error) {
	return nil, nil
}
func (r *tokenVersionRepoStub) ExistsByUsername(context.Context, string) (bool, error) {
	return false, nil
}
func (r *tokenVersionRepoStub) GetByUsername(context.Context, string) (*domain.User, error) {
	return nil, nil
}
func (r *tokenVersionRepoStub) GetByUserID(context.Context, uuid.UUID) (*domain.User, error) {
	return nil, nil
}
func (r *tokenVersionRepoStub) GetTokenVersion(context.Context, uuid.UUID) (int64, error) {
	r.getTokenVersionCalls++
	return r.version, nil
}
func (r *tokenVersionRepoStub) IncrementTokenVersion(context.Context, uuid.UUID) (int64, error) {
	return r.version + 1, nil
}
func (r *tokenVersionRepoStub) UpdateCredentials(context.Context, repository.UpdateCredentialsInput) (int64, error) {
	return r.version + 1, nil
}
func (r *tokenVersionRepoStub) ListUsers(context.Context, repository.ListUsersInput) ([]domain.User, int, error) {
	return nil, 0, nil
}
