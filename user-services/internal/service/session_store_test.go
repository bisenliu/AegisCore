package service

import (
	"context"
	"testing"
	"time"

	"github.com/aegiscore/common/config"
	"github.com/aegiscore/user-services/ent"
	"github.com/aegiscore/user-services/internal/domain"
	"github.com/aegiscore/user-services/internal/repository"
	"github.com/alicebob/miniredis/v2"
	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
)

var sessionTestUserID = uuid.MustParse("018f6f3e-7c4d-7b2a-9f8a-4f6b1b2c3d4e")

func TestSessionStoreTokenVersionCacheMissReadsRepository(t *testing.T) {
	redisServer := miniredis.RunT(t)
	store := newTestSessionStore(redisServer, &tokenVersionRepoStub{version: 7})

	version, err := store.GetCurrentTokenVersion(context.Background(), sessionTestUserID.String())
	if err != nil {
		t.Fatalf("GetCurrentTokenVersion: %v", err)
	}
	if version != 7 {
		t.Fatalf("version = %d, want 7", version)
	}
	got, err := redisServer.Get("auth:user:" + sessionTestUserID.String() + ":token_version")
	if err != nil {
		t.Fatalf("Get cached token version: %v", err)
	}
	if got != "7" {
		t.Fatalf("cached token version = %q, want 7", got)
	}
}

func TestSessionStoreTokenVersionCacheUsesDefaultTTL(t *testing.T) {
	redisServer := miniredis.RunT(t)
	store := newTestSessionStoreWithConfig(redisServer, &tokenVersionRepoStub{version: 7}, config.AuthConfig{})

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

func TestSessionStoreTokenVersionCacheUsesExplicitTTL(t *testing.T) {
	redisServer := miniredis.RunT(t)
	explicitTTL := time.Minute
	store := newTestSessionStoreWithConfig(redisServer, &tokenVersionRepoStub{version: 7}, config.AuthConfig{TokenVersionCacheTTL: explicitTTL})

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

func TestSessionStoreCreateGetAndDeleteSession(t *testing.T) {
	redisServer := miniredis.RunT(t)
	store := newTestSessionStore(redisServer, &tokenVersionRepoStub{version: 1})
	ctx := context.Background()
	session := Session{UserID: sessionTestUserID.String(), SessionID: "s-123", TokenVersion: 1, ExpiresAt: time.Now().Add(time.Hour)}

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
	indexKey := "auth:user:" + sessionTestUserID.String() + ":sessions"
	isMember, err := redisServer.SIsMember(indexKey, "s-123")
	if err != nil {
		t.Fatalf("SIsMember: %v", err)
	}
	if !isMember {
		t.Fatal("session index missing s-123")
	}

	if err := store.DeleteSession(ctx, sessionTestUserID.String(), "s-123"); err != nil {
		t.Fatalf("DeleteSession: %v", err)
	}
	if redisServer.Exists("auth:session:s-123") {
		t.Fatal("session key still exists")
	}
	isMember, err = redisServer.SIsMember(indexKey, "s-123")
	if err != nil && err.Error() != "ERR no such key" {
		t.Fatalf("SIsMember: %v", err)
	}
	if isMember {
		t.Fatal("session index still contains s-123")
	}
}

func TestSessionStoreDeleteAllUserSessions(t *testing.T) {
	redisServer := miniredis.RunT(t)
	store := newTestSessionStore(redisServer, &tokenVersionRepoStub{version: 1})
	ctx := context.Background()
	for _, sessionID := range []string{"s-1", "s-2"} {
		if err := store.CreateSession(ctx, Session{UserID: sessionTestUserID.String(), SessionID: sessionID, TokenVersion: 1, ExpiresAt: time.Now().Add(time.Hour)}, time.Hour); err != nil {
			t.Fatalf("CreateSession: %v", err)
		}
	}
	if err := store.DeleteAllUserSessions(ctx, sessionTestUserID.String()); err != nil {
		t.Fatalf("DeleteAllUserSessions: %v", err)
	}
	if redisServer.Exists("auth:session:s-1") || redisServer.Exists("auth:session:s-2") || redisServer.Exists("auth:user:"+sessionTestUserID.String()+":sessions") {
		t.Fatal("user sessions were not fully deleted")
	}
}

func newTestSessionStore(redisServer *miniredis.Miniredis, repo repository.UserRepository) *redisSessionStore {
	return newTestSessionStoreWithConfig(redisServer, repo, config.AuthConfig{TokenVersionCacheTTL: time.Minute})
}

func newTestSessionStoreWithConfig(redisServer *miniredis.Miniredis, repo repository.UserRepository, authCfg config.AuthConfig) *redisSessionStore {
	client := redis.NewClient(&redis.Options{Addr: redisServer.Addr()})
	return &redisSessionStore{redis: client, repo: repo, tokenVersionCacheTTL: authCfg.TokenVersionCacheTTL}
}

type tokenVersionRepoStub struct {
	version int64
}

func (r *tokenVersionRepoStub) Create(context.Context, repository.CreateUserInput) (*ent.User, error) {
	return nil, nil
}
func (r *tokenVersionRepoStub) ExistsByUsername(context.Context, string) (bool, error) {
	return false, nil
}
func (r *tokenVersionRepoStub) GetByUsername(context.Context, string) (*ent.User, error) {
	return nil, nil
}
func (r *tokenVersionRepoStub) GetByUserID(context.Context, uuid.UUID) (*ent.User, error) {
	return nil, nil
}
func (r *tokenVersionRepoStub) GetTokenVersion(context.Context, uuid.UUID) (int64, error) {
	return r.version, nil
}
func (r *tokenVersionRepoStub) IncrementTokenVersion(context.Context, uuid.UUID) (int64, error) {
	return r.version + 1, nil
}
func (r *tokenVersionRepoStub) UpdatePasswordHashAndStatus(context.Context, uuid.UUID, string, domain.UserStatus) (int64, error) {
	return r.version + 1, nil
}
func (r *tokenVersionRepoStub) ListUsers(context.Context, repository.ListUsersInput) ([]*ent.User, int, error) {
	return nil, 0, nil
}
