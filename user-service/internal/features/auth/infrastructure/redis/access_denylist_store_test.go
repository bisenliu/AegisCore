package redis

import (
	"context"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/stretchr/testify/require"
)

func TestAccessTokenDenylistUsesRemainingTTL(t *testing.T) {
	redisServer := miniredis.RunT(t)
	store := newTestSessionStore(redisServer)
	ctx := context.Background()

	require.NoError(t, store.DenyAccessToken(ctx, "jti-current", 30*time.Second))
	denied, err := store.AccessTokenDenied(ctx, "jti-current")
	require.NoError(t, err)
	require.True(t, denied)

	denied, err = store.AccessTokenDenied(ctx, "jti-other")
	require.NoError(t, err)
	require.False(t, denied)

	redisServer.FastForward(31 * time.Second)
	denied, err = store.AccessTokenDenied(ctx, "jti-current")
	require.NoError(t, err)
	require.False(t, denied)
}

func TestBoundedAccessTokenDenylistCapsTTL(t *testing.T) {
	inner := &recordingDenylist{}
	store := NewBoundedAccessTokenDenylist(inner, 15*time.Minute)

	require.NoError(t, store.DenyAccessToken(context.Background(), "jti-current", 20*time.Minute))
	require.Equal(t, 15*time.Minute, inner.ttl)
}

type recordingDenylist struct {
	ttl time.Duration
}

func (d *recordingDenylist) DenyAccessToken(_ context.Context, _ string, ttl time.Duration) error {
	d.ttl = ttl
	return nil
}

func (d *recordingDenylist) AccessTokenDenied(context.Context, string) (bool, error) {
	return false, nil
}

func TestAccessTokenDenylistFailsClosedWhenRedisClosed(t *testing.T) {
	redisServer := miniredis.RunT(t)
	store := newTestSessionStore(redisServer)
	redisServer.Close()

	_, err := store.AccessTokenDenied(context.Background(), "jti-current")
	require.Error(t, err)
}
