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
	authapplication "github.com/aegiscore/user-service/internal/features/auth/application"
	authdomain "github.com/aegiscore/user-service/internal/features/auth/domain"
)

var tokenVersionTestUserID = uuid.MustParse("018f6f3e-7c4d-7b2a-9f8a-4f6b1b2c3d4e")
var tokenVersionOtherUserID = uuid.MustParse("018f6f3e-7c4d-7b2a-9f8a-4f6b1b2c3d4f")

func TestTokenVersionValidatorUsesLocalCache(t *testing.T) {
	users := &tokenVersionUserTestStore{version: 7}
	sessions := &tokenVersionSessionTestStore{cacheMiss: true}
	validator := newTestTokenVersionValidator(t, users, sessions, time.Minute)

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
	users := &tokenVersionUserTestStore{version: 7}
	sessions := &tokenVersionSessionTestStore{cacheMiss: true}
	validator := newTestTokenVersionValidator(t, users, sessions, time.Nanosecond)

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
	users := &tokenVersionUserTestStore{version: 8}
	sessions := &tokenVersionSessionTestStore{cacheMiss: true}
	validator := newTestTokenVersionValidator(t, users, sessions, time.Minute)
	if err := validator.ValidateTokenVersion(context.Background(), tokenVersionTestUserID.String(), 8); err != nil {
		t.Fatalf("ValidateTokenVersion warmup: %v", err)
	}

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
	users := &tokenVersionUserTestStore{version: 7}
	sessions := &tokenVersionSessionTestStore{getErr: cacheErr}
	validator := newTestTokenVersionValidator(t, users, sessions, time.Minute)

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
	users := &tokenVersionUserTestStore{version: 7, wait: make(chan struct{}), started: make(chan struct{})}
	sessions := &tokenVersionSessionTestStore{cacheMiss: true}
	validator := newTestTokenVersionValidator(t, users, sessions, time.Minute)
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
	users := &tokenVersionUserTestStore{versions: map[uuid.UUID]int64{tokenVersionTestUserID: 7, tokenVersionOtherUserID: 9}}
	sessions := &tokenVersionSessionTestStore{cacheMiss: true}
	validator := newTestTokenVersionValidator(t, users, sessions, time.Minute)

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
	users := &tokenVersionUserTestStore{version: 7}
	sessions := &tokenVersionSessionTestStore{cacheMiss: true}
	validator := newTestTokenVersionValidator(t, users, sessions, time.Minute)

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

func newTestTokenVersionValidator(t *testing.T, users authapplication.UserTokenVersionStore, tokenCache authapplication.TokenVersionCache, ttl time.Duration) *TokenVersionValidator {
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
	if err != nil {
		t.Fatalf("New localcache: %v", err)
	}
	t.Cleanup(cache.Close)
	return NewCachingValidator(cache)
}

type tokenVersionUserTestStore struct {
	mu       sync.Mutex
	version  int64
	versions map[uuid.UUID]int64
	err      error
	wait     chan struct{}
	started  chan struct{}
	getCalls int
}

func (s *tokenVersionUserTestStore) GetTokenVersion(_ context.Context, userID uuid.UUID) (int64, error) {
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

func (s *tokenVersionUserTestStore) IncrementTokenVersion(context.Context, uuid.UUID) (int64, error) {
	return 0, errors.New("not implemented")
}

type tokenVersionSessionTestStore struct {
	mu             sync.Mutex
	version        int64
	getErr         error
	cacheErr       error
	cacheMiss      bool
	getCachedCalls int
	cacheCalls     int
}

func (s *tokenVersionSessionTestStore) GetCachedTokenVersion(context.Context, string) (int64, error) {
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

func (s *tokenVersionSessionTestStore) CacheTokenVersion(context.Context, string, int64) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.cacheCalls++
	return s.cacheErr
}

func (s *tokenVersionSessionTestStore) DeleteCachedTokenVersion(context.Context, string) error {
	return nil
}
