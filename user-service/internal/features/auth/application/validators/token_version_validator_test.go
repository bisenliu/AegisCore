package validators

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/aegiscore/common/runtime/localcache"
	commonauth "github.com/aegiscore/common/security/auth"
	authdomain "github.com/aegiscore/user-service/internal/features/auth/domain"
)

var tokenVersionTestUserID = uuid.MustParse("018f6f3e-7c4d-7b2a-9f8a-4f6b1b2c3d4e")
var tokenVersionOtherUserID = uuid.MustParse("018f6f3e-7c4d-7b2a-9f8a-4f6b1b2c3d4f")

func TestTokenVersionValidatorUsesLocalCache(t *testing.T) {
	users := &tokenVersionUserStoreStub{version: 7}
	sessions := &tokenVersionSessionStoreStub{cacheMiss: true}
	validator := NewCachingValidator(users, sessions)

	if err := validator.ValidateTokenVersion(context.Background(), tokenVersionTestUserID.String(), 7); err != nil {
		t.Fatalf("ValidateTokenVersion first: %v", err)
	}
	if err := validator.ValidateTokenVersion(context.Background(), tokenVersionTestUserID.String(), 7); err != nil {
		t.Fatalf("ValidateTokenVersion second: %v", err)
	}

	if users.getCalls != 1 || sessions.getCachedCalls != 1 || sessions.cacheCalls != 1 {
		t.Fatalf("calls users=%d getCached=%d cache=%d, want 1/1/1", users.getCalls, sessions.getCachedCalls, sessions.cacheCalls)
	}
}

func TestTokenVersionValidatorReloadsAfterLocalCacheExpires(t *testing.T) {
	users := &tokenVersionUserStoreStub{version: 7}
	sessions := &tokenVersionSessionStoreStub{cacheMiss: true}
	validator := NewCachingValidator(users, sessions)
	validator.cache = localcache.New[string, int64](time.Nanosecond)

	if err := validator.ValidateTokenVersion(context.Background(), tokenVersionTestUserID.String(), 7); err != nil {
		t.Fatalf("ValidateTokenVersion first: %v", err)
	}
	time.Sleep(time.Millisecond)
	if err := validator.ValidateTokenVersion(context.Background(), tokenVersionTestUserID.String(), 7); err != nil {
		t.Fatalf("ValidateTokenVersion second: %v", err)
	}

	if users.getCalls != 2 || sessions.getCachedCalls != 2 {
		t.Fatalf("calls users=%d getCached=%d, want 2/2", users.getCalls, sessions.getCachedCalls)
	}
}

func TestTokenVersionValidatorRejectsMismatchFromLocalCache(t *testing.T) {
	validator := NewCachingValidator(&tokenVersionUserStoreStub{}, &tokenVersionSessionStoreStub{})
	validator.cache.Set(tokenVersionTestUserID.String(), 8)

	err := validator.ValidateTokenVersion(context.Background(), tokenVersionTestUserID.String(), 7)

	if !errors.Is(err, commonauth.ErrTokenVersionMismatch) {
		t.Fatalf("err = %v, want ErrTokenVersionMismatch", err)
	}
	var mismatch *commonauth.TokenVersionMismatchError
	if !errors.As(err, &mismatch) || mismatch.Current != 8 || mismatch.Token != 7 {
		t.Fatalf("mismatch = %#v, err = %v", mismatch, err)
	}
}

func TestTokenVersionValidatorDoesNotCacheLoaderError(t *testing.T) {
	cacheErr := errors.New("redis failed")
	users := &tokenVersionUserStoreStub{version: 7}
	sessions := &tokenVersionSessionStoreStub{getErr: cacheErr}
	validator := NewCachingValidator(users, sessions)

	for i := 0; i < 2; i++ {
		_, err := validator.Current(context.Background(), tokenVersionTestUserID.String())
		if !errors.Is(err, cacheErr) {
			t.Fatalf("Current(%d) err = %v, want cacheErr", i, err)
		}
	}

	if sessions.getCachedCalls != 2 || users.getCalls != 0 {
		t.Fatalf("calls getCached=%d users=%d, want 2/0", sessions.getCachedCalls, users.getCalls)
	}
}

func TestTokenVersionValidatorSingleflightCoalescesSameUser(t *testing.T) {
	users := &tokenVersionUserStoreStub{version: 7, wait: make(chan struct{}), started: make(chan struct{})}
	sessions := &tokenVersionSessionStoreStub{cacheMiss: true}
	validator := NewCachingValidator(users, sessions)
	started := users.started
	const goroutines = 8

	var wg sync.WaitGroup
	errs := make(chan error, goroutines)
	for i := 0; i < goroutines; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			errs <- validator.ValidateTokenVersion(context.Background(), tokenVersionTestUserID.String(), 7)
		}()
	}

	<-started
	close(users.wait)
	wg.Wait()
	close(errs)
	for err := range errs {
		if err != nil {
			t.Fatalf("ValidateTokenVersion concurrent: %v", err)
		}
	}
	if users.getCalls != 1 || sessions.getCachedCalls != 1 || sessions.cacheCalls != 1 {
		t.Fatalf("calls users=%d getCached=%d cache=%d, want 1/1/1", users.getCalls, sessions.getCachedCalls, sessions.cacheCalls)
	}
}

