package redis

import (
	"context"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	commonauth "github.com/aegiscore/common/security/auth"
	authdomain "github.com/aegiscore/user-service/internal/features/auth/domain"
)

func TestTokenVersionValidatorBackfillsMiniredisCacheOnMiss(t *testing.T) {
	redisServer := miniredis.RunT(t)
	store := newTestSessionStore(redisServer)
	ctrl := gomock.NewController(t)
	users := NewMockUserTokenVersionStore(ctrl)
	users.EXPECT().GetTokenVersion(gomock.Any(), sessionTestUserID).Return(int64(7), nil)
	validator := newTestTokenVersionValidator(t, users, store)
	ctx := context.Background()

	err := validator.ValidateTokenVersion(ctx, sessionTestUserID.String(), 7)
	require.NoError(t, err,
		"ValidateTokenVersion: %v", err)

	version, err := store.GetCachedTokenVersion(ctx, sessionTestUserID)
	require.NoError(t, err,
		"GetCachedTokenVersion after backfill: %v", err)
	require.EqualValues(t, 7, version,
		"cached version = %d, want 7", version)

	ttl, err := store.redis.TTL(ctx, store.tokenVersionKey(sessionTestUserID)).Result()
	require.NoError(t, err,
		"TTL: %v", err)
	require.False(t, ttl <= 0 || ttl > time.Minute,
		"TTL = %s, want within explicit %s", ttl, time.Minute)

}

func TestTokenVersionValidatorUsesMiniredisCacheHitWithoutRepositoryLookup(t *testing.T) {
	redisServer := miniredis.RunT(t)
	store := newTestSessionStore(redisServer)
	ctrl := gomock.NewController(t)
	users := NewMockUserTokenVersionStore(ctrl)
	ctx := context.Background()
	{
		err := store.CacheTokenVersion(ctx, sessionTestUserID, 8)
		require.NoError(t, err,
			"CacheTokenVersion: %v", err)
	}

	validator := newTestTokenVersionValidator(t, users, store)

	err := validator.ValidateTokenVersion(ctx, sessionTestUserID.String(), 8)
	require.NoError(t, err,
		"ValidateTokenVersion: %v", err)

}

func TestTokenVersionValidatorRejectsStaleTokenUsingMiniredisCache(t *testing.T) {
	redisServer := miniredis.RunT(t)
	store := newTestSessionStore(redisServer)
	ctrl := gomock.NewController(t)
	users := NewMockUserTokenVersionStore(ctrl)
	ctx := context.Background()
	{
		err := store.CacheTokenVersion(ctx, sessionTestUserID, 9)
		require.NoError(t, err,
			"CacheTokenVersion: %v", err)
	}

	validator := newTestTokenVersionValidator(t, users, store)

	err := validator.ValidateTokenVersion(ctx, sessionTestUserID.String(), 8)
	require.ErrorIs(t, err, commonauth.ErrTokenVersionMismatch,
		"err = %v, want token version mismatch", err)

}

func TestTokenVersionCacheRefreshMakesStaleTokenObservable(t *testing.T) {
	redisServer := miniredis.RunT(t)
	store := newTestSessionStore(redisServer)
	ctx := context.Background()
	{
		err := store.CacheTokenVersion(ctx, sessionTestUserID, 5)
		require.NoError(t, err,
			"CacheTokenVersion old: %v", err)
	}
	{

		err := store.CreateSession(ctx, authdomain.AuthSession{UserID: sessionTestUserID, SessionID: "s-stale", TokenVersion: 5}, time.Hour, defaultMaxActiveSessionsPerUser())
		require.NoError(t, err,
			"CreateSession: %v", err)
	}
	{

		err := store.CacheTokenVersion(ctx, sessionTestUserID, 6)
		require.NoError(t, err,
			"CacheTokenVersion refreshed: %v", err)
	}
	{

		err := store.DeleteAllUserSessions(ctx, sessionTestUserID)
		require.NoError(t, err,
			"DeleteAllUserSessions: %v", err)
	}

	ctrl := gomock.NewController(t)
	users := NewMockUserTokenVersionStore(ctrl)
	validator := newTestTokenVersionValidator(t, users, store)

	err := validator.ValidateTokenVersion(ctx, sessionTestUserID.String(), 5)
	require.ErrorIs(t, err, commonauth.ErrTokenVersionMismatch,
		"err = %v, want token version mismatch", err)

	version, err := store.GetCachedTokenVersion(ctx, sessionTestUserID)
	require.NoError(t, err,
		"GetCachedTokenVersion: %v", err)
	require.EqualValues(t, 6, version,
		"cached version = %d, want 6", version)

	waitForRedisCondition(t, func() bool {
		return !redisServer.Exists(store.sessionKey(sessionTestUserID, "s-stale")) &&
			!redisServer.Exists(store.userSessionsKey(sessionTestUserID))
	}, "user sessions were not deleted during cache refresh flow")
}
