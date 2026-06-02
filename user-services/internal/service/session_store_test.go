package service

import (
	"context"
	"testing"
	"time"

	"github.com/aegiscore/user-services/ent"
	"github.com/aegiscore/user-services/internal/repository"
	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
)

func TestSessionStoreTokenVersionCacheMissReadsRepository(t *testing.T) {
	redisServer := miniredis.RunT(t)
	store := newTestSessionStore(redisServer, &tokenVersionRepoStub{version: 7})

	version, err := store.GetCurrentTokenVersion(context.Background(), 123)
	if err != nil {
		t.Fatalf("GetCurrentTokenVersion: %v", err)
	}
	if version != 7 {
		t.Fatalf("version = %d, want 7", version)
	}
	got, err := redisServer.Get("auth:user:123:token_version")
	if err != nil {
		t.Fatalf("Get cached token version: %v", err)
	}
	if got != "7" {
		t.Fatalf("cached token version = %q, want 7", got)
	}
}

func TestSessionStoreCreateGetAndDeleteSession(t *testing.T) {
	redisServer := miniredis.RunT(t)
	store := newTestSessionStore(redisServer, &tokenVersionRepoStub{version: 1})
	ctx := context.Background()
	session := Session{UserID: 123, SessionID: "s-123", TokenVersion: 1, ExpiresAt: time.Now().Add(time.Hour)}

	if err := store.CreateSession(ctx, session, time.Hour); err != nil {
		t.Fatalf("CreateSession: %v", err)
	}
	stored, err := store.GetSession(ctx, "s-123")
	if err != nil {
		t.Fatalf("GetSession: %v", err)
	}
	if stored.UserID != 123 || stored.SessionID != "s-123" || stored.TokenVersion != 1 {
		t.Fatalf("stored = %#v", stored)
	}
	isMember, err := redisServer.SIsMember("auth:user:123:sessions", "s-123")
	if err != nil {
		t.Fatalf("SIsMember: %v", err)
	}
	if !isMember {
		t.Fatal("session index missing s-123")
	}

	if err := store.DeleteSession(ctx, 123, "s-123"); err != nil {
		t.Fatalf("DeleteSession: %v", err)
	}
	if redisServer.Exists("auth:session:s-123") {
		t.Fatal("session key still exists")
	}
	isMember, err = redisServer.SIsMember("auth:user:123:sessions", "s-123")
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
		if err := store.CreateSession(ctx, Session{UserID: 123, SessionID: sessionID, TokenVersion: 1, ExpiresAt: time.Now().Add(time.Hour)}, time.Hour); err != nil {
			t.Fatalf("CreateSession: %v", err)
		}
	}
	if err := store.DeleteAllUserSessions(ctx, 123); err != nil {
		t.Fatalf("DeleteAllUserSessions: %v", err)
	}
	if redisServer.Exists("auth:session:s-1") || redisServer.Exists("auth:session:s-2") || redisServer.Exists("auth:user:123:sessions") {
		t.Fatal("user sessions were not fully deleted")
	}
}

func newTestSessionStore(redisServer *miniredis.Miniredis, repo repository.UserRepository) *redisSessionStore {
	client := redis.NewClient(&redis.Options{Addr: redisServer.Addr()})
	return &redisSessionStore{redis: client, repo: repo, tokenVersionCacheTTL: time.Minute}
}

type tokenVersionRepoStub struct {
	version int64
}

func (r *tokenVersionRepoStub) Create(context.Context, repository.CreateUserInput) (*ent.User, error) {
	return nil, nil
}
func (r *tokenVersionRepoStub) ExistsByEmail(context.Context, string) (bool, error) {
	return false, nil
}
func (r *tokenVersionRepoStub) GetByEmail(context.Context, string) (*ent.User, error) {
	return nil, nil
}
func (r *tokenVersionRepoStub) GetByID(context.Context, int64) (*ent.User, error) { return nil, nil }
func (r *tokenVersionRepoStub) GetTokenVersion(context.Context, int64) (int64, error) {
	return r.version, nil
}
func (r *tokenVersionRepoStub) IncrementTokenVersion(context.Context, int64) (int64, error) {
	return r.version + 1, nil
}
func (r *tokenVersionRepoStub) ListUsers(context.Context, repository.ListUsersInput) ([]*ent.User, int, error) {
	return nil, 0, nil
}
