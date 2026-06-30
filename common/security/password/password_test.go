package password

import (
	"context"
	"encoding/base64"
	"errors"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestNewServiceRejectsInvalidOptions(t *testing.T) {
	tests := []struct {
		name string
		opts Options
	}{
		{name: "zero concurrency", opts: Options{Concurrency: 0, QueueSize: 1}},
		{name: "negative concurrency", opts: Options{Concurrency: -1, QueueSize: 1}},
		{name: "zero queue", opts: Options{Concurrency: 1, QueueSize: 0}},
		{name: "negative queue", opts: Options{Concurrency: 1, QueueSize: -1}},
		{name: "queue smaller than concurrency", opts: Options{Concurrency: 2, QueueSize: 1}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if _, err := NewService(tt.opts); err == nil {
				t.Fatal("NewService error = nil, want error")
			}
		})
	}
}

func TestHashContextAndVerifyContext(t *testing.T) {
	service := newTestService(t, Options{Concurrency: 1, QueueSize: 1})
	hash, err := service.HashContext(context.Background(), "secret")
	if err != nil {
		t.Fatalf("HashContext: %v", err)
	}
	if hash == "secret" {
		t.Fatal("hash must not equal plain password")
	}
	if !strings.HasPrefix(hash, "$argon2id$v=19$m=65536,t=3,p=4$") {
		t.Fatalf("unexpected hash format: %q", hash)
	}

	ok, err := service.VerifyContext(context.Background(), "secret", hash)
	if err != nil {
		t.Fatalf("VerifyContext matching: %v", err)
	}
	if !ok {
		t.Fatal("VerifyContext matching = false")
	}
}

func TestVerifyContextRejectsWrongPassword(t *testing.T) {
	service := newTestService(t, Options{Concurrency: 1, QueueSize: 1})
	hash, err := service.HashContext(context.Background(), "secret")
	if err != nil {
		t.Fatalf("HashContext: %v", err)
	}
	ok, err := service.VerifyContext(context.Background(), "wrong", hash)
	if err != nil {
		t.Fatalf("VerifyContext wrong: %v", err)
	}
	if ok {
		t.Fatal("VerifyContext wrong = true")
	}
}

func TestVerifyContextRejectsMalformedHash(t *testing.T) {
	service := newTestService(t, Options{Concurrency: 1, QueueSize: 1})
	ok, err := service.VerifyContext(context.Background(), "secret", "not-a-hash")
	if err == nil {
		t.Fatal("VerifyContext malformed error = nil")
	}
	if !errors.Is(err, ErrInvalidHash) {
		t.Fatalf("VerifyContext malformed error = %v", err)
	}
	if ok {
		t.Fatal("VerifyContext malformed = true")
	}
}

func TestVerifyContextRejectsOversizedHash(t *testing.T) {
	service := newTestService(t, Options{Concurrency: 1, QueueSize: 1})
	ok, err := service.VerifyContext(context.Background(), "secret", "$argon2id$v=19$m=65536,t=3,p=4$"+strings.Repeat("a", 513))
	if !errors.Is(err, ErrInvalidHash) {
		t.Fatalf("VerifyContext oversized error = %v", err)
	}
	if ok {
		t.Fatal("VerifyContext oversized = true")
	}
}

func TestVerifyContextRejectsMalformedParams(t *testing.T) {
	service := newTestService(t, Options{Concurrency: 1, QueueSize: 1})
	hash, err := service.HashContext(context.Background(), "secret")
	if err != nil {
		t.Fatalf("HashContext: %v", err)
	}

	malformed := strings.Replace(hash, "m=65536,t=3,p=4", "m=65536,t=3,p=4=bad", 1)
	ok, err := service.VerifyContext(context.Background(), "secret", malformed)
	if !errors.Is(err, ErrInvalidHash) {
		t.Fatalf("VerifyContext malformed params error = %v", err)
	}
	if ok {
		t.Fatal("VerifyContext malformed params = true")
	}
}

func TestVerifyContextRejectsUnsupportedParams(t *testing.T) {
	service := newTestService(t, Options{Concurrency: 1, QueueSize: 1})
	hash, err := service.HashContext(context.Background(), "secret")
	if err != nil {
		t.Fatalf("HashContext: %v", err)
	}

	unsupported := strings.Replace(hash, "m=65536,t=3,p=4", "m=32768,t=3,p=4", 1)
	ok, err := service.VerifyContext(context.Background(), "secret", unsupported)
	if !errors.Is(err, ErrInvalidHash) {
		t.Fatalf("VerifyContext unsupported params error = %v", err)
	}
	if ok {
		t.Fatal("VerifyContext unsupported params = true")
	}
}

func TestVerifyContextRejectsUnsupportedSaltLength(t *testing.T) {
	service := newTestService(t, Options{Concurrency: 1, QueueSize: 1})
	hash, err := service.HashContext(context.Background(), "secret")
	if err != nil {
		t.Fatalf("HashContext: %v", err)
	}
	parts := strings.Split(hash, "$")
	parts[4] = base64.RawStdEncoding.EncodeToString([]byte("short"))

	ok, err := service.VerifyContext(context.Background(), "secret", strings.Join(parts, "$"))
	if !errors.Is(err, ErrInvalidHash) {
		t.Fatalf("VerifyContext unsupported salt length error = %v", err)
	}
	if ok {
		t.Fatal("VerifyContext unsupported salt length = true")
	}
}

