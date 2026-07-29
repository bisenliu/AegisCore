package redis

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/stretchr/testify/require"

	authdomain "github.com/aegiscore/user-service/internal/features/auth/domain"
)

func TestPasswordChangeSessionCreateConsumeAndRevoke(t *testing.T) {
	redisServer := miniredis.RunT(t)
	store := newTestSessionStore(redisServer)
	ctx := context.Background()
	session := passwordChangeSession("pc-123", "jti-123", 2)

	err := store.CreatePasswordChangeSession(ctx, session, time.Minute)
	require.NoError(t, err,
		"CreatePasswordChangeSession: %v", err)
	require.True(t, redisServer.Exists(store.passwordChangeSessionKey(session.UserID, session.SessionID)))

	err = store.ConsumePasswordChangeSession(ctx, session)
	require.NoError(t, err,
		"ConsumePasswordChangeSession: %v", err)
	require.False(t, redisServer.Exists(store.passwordChangeSessionKey(session.UserID, session.SessionID)))

	err = store.CreatePasswordChangeSession(ctx, session, time.Minute)
	require.NoError(t, err,
		"CreatePasswordChangeSession second: %v", err)
	err = store.RevokePasswordChangeSession(ctx, session.UserID, session.SessionID)
	require.NoError(t, err,
		"RevokePasswordChangeSession: %v", err)
	err = store.ConsumePasswordChangeSession(ctx, session)
	require.ErrorIs(t, err, authdomain.ErrPasswordChangeSessionNotFound,
		"err = %v, want not found", err)
}

func TestPasswordChangeSessionRejectsMismatchAndExpired(t *testing.T) {
	redisServer := miniredis.RunT(t)
	store := newTestSessionStore(redisServer)
	ctx := context.Background()
	session := passwordChangeSession("pc-123", "jti-123", 2)

	err := store.CreatePasswordChangeSession(ctx, session, time.Minute)
	require.NoError(t, err,
		"CreatePasswordChangeSession: %v", err)
	mismatch := session
	mismatch.TokenID = "other-jti"
	err = store.ConsumePasswordChangeSession(ctx, mismatch)
	require.ErrorIs(t, err, authdomain.ErrPasswordChangeSessionMismatch,
		"err = %v, want mismatch", err)
	require.True(t, redisServer.Exists(store.passwordChangeSessionKey(session.UserID, session.SessionID)))

	redisServer.FastForward(time.Minute + time.Second)
	err = store.ConsumePasswordChangeSession(ctx, session)
	require.ErrorIs(t, err, authdomain.ErrPasswordChangeSessionNotFound,
		"err = %v, want not found", err)
}

func TestPasswordChangeSessionConcurrentConsumeSucceedsOnce(t *testing.T) {
	redisServer := miniredis.RunT(t)
	store := newTestSessionStore(redisServer)
	ctx := context.Background()
	session := passwordChangeSession("pc-123", "jti-123", 2)
	require.NoError(t, store.CreatePasswordChangeSession(ctx, session, time.Minute))

	var wg sync.WaitGroup
	var successes int64
	var rejects int64
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			err := store.ConsumePasswordChangeSession(ctx, session)
			if err == nil {
				atomic.AddInt64(&successes, 1)
				return
			}
			if errors.Is(err, authdomain.ErrPasswordChangeSessionNotFound) {
				atomic.AddInt64(&rejects, 1)
			}
		}()
	}
	wg.Wait()

	require.EqualValues(t, 1, successes,
		"successes = %d, want 1", successes)
	require.EqualValues(t, 7, rejects,
		"rejects = %d, want 7", rejects)
}

func passwordChangeSession(sessionID string, tokenID string, tokenVersion int64) authdomain.PasswordChangeSession {
	return authdomain.PasswordChangeSession{UserID: sessionTestUserID, SessionID: sessionID, TokenID: tokenID, TokenVersion: tokenVersion}
}
