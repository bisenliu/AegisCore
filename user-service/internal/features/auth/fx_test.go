package auth

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"go.uber.org/fx/fxtest"
	"go.uber.org/mock/gomock"

	serviceconfig "github.com/aegiscore/user-service/internal/config"
	authvalidators "github.com/aegiscore/user-service/internal/features/auth/application/validators"
	authdomain "github.com/aegiscore/user-service/internal/features/auth/domain"
)

func TestNewTokenVersionLocalCacheUsesFeatureConfig(t *testing.T) {
	enabled := true
	size := int64(123)
	ttl := time.Minute
	loadTimeout := time.Second
	lifecycle := fxtest.NewLifecycle(t)
	ctrl := gomock.NewController(t)
	result, err := newTokenVersionLocalCache(tokenVersionCacheParams{
		Lifecycle: lifecycle,
		Config: &serviceconfig.Config{Auth: serviceconfig.AuthConfig{TokenVersionCache: serviceconfig.FeatureCacheConfig{
			Enabled: &enabled, Size: &size, TTL: &ttl, LoadTimeout: &loadTimeout,
		}}},
		Users: NewMockUserTokenVersionStore(ctrl),
		Cache: NewMockTokenVersionCache(ctrl),
	})
	require.NoError(t, err)
	require.EqualValues(t, 123, result.Stats.Stats().Capacity)
	lifecycle.RequireStop()
}

func TestDisabledTokenVersionLocalCacheReadsThroughAndPreservesValidation(t *testing.T) {
	disabled := false
	userID := uuid.MustParse("018f0000-0000-7000-8000-000000000501")
	ctrl := gomock.NewController(t)
	users := NewMockUserTokenVersionStore(ctrl)
	cache := NewMockTokenVersionCache(ctrl)
	gomock.InOrder(
		cache.EXPECT().GetCachedTokenVersion(gomock.Any(), userID.String()).Return(int64(0), authdomain.ErrTokenVersionCacheMiss),
		users.EXPECT().GetTokenVersion(gomock.Any(), userID).Return(int64(7), nil),
		cache.EXPECT().CacheTokenVersion(gomock.Any(), userID.String(), int64(7)).Return(nil),
		cache.EXPECT().GetCachedTokenVersion(gomock.Any(), userID.String()).Return(int64(0), authdomain.ErrTokenVersionCacheMiss),
		users.EXPECT().GetTokenVersion(gomock.Any(), userID).Return(int64(7), nil),
		cache.EXPECT().CacheTokenVersion(gomock.Any(), userID.String(), int64(7)).Return(nil),
	)
	result, err := newTokenVersionLocalCache(tokenVersionCacheParams{
		Config: &serviceconfig.Config{Auth: serviceconfig.AuthConfig{TokenVersionCache: serviceconfig.FeatureCacheConfig{Enabled: &disabled}}},
		Users:  users,
		Cache:  cache,
	})
	require.NoError(t, err)

	validator := authvalidators.NewCachingValidator(result.Cache)
	require.NoError(t, validator.ValidateTokenVersion(context.Background(), userID.String(), 7))
	require.Error(t, validator.ValidateTokenVersion(context.Background(), userID.String(), 8))
	require.NoError(t, validator.InvalidateTokenVersion(userID.String()))
	require.Equal(t, authTokenVersionCacheName, result.Stats.Name())
	require.EqualValues(t, 2, result.Stats.Stats().Load)
	require.Zero(t, result.Stats.Stats().Capacity)
}