func TestVerifyContextRejectsUnsupportedKeyLength(t *testing.T) {
	service := newTestService(t, Options{Concurrency: 1, QueueSize: 1})
	hash, err := service.HashContext(context.Background(), "secret")
	if err != nil {
		t.Fatalf("HashContext: %v", err)
	}
	parts := strings.Split(hash, "$")
	parts[5] = base64.RawStdEncoding.EncodeToString([]byte("short"))

	ok, err := service.VerifyContext(context.Background(), "secret", strings.Join(parts, "$"))
	if !errors.Is(err, ErrInvalidHash) {
		t.Fatalf("VerifyContext unsupported key length error = %v", err)
	}
	if ok {
		t.Fatal("VerifyContext unsupported key length = true")
	}
}

func TestHashContextRejectsEmptyPassword(t *testing.T) {
	service := newTestService(t, Options{Concurrency: 1, QueueSize: 1})
	_, err := service.HashContext(context.Background(), "")
	if !errors.Is(err, ErrEmptyPassword) {
		t.Fatalf("HashContext empty error = %v", err)
	}
}

func TestVerifyContextRejectsEmptyPassword(t *testing.T) {
	service := newTestService(t, Options{Concurrency: 1, QueueSize: 1})
	hash, err := service.HashContext(context.Background(), "secret")
	if err != nil {
		t.Fatalf("HashContext: %v", err)
	}
	ok, err := service.VerifyContext(context.Background(), "", hash)
	if !errors.Is(err, ErrEmptyPassword) {
		t.Fatalf("VerifyContext empty error = %v", err)
	}
	if ok {
		t.Fatal("VerifyContext empty = true")
	}
}

func TestHashContextRejectsOversizedPassword(t *testing.T) {
	service := newTestService(t, Options{Concurrency: 1, QueueSize: 1})
	_, err := service.HashContext(context.Background(), strings.Repeat("a", maxPasswordLength+1))
	if !errors.Is(err, ErrPasswordTooLong) {
		t.Fatalf("HashContext oversized error = %v", err)
	}
}

func TestVerifyContextRejectsOversizedPassword(t *testing.T) {
	service := newTestService(t, Options{Concurrency: 1, QueueSize: 1})
	hash, err := service.HashContext(context.Background(), "secret")
	if err != nil {
		t.Fatalf("HashContext: %v", err)
	}
	ok, err := service.VerifyContext(context.Background(), strings.Repeat("a", maxPasswordLength+1), hash)
	if !errors.Is(err, ErrPasswordTooLong) {
		t.Fatalf("VerifyContext oversized error = %v", err)
	}
	if ok {
		t.Fatal("VerifyContext oversized = true")
	}
}

func TestHashContextReturnsBusyWhenQueueIsFull(t *testing.T) {
	service := newTestService(t, Options{Concurrency: 1, QueueSize: 1})
	service.queue <- struct{}{}

	_, err := service.HashContext(context.Background(), "secret")
	if !errors.Is(err, ErrPasswordKDFBusy) {
		t.Fatalf("HashContext busy error = %v", err)
	}
}

func TestHashContextCancelsWhileWaitingForKDFSlot(t *testing.T) {
	service := newTestService(t, Options{Concurrency: 1, QueueSize: 1})
	service.gate <- struct{}{}
	ctx, cancel := context.WithTimeout(context.Background(), time.Millisecond)
	defer cancel()

	_, err := service.HashContext(ctx, "secret")
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("HashContext canceled error = %v", err)
	}
	if got := len(service.queue); got != 0 {
		t.Fatalf("service.queue len = %d, want 0", got)
	}
}

func TestServiceInstancesDoNotShareKDFControls(t *testing.T) {
	blocked := newTestService(t, Options{Concurrency: 1, QueueSize: 1})
	available := newTestService(t, Options{Concurrency: 1, QueueSize: 1})
	blocked.queue <- struct{}{}

	var wg sync.WaitGroup
	wg.Add(2)

	var blockedErr error
	go func() {
		defer wg.Done()
		_, blockedErr = blocked.HashContext(context.Background(), "secret")
	}()

	var availableErr error
	go func() {
		defer wg.Done()
		_, availableErr = available.HashContext(context.Background(), "secret")
	}()

	wg.Wait()
	if !errors.Is(blockedErr, ErrPasswordKDFBusy) {
		t.Fatalf("blocked HashContext error = %v, want %v", blockedErr, ErrPasswordKDFBusy)
	}
	if availableErr != nil {
		t.Fatalf("available HashContext error = %v", availableErr)
	}
}

func newTestService(t *testing.T, opts Options) *Service {
	t.Helper()
	service, err := NewService(opts)
	if err != nil {
		t.Fatalf("NewService: %v", err)
	}
	return service
}
