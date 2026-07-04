package validators

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"github.com/aegiscore/common/runtime/localcache"
	commonauth "github.com/aegiscore/common/security/auth"
	authdomain "github.com/aegiscore/user-service/internal/features/auth/domain"
)

var tokenVersionTestUserID = uuid.MustParse("018f6f3e-7c4d-7b2a-9f8a-4f6b1b2c3d4e")
var tokenVersionOtherUserID = uuid.MustParse("018f6f3e-7c4d-7b2a-9f8a-4f6b1b2c3d4f")

func TestTokenVersionValidatorUsesLocalCache(t *testing.T) {
	ctrl := gomock.NewController(t)
	users := NewMockUserTokenVersionStore(ctrl)
	tokenCache := NewMockTokenVersionCache(ctrl)
	userID := tokenVersionTestUserID.String()
	tokenCache.EXPECT().GetCachedTokenVersion(gomock.Any(), userID).Return(int64(0), authdomain.ErrTokenVersionCacheMiss).Times(1)
	users.EXPECT().GetTokenVersion(gomock.Any(), tokenVersionTestUserID).Return(int64(7), nil).Times(1)
	tokenCache.EXPECT().CacheTokenVersion(gomock.Any(), userID, int64(7)).Return(nil).Times(1)
	validator := newTestTokenVersionValidator(t, users, tokenCache, time.Minute)
	{

		err := validator.ValidateTokenVersion(context.Background(), userID, 7)
		require.NoError(t, err,
			"ValidateTokenVersion first: %v", err)
	}
	{

		err := validator.ValidateTokenVersion(context.Background(), userID, 7)
		require.NoError(t, err,
			"ValidateTokenVersion second: %v", err)
	}

}

func TestTokenVersionValidatorUsesRedisCache(t *testing.T) {
	ctrl := gomock.NewController(t)
	users := NewMockUserTokenVersionStore(ctrl)
	tokenCache := NewMockTokenVersionCache(ctrl)
	userID := tokenVersionTestUserID.String()
	tokenCache.EXPECT().GetCachedTokenVersion(gomock.Any(), userID).Return(int64(7), nil).Times(1)
	validator := newTestTokenVersionValidator(t, users, tokenCache, time.Minute)
	{

		err := validator.ValidateTokenVersion(context.Background(), userID, 7)
		require.NoError(t, err,
			"ValidateTokenVersion first: %v", err)
	}
	{

		err := validator.ValidateTokenVersion(context.Background(), userID, 7)
		require.NoError(t, err,
			"ValidateTokenVersion second: %v", err)
	}

}

func TestTokenVersionValidatorReloadsAfterLocalCacheExpires(t *testing.T) {
	ctrl := gomock.NewController(t)
	users := NewMockUserTokenVersionStore(ctrl)
	tokenCache := NewMockTokenVersionCache(ctrl)
	userID := tokenVersionTestUserID.String()
	var userLoads atomic.Int64
	tokenCache.EXPECT().GetCachedTokenVersion(gomock.Any(), userID).Return(int64(0), authdomain.ErrTokenVersionCacheMiss).Times(2)
	users.EXPECT().GetTokenVersion(gomock.Any(), tokenVersionTestUserID).DoAndReturn(func(context.Context, uuid.UUID) (int64, error) {
		userLoads.Add(1)
		return int64(7), nil
	}).Times(2)
	tokenCache.EXPECT().CacheTokenVersion(gomock.Any(), userID, int64(7)).Return(nil).Times(2)
	validator := newTestTokenVersionValidator(t, users, tokenCache, time.Nanosecond)
	{

		err := validator.ValidateTokenVersion(context.Background(), userID, 7)
		require.NoError(t, err,
			"ValidateTokenVersion first: %v", err)
	}
	require.EqualValues(t, 1, userLoads.Load())

	require.Eventually(t, func() bool {
		err := validator.ValidateTokenVersion(context.Background(), userID, 7)
		require.NoError(t, err,
			"ValidateTokenVersion second: %v", err)
		return userLoads.Load() == 2
	}, time.Second, time.Millisecond)

}

