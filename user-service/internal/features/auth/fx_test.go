package auth

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/aegiscore/common/runtime/config"
	authapplication "github.com/aegiscore/user-service/internal/features/auth/application"
	authdomain "github.com/aegiscore/user-service/internal/features/auth/domain"
)

func TestNewTokenVersionLocalCacheRequiresConfigInstance(t *testing.T) {
	_, err := newTokenVersionLocalCache(tokenVersionCacheParams{
		Config:   &config.Config{LocalCache: config.LocalCacheConfig{}},
		Users:    fakeTokenVersionStore{},
		Sessions: fakeAuthSessionStore{},
	})

	if err == nil || !strings.Contains(err.Error(), "local_cache.auth_token_version is required") {
		t.Fatalf("newTokenVersionLocalCache error = %v, want missing local cache config", err)
	}
}

var _ authapplication.UserTokenVersionStore = fakeTokenVersionStore{}
var _ authapplication.AuthSessionStore = fakeAuthSessionStore{}

type fakeTokenVersionStore struct{}

func (fakeTokenVersionStore) GetTokenVersion(context.Context, uuid.UUID) (int64, error) {
	return 0, errors.New("not implemented")
}

func (fakeTokenVersionStore) IncrementTokenVersion(context.Context, uuid.UUID) (int64, error) {
	return 0, errors.New("not implemented")
}

type fakeAuthSessionStore struct{}

func (fakeAuthSessionStore) GetCachedTokenVersion(context.Context, string) (int64, error) {
	return 0, authdomain.ErrTokenVersionCacheMiss
}

func (fakeAuthSessionStore) CacheTokenVersion(context.Context, string, int64) error {
	return nil
}

func (fakeAuthSessionStore) DeleteCachedTokenVersion(context.Context, string) error {
	return nil
}

func (fakeAuthSessionStore) CreateSession(context.Context, authdomain.AuthSession, time.Duration, int) error {
	return nil
}

func (fakeAuthSessionStore) RotateSession(context.Context, authdomain.AuthSession, authdomain.AuthSession, time.Duration, int) error {
	return nil
}

func (fakeAuthSessionStore) GetSession(context.Context, string, string) (authdomain.AuthSession, error) {
	return authdomain.AuthSession{}, errors.New("not implemented")
}

func (fakeAuthSessionStore) DeleteSession(context.Context, string, string) error {
	return nil
}

func (fakeAuthSessionStore) DeleteAllUserSessions(context.Context, string) error {
	return nil
}