func TestTokenVersionValidatorSingleflightKeepsUsersSeparate(t *testing.T) {
	users := &tokenVersionUserStoreStub{versions: map[uuid.UUID]int64{tokenVersionTestUserID: 7, tokenVersionOtherUserID: 9}}
	sessions := &tokenVersionSessionStoreStub{cacheMiss: true}
	validator := NewCachingValidator(users, sessions)

	var wg sync.WaitGroup
	errs := make(chan error, 2)
	wg.Add(2)
	go func() {
		defer wg.Done()
		errs <- validator.ValidateTokenVersion(context.Background(), tokenVersionTestUserID.String(), 7)
	}()
	go func() {
		defer wg.Done()
		errs <- validator.ValidateTokenVersion(context.Background(), tokenVersionOtherUserID.String(), 9)
	}()
	wg.Wait()
	close(errs)
	for err := range errs {
		if err != nil {
			t.Fatalf("ValidateTokenVersion concurrent: %v", err)
		}
	}
	if users.getCalls != 2 || sessions.getCachedCalls != 2 || sessions.cacheCalls != 2 {
		t.Fatalf("calls users=%d getCached=%d cache=%d, want 2/2/2", users.getCalls, sessions.getCachedCalls, sessions.cacheCalls)
	}
}

func TestTokenVersionValidatorInvalidateReloads(t *testing.T) {
	users := &tokenVersionUserStoreStub{version: 7}
	sessions := &tokenVersionSessionStoreStub{cacheMiss: true}
	validator := NewCachingValidator(users, sessions)

	if err := validator.ValidateTokenVersion(context.Background(), tokenVersionTestUserID.String(), 7); err != nil {
		t.Fatalf("ValidateTokenVersion first: %v", err)
	}
	validator.InvalidateTokenVersion(tokenVersionTestUserID.String())
	if err := validator.ValidateTokenVersion(context.Background(), tokenVersionTestUserID.String(), 7); err != nil {
		t.Fatalf("ValidateTokenVersion second: %v", err)
	}

	if users.getCalls != 2 || sessions.getCachedCalls != 2 {
		t.Fatalf("calls users=%d getCached=%d, want 2/2", users.getCalls, sessions.getCachedCalls)
	}
}

type tokenVersionUserStoreStub struct {
	mu       sync.Mutex
	version  int64
	versions map[uuid.UUID]int64
	err      error
	wait     chan struct{}
	started  chan struct{}
	getCalls int
}

func (s *tokenVersionUserStoreStub) GetTokenVersion(_ context.Context, userID uuid.UUID) (int64, error) {
	s.mu.Lock()
	s.getCalls++
	if s.started != nil {
		close(s.started)
		s.started = nil
	}
	wait := s.wait
	s.mu.Unlock()
	if wait != nil {
		<-wait
	}
	if s.err != nil {
		return 0, s.err
	}
	if s.versions != nil {
		return s.versions[userID], nil
	}
	return s.version, nil
}

func (s *tokenVersionUserStoreStub) IncrementTokenVersion(context.Context, uuid.UUID) (int64, error) {
	return 0, errors.New("not implemented")
}

type tokenVersionSessionStoreStub struct {
	mu             sync.Mutex
	version        int64
	getErr         error
	cacheErr       error
	cacheMiss      bool
	getCachedCalls int
	cacheCalls     int
}

func (s *tokenVersionSessionStoreStub) GetCachedTokenVersion(context.Context, string) (int64, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.getCachedCalls++
	if s.getErr != nil {
		return 0, s.getErr
	}
	if s.cacheMiss {
		return 0, authdomain.ErrTokenVersionCacheMiss
	}
	return s.version, nil
}

func (s *tokenVersionSessionStoreStub) CacheTokenVersion(context.Context, string, int64) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.cacheCalls++
	return s.cacheErr
}

func (s *tokenVersionSessionStoreStub) DeleteCachedTokenVersion(context.Context, string) error {
	return nil
}

func (s *tokenVersionSessionStoreStub) CreateSession(context.Context, authdomain.AuthSession, time.Duration, int) error {
	return nil
}

func (s *tokenVersionSessionStoreStub) RotateSession(context.Context, authdomain.AuthSession, authdomain.AuthSession, time.Duration, int) error {
	return nil
}

func (s *tokenVersionSessionStoreStub) GetSession(context.Context, string, string) (authdomain.AuthSession, error) {
	return authdomain.AuthSession{}, authdomain.ErrAuthSessionNotFound
}

func (s *tokenVersionSessionStoreStub) DeleteSession(context.Context, string, string) error {
	return nil
}

func (s *tokenVersionSessionStoreStub) DeleteAllUserSessions(context.Context, string) error {
	return nil
}
