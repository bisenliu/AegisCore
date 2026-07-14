package auth

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"go.uber.org/fx/fxtest"

	serviceconfig "github.com/aegiscore/user-service/internal/config"
	authapplication "github.com/aegiscore/user-service/internal/features/auth/application"
	authvalidators "github.com/aegiscore/user-service/internal/features/auth/application/validators"
	authdomain "github.com/aegiscore/user-service/internal/features/auth/domain"
)

func TestNewTokenVersionLocalCacheUsesFeatureConfig(t *testing.T) {
	enabled := true
	size := int64(123)
	ttl := time.Minute
	loadTimeout := time.Second
	lifecycle := fxtest.NewLifecycle(t)
	result, err := newTokenVersionLocalCache(tokenVersionCacheParams{
		Lifecycle: lifecycle,
		Config: &serviceconfig.Config{Auth: serviceconfig.AuthConfig{TokenVersionCache: serviceconfig.FeatureCacheConfig{
			Enabled: &enabled, Size: &size, TTL: &ttl, LoadTimeout: &loadTimeout,
		}}},
		Users: fakeTokenVersionStore{},
		Cache: fakeAuthStore{},
	})
	require.NoError(t, err)
	require.EqualValues(t, 123, result.Stats.Stats().Capacity)
	lifecycle.RequireStop()
}

func TestDisabledTokenVersionLocalCacheReadsThroughAndPreservesValidation(t *testing.T) {
	disabled := false
	userID := uuid.MustParse("018f0000-0000-7000-8000-000000000501")
	users := &countingTokenVersionStore{version: 7}
	result, err := newTokenVersionLocalCache(tokenVersionCacheParams{
		Config: &serviceconfig.Config{Auth: serviceconfig.AuthConfig{TokenVersionCache: serviceconfig.FeatureCacheConfig{Enabled: &disabled}}},
		Users:  users,
		Cache:  fakeAuthStore{},
	})
	require.NoError(t, err)

	validator := authvalidators.NewCachingValidator(result.Cache)
	require.NoError(t, validator.ValidateTokenVersion(context.Background(), userID.String(), 7))
	require.Error(t, validator.ValidateTokenVersion(context.Background(), userID.String(), 8))
	require.EqualValues(t, 2, users.reads.Load(), "disabled cache must read through on every validation")
	require.NoError(t, validator.InvalidateTokenVersion(userID.String()))
	require.Equal(t, authTokenVersionCacheName, result.Stats.Name())
	require.EqualValues(t, 2, result.Stats.Stats().Load)
	require.Zero(t, result.Stats.Stats().Capacity)

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

type countingTokenVersionStore struct {
	version int64
	reads   atomic.Int64
}

func (s *countingTokenVersionStore) GetTokenVersion(context.Context, uuid.UUID) (int64, error) {
	s.reads.Add(1)
	return s.version, nil
}

func (*countingTokenVersionStore) IncrementTokenVersion(context.Context, uuid.UUID) (int64, error) {
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