func TestTokenVersionValidatorRejectsMismatchFromLocalCache(t *testing.T) {
	ctrl := gomock.NewController(t)
	users := NewMockUserTokenVersionStore(ctrl)
	tokenCache := NewMockTokenVersionCache(ctrl)
	userID := tokenVersionTestUserID.String()
	tokenCache.EXPECT().GetCachedTokenVersion(gomock.Any(), userID).Return(int64(0), authdomain.ErrTokenVersionCacheMiss).Times(1)
	users.EXPECT().GetTokenVersion(gomock.Any(), tokenVersionTestUserID).Return(int64(8), nil).Times(1)
	tokenCache.EXPECT().CacheTokenVersion(gomock.Any(), userID, int64(8)).Return(nil).Times(1)
	validator := newTestTokenVersionValidator(t, users, tokenCache, time.Minute)
	{
		err := validator.ValidateTokenVersion(context.Background(), userID, 8)
		require.NoError(t, err,
			"ValidateTokenVersion warmup: %v", err)
	}

	err := validator.ValidateTokenVersion(context.Background(), userID, 7)
	require.ErrorIs(t, err, commonauth.ErrTokenVersionMismatch,
		"err = %v, want ErrTokenVersionMismatch", err)

	var mismatch *commonauth.TokenVersionMismatchError
	require.False(t, !errors.As(err, &mismatch) || mismatch.Current != 8 || mismatch.Token != 7,
		"mismatch = %#v, err = %v", mismatch, err)

}

func TestTokenVersionValidatorDoesNotCacheLoaderError(t *testing.T) {
	cacheErr := errors.New("redis failed")
	ctrl := gomock.NewController(t)
	users := NewMockUserTokenVersionStore(ctrl)
	tokenCache := NewMockTokenVersionCache(ctrl)
	userID := tokenVersionTestUserID.String()
	tokenCache.EXPECT().GetCachedTokenVersion(gomock.Any(), userID).Return(int64(0), cacheErr).Times(2)
	validator := newTestTokenVersionValidator(t, users, tokenCache, time.Minute)

	for i := 0; i < 2; i++ {
		_, err := validator.Current(context.Background(), userID)
		require.ErrorIs(t, err, cacheErr,
			"Current(%d) err = %v, want cacheErr", i, err)

	}
}

func TestTokenVersionValidatorDoesNotCacheBackfillError(t *testing.T) {
	cacheErr := errors.New("redis set failed")
	ctrl := gomock.NewController(t)
	users := NewMockUserTokenVersionStore(ctrl)
	tokenCache := NewMockTokenVersionCache(ctrl)
	userID := tokenVersionTestUserID.String()
	tokenCache.EXPECT().GetCachedTokenVersion(gomock.Any(), userID).Return(int64(0), authdomain.ErrTokenVersionCacheMiss).Times(2)
	users.EXPECT().GetTokenVersion(gomock.Any(), tokenVersionTestUserID).Return(int64(7), nil).Times(2)
	tokenCache.EXPECT().CacheTokenVersion(gomock.Any(), userID, int64(7)).Return(cacheErr).Times(2)
	validator := newTestTokenVersionValidator(t, users, tokenCache, time.Minute)

	for i := 0; i < 2; i++ {
		_, err := validator.Current(context.Background(), userID)
		require.ErrorIs(t, err, cacheErr,
			"Current(%d) err = %v, want cacheErr", i, err)

	}
}

func TestTokenVersionValidatorSingleflightCoalescesSameUser(t *testing.T) {
	ctrl := gomock.NewController(t)
	users := NewMockUserTokenVersionStore(ctrl)
	tokenCache := NewMockTokenVersionCache(ctrl)
	userID := tokenVersionTestUserID.String()
	started := make(chan struct{})
	release := make(chan struct{})
	var startedOnce sync.Once
	tokenCache.EXPECT().GetCachedTokenVersion(gomock.Any(), userID).Return(int64(0), authdomain.ErrTokenVersionCacheMiss).Times(1)
	users.EXPECT().GetTokenVersion(gomock.Any(), tokenVersionTestUserID).DoAndReturn(func(context.Context, uuid.UUID) (int64, error) {
		startedOnce.Do(func() { close(started) })
		<-release
		return 7, nil
	}).Times(1)
	tokenCache.EXPECT().CacheTokenVersion(gomock.Any(), userID, int64(7)).Return(nil).Times(1)
	validator := newTestTokenVersionValidator(t, users, tokenCache, time.Minute)
	const goroutines = 8

	var wg sync.WaitGroup
	errs := make(chan error, goroutines)
	for i := 0; i < goroutines; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			errs <- validator.ValidateTokenVersion(context.Background(), userID, 7)
		}()
	}

	<-started
	close(release)
	wg.Wait()
	close(errs)
	for err := range errs {
		require.NoError(t, err,
			"ValidateTokenVersion concurrent: %v", err)

	}
}

