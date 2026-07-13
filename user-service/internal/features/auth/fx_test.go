package auth

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	commonconfig "github.com/aegiscore/common/runtime/config"
	serviceconfig "github.com/aegiscore/user-service/internal/config"
	authapplication "github.com/aegiscore/user-service/internal/features/auth/application"
	authdomain "github.com/aegiscore/user-service/internal/features/auth/domain"
)

func TestNewTokenVersionLocalCacheRequiresConfigInstance(t *testing.T) {
	_, err := newTokenVersionLocalCache(tokenVersionCacheParams{
		Config: &serviceconfig.Config{Config: commonconfig.Config{LocalCache: commonconfig.LocalCacheConfig{}}},
		Users:  fakeTokenVersionStore{},
		Cache:  fakeAuthStore{},
	})
	require.False(t, err == nil || !strings.Contains(err.Error(), "local_cache.auth_token_version is required"),
		"newTokenVersionLocalCache error = %v, want missing local cache config", err)

}

var _ authapplication.UserTokenVersionStore = fakeTokenVersionStore{}
var _ authapplication.TokenVersionCache = fakeAuthStore{}
var _ authapplication.RefreshSessionStore = fakeAuthStore{}

type fakeTokenVersionStore struct{}

func (fakeTokenVersionStore) GetTokenVersion(context.Context, uuid.UUID) (int64, error) {
	return 0, errors.New("not implemented")
}

func (fakeTokenVersionStore) IncrementTokenVersion(context.Context, uuid.UUID) (int64, error) {
	return 0, errors.New("not implemented")
}

type fakeAuthStore struct{}

func (fakeAuthStore) GetCachedTokenVersion(context.Context, string) (int64, error) {
	return 0, authdomain.ErrTokenVersionCacheMiss
}

func (fakeAuthStore) CacheTokenVersion(context.Context, string, int64) error {
	return nil
}

func (fakeAuthStore) DeleteCachedTokenVersion(context.Context, string) error {
	return nil
}

func (fakeAuthStore) CreateSession(context.Context, authdomain.AuthSession, time.Duration, int) error {
	return nil
}

func (fakeAuthStore) RotateSession(context.Context, authdomain.AuthSession, authdomain.AuthSession, time.Duration, int) error {
	return nil
}

func (fakeAuthStore) GetSession(context.Context, string, string) (authdomain.AuthSession, error) {
	return authdomain.AuthSession{}, errors.New("not implemented")
}

func (fakeAuthStore) DeleteSession(context.Context, string, string) error {
	return nil
}

func (fakeAuthStore) DeleteAllUserSessions(context.Context, string) error {
	return nil
}
