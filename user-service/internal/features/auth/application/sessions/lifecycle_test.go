package sessions

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"

	authdomain "github.com/aegiscore/user-service/internal/features/auth/domain"
	"github.com/aegiscore/user-service/internal/shared/identity"
)

var sessionTestUserID = uuid.MustParse("018f6f3e-7c4d-7b2a-9f8a-4f6b1b2c3d4e")

func TestLifecycleRotateTokenSessionMapsRejectedSession(t *testing.T) {
	for _, err := range []error{authdomain.ErrAuthSessionNotFound, authdomain.ErrAuthSessionMismatch} {
		t.Run(err.Error(), func(t *testing.T) {
			store := &authSessionTestStore{rotateErr: err}
			lifecycle := NewLifecycle(&sessionUserTestStore{}, store, store, 5)
			oldSession := authdomain.AuthSession{UserID: sessionTestUserID.String(), SessionID: "s-old", TokenVersion: 2}
			newSession := authdomain.AuthSession{UserID: sessionTestUserID.String(), SessionID: "s-new", TokenVersion: 2}

			err := lifecycle.RotateTokenSession(context.Background(), oldSession, newSession, time.Hour)

			if !errors.Is(err, authdomain.ErrTokenInvalid) {
				t.Fatalf("err = %v, want ErrTokenInvalid", err)
			}
		})
	}
}

func TestLifecycleCurrentTokenVersionCacheMissReadsRepository(t *testing.T) {
	users := &sessionUserTestStore{tokenVersion: 7}
	store := &authSessionTestStore{cacheMiss: true}
	lifecycle := NewLifecycle(users, store, store, 5)

	version, err := lifecycle.CurrentTokenVersion(context.Background(), sessionTestUserID.String())

	if err != nil {
		t.Fatalf("CurrentTokenVersion: %v", err)
	}
	if version != 7 {
		t.Fatalf("version = %d, want 7", version)
	}
	if users.getTokenVersionID != sessionTestUserID || !store.cached || store.cachedVersion != 7 {
		t.Fatalf("users=%#v store=%#v", users, store)
	}
}

func TestLifecycleRevokeAllUserSessions(t *testing.T) {
	users := &sessionUserTestStore{newVersion: 4}
	store := &authSessionTestStore{}
	invalidator := &tokenVersionRecordingInvalidator{}
	lifecycle := NewLifecycle(users, store, store, 5, invalidator)

	result, err := lifecycle.RevokeAllUserSessions(context.Background(), sessionTestUserID)

	if err != nil {
		t.Fatalf("RevokeAllUserSessions: %v", err)
	}
	if result.UserID != sessionTestUserID || result.TokenVersion != 4 {
		t.Fatalf("result = %#v", result)
	}
	if result.ProjectionError != nil {
		t.Fatalf("projection error = %v, want nil", result.ProjectionError)
	}
	if users.incrementedUserID != sessionTestUserID || !store.cached || store.cachedVersion != 4 || !store.deletedAll {
		t.Fatalf("users=%#v store=%#v", users, store)
	}
	if invalidator.calls == 0 || invalidator.userID != sessionTestUserID.String() {
		t.Fatalf("invalidator = %#v, want user %s", invalidator, sessionTestUserID.String())
	}
}

type sessionUserTestStore struct {
	tokenVersion      int64
	newVersion        int64
	getTokenVersionID uuid.UUID
	incrementedUserID uuid.UUID
}

func (s *sessionUserTestStore) GetTokenVersion(_ context.Context, userID uuid.UUID) (int64, error) {
	s.getTokenVersionID = userID
	return s.tokenVersion, nil
}

func (s *sessionUserTestStore) IncrementTokenVersion(_ context.Context, userID uuid.UUID) (int64, error) {
	s.incrementedUserID = userID
	if s.newVersion == 0 {
		return 0, identity.ErrUserNotFound
	}
	return s.newVersion, nil
}

type authSessionTestStore struct {
	cacheMiss     bool
	cached        bool
	cachedVersion int64
	deletedAll    bool
	rotateErr     error
}

func (s *authSessionTestStore) GetCachedTokenVersion(context.Context, string) (int64, error) {
	if s.cacheMiss {
		return 0, authdomain.ErrTokenVersionCacheMiss
	}
	return s.cachedVersion, nil
}

func (s *authSessionTestStore) CacheTokenVersion(_ context.Context, _ string, tokenVersion int64) error {
	s.cached = true
	s.cachedVersion = tokenVersion
	return nil
}

func (s *authSessionTestStore) DeleteCachedTokenVersion(context.Context, string) error {
	return nil
}

func (s *authSessionTestStore) CreateSession(context.Context, authdomain.AuthSession, time.Duration, int) error {
	return nil
}

func (s *authSessionTestStore) RotateSession(context.Context, authdomain.AuthSession, authdomain.AuthSession, time.Duration, int) error {
	return s.rotateErr
}

func (s *authSessionTestStore) GetSession(context.Context, string, string) (authdomain.AuthSession, error) {
	return authdomain.AuthSession{}, authdomain.ErrAuthSessionNotFound
}

func (s *authSessionTestStore) DeleteSession(context.Context, string, string) error {
	return nil
}

func (s *authSessionTestStore) DeleteAllUserSessions(context.Context, string) error {
	s.deletedAll = true
	return nil
}

type tokenVersionRecordingInvalidator struct {
	calls  int
	userID string
}

func (s *tokenVersionRecordingInvalidator) InvalidateTokenVersion(userID string) {
	s.calls++
	s.userID = userID
}