func TestTokenVersionValidatorSingleflightKeepsUsersSeparate(t *testing.T) {
	ctrl := gomock.NewController(t)
	users := NewMockUserTokenVersionStore(ctrl)
	tokenCache := NewMockTokenVersionCache(ctrl)
	userID := tokenVersionTestUserID.String()
	otherUserID := tokenVersionOtherUserID.String()
	tokenCache.EXPECT().GetCachedTokenVersion(gomock.Any(), userID).Return(int64(0), authdomain.ErrTokenVersionCacheMiss).Times(1)
	users.EXPECT().GetTokenVersion(gomock.Any(), tokenVersionTestUserID).Return(int64(7), nil).Times(1)
	tokenCache.EXPECT().CacheTokenVersion(gomock.Any(), userID, int64(7)).Return(nil).Times(1)
	tokenCache.EXPECT().GetCachedTokenVersion(gomock.Any(), otherUserID).Return(int64(0), authdomain.ErrTokenVersionCacheMiss).Times(1)
	users.EXPECT().GetTokenVersion(gomock.Any(), tokenVersionOtherUserID).Return(int64(9), nil).Times(1)
	tokenCache.EXPECT().CacheTokenVersion(gomock.Any(), otherUserID, int64(9)).Return(nil).Times(1)
	validator := newTestTokenVersionValidator(t, users, tokenCache, time.Minute)

	var wg sync.WaitGroup
	errs := make(chan error, 2)
	wg.Add(2)
	go func() {
		defer wg.Done()
		errs <- validator.ValidateTokenVersion(context.Background(), userID, 7)
	}()
	go func() {
		defer wg.Done()
		errs <- validator.ValidateTokenVersion(context.Background(), otherUserID, 9)
	}()
	wg.Wait()
	close(errs)
	for err := range errs {
		require.NoError(t, err,
			"ValidateTokenVersion concurrent: %v", err)

	}
}

func TestTokenVersionValidatorInvalidateReloads(t *testing.T) {
	ctrl := gomock.NewController(t)
	users := NewMockUserTokenVersionStore(ctrl)
	tokenCache := NewMockTokenVersionCache(ctrl)
	userID := tokenVersionTestUserID.String()
	tokenCache.EXPECT().GetCachedTokenVersion(gomock.Any(), userID).Return(int64(0), authdomain.ErrTokenVersionCacheMiss).Times(2)
	users.EXPECT().GetTokenVersion(gomock.Any(), tokenVersionTestUserID).Return(int64(7), nil).Times(2)
	tokenCache.EXPECT().CacheTokenVersion(gomock.Any(), userID, int64(7)).Return(nil).Times(2)
	validator := newTestTokenVersionValidator(t, users, tokenCache, time.Minute)
	{

		err := validator.ValidateTokenVersion(context.Background(), userID, 7)
		require.NoError(t, err,
			"ValidateTokenVersion first: %v", err)
	}

	require.NoError(t, validator.InvalidateTokenVersion(userID),
		"InvalidateTokenVersion")
	{
		err := validator.ValidateTokenVersion(context.Background(), userID, 7)
		require.NoError(t, err,
			"ValidateTokenVersion second: %v", err)
	}

}

func TestTokenVersionValidatorInvalidateReturnsDeleteError(t *testing.T) {
	cache, err := localcache.New[string, int64](localcache.Config[string]{
		Name:        "auth_token_version_invalidate_error_test",
		Capacity:    100,
		TTL:         time.Minute,
		LoadTimeout: time.Second,
		KeyString:   func(key string) string { return key },
	}, func(context.Context, string) (int64, error) {
		return 0, nil
	}, nil)
	require.NoError(t, err,
		"New localcache: %v", err)
	cache.Close()

	validator := NewCachingValidator(cache)
	err = validator.InvalidateTokenVersion(tokenVersionTestUserID.String())
	require.ErrorIs(t, err, localcache.ErrClosed,
		"err = %v, want ErrClosed", err)
}

func newTestTokenVersionValidator(t *testing.T, users *MockUserTokenVersionStore, tokenCache *MockTokenVersionCache, ttl time.Duration) *TokenVersionValidator {
	t.Helper()
	cache, err := localcache.New[string, int64](localcache.Config[string]{
		Name:        "auth_token_version_test",
		Capacity:    100,
		TTL:         ttl,
		LoadTimeout: time.Second,
		KeyString:   func(key string) string { return key },
	}, func(ctx context.Context, userID string) (int64, error) {
		return Current(ctx, users, tokenCache, userID)
	}, nil)
	require.NoError(t, err,
		"New localcache: %v", err)

	t.Cleanup(cache.Close)
	return NewCachingValidator(cache)
}
