package password

import (
	"context"
	"encoding/base64"
	"errors"
	"strings"
	"testing"
	"time"
)

func TestHashContextAndVerifyContext(t *testing.T) {
	hash, err := HashContext(context.Background(), "secret")
	if err != nil {
		t.Fatalf("HashContext: %v", err)
	}
	if hash == "secret" {
		t.Fatal("hash must not equal plain password")
	}
	if !strings.HasPrefix(hash, "$argon2id$v=19$m=65536,t=3,p=4$") {
		t.Fatalf("unexpected hash format: %q", hash)
	}

	ok, err := VerifyContext(context.Background(), "secret", hash)
	if err != nil {
		t.Fatalf("VerifyContext matching: %v", err)
	}
	if !ok {
		t.Fatal("VerifyContext matching = false")
	}
}

func TestVerifyContextRejectsWrongPassword(t *testing.T) {
	hash, err := HashContext(context.Background(), "secret")
	if err != nil {
		t.Fatalf("HashContext: %v", err)
	}
	ok, err := VerifyContext(context.Background(), "wrong", hash)
	if err != nil {
		t.Fatalf("VerifyContext wrong: %v", err)
	}
	if ok {
		t.Fatal("VerifyContext wrong = true")
	}
}

func TestVerifyContextRejectsMalformedHash(t *testing.T) {
	ok, err := VerifyContext(context.Background(), "secret", "not-a-hash")
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
	ok, err := VerifyContext(context.Background(), "secret", "$argon2id$v=19$m=65536,t=3,p=4$"+strings.Repeat("a", 513))
	if !errors.Is(err, ErrInvalidHash) {
		t.Fatalf("VerifyContext oversized error = %v", err)
	}
	if ok {
		t.Fatal("VerifyContext oversized = true")
	}
}

func TestVerifyContextRejectsMalformedParams(t *testing.T) {
	hash, err := HashContext(context.Background(), "secret")
	if err != nil {
		t.Fatalf("HashContext: %v", err)
	}

	malformed := strings.Replace(hash, "m=65536,t=3,p=4", "m=65536,t=3,p=4=bad", 1)
	ok, err := VerifyContext(context.Background(), "secret", malformed)
	if !errors.Is(err, ErrInvalidHash) {
		t.Fatalf("VerifyContext malformed params error = %v", err)
	}
	if ok {
		t.Fatal("VerifyContext malformed params = true")
	}
}

func TestVerifyContextRejectsUnsupportedParams(t *testing.T) {
	hash, err := HashContext(context.Background(), "secret")
	if err != nil {
		t.Fatalf("HashContext: %v", err)
	}

	unsupported := strings.Replace(hash, "m=65536,t=3,p=4", "m=32768,t=3,p=4", 1)
	ok, err := VerifyContext(context.Background(), "secret", unsupported)
	if !errors.Is(err, ErrInvalidHash) {
		t.Fatalf("VerifyContext unsupported params error = %v", err)
	}
	if ok {
		t.Fatal("VerifyContext unsupported params = true")
	}
}

func TestVerifyContextRejectsUnsupportedSaltLength(t *testing.T) {
	hash, err := HashContext(context.Background(), "secret")
	if err != nil {
		t.Fatalf("HashContext: %v", err)
	}
	parts := strings.Split(hash, "$")
	parts[4] = base64.RawStdEncoding.EncodeToString([]byte("short"))

	ok, err := VerifyContext(context.Background(), "secret", strings.Join(parts, "$"))
	if !errors.Is(err, ErrInvalidHash) {
		t.Fatalf("VerifyContext unsupported salt length error = %v", err)
	}
	if ok {
		t.Fatal("VerifyContext unsupported salt length = true")
	}
}

func TestVerifyContextRejectsUnsupportedKeyLength(t *testing.T) {
	hash, err := HashContext(context.Background(), "secret")
	if err != nil {
		t.Fatalf("HashContext: %v", err)
	}
	parts := strings.Split(hash, "$")
	parts[5] = base64.RawStdEncoding.EncodeToString([]byte("short"))

	ok, err := VerifyContext(context.Background(), "secret", strings.Join(parts, "$"))
	if !errors.Is(err, ErrInvalidHash) {
		t.Fatalf("VerifyContext unsupported key length error = %v", err)
	}
	if ok {
		t.Fatal("VerifyContext unsupported key length = true")
	}
}

func TestHashContextRejectsEmptyPassword(t *testing.T) {
	_, err := HashContext(context.Background(), "")
	if !errors.Is(err, ErrEmptyPassword) {
		t.Fatalf("HashContext empty error = %v", err)
	}
}

func TestVerifyContextRejectsEmptyPassword(t *testing.T) {
	hash, err := HashContext(context.Background(), "secret")
	if err != nil {
		t.Fatalf("HashContext: %v", err)
	}
	ok, err := VerifyContext(context.Background(), "", hash)
	if !errors.Is(err, ErrEmptyPassword) {
		t.Fatalf("VerifyContext empty error = %v", err)
	}
	if ok {
		t.Fatal("VerifyContext empty = true")
	}
}

func TestHashContextRejectsOversizedPassword(t *testing.T) {
	_, err := HashContext(context.Background(), strings.Repeat("a", maxPasswordLength+1))
	if !errors.Is(err, ErrPasswordTooLong) {
		t.Fatalf("HashContext oversized error = %v", err)
	}
}

func TestVerifyContextRejectsOversizedPassword(t *testing.T) {
	hash, err := HashContext(context.Background(), "secret")
	if err != nil {
		t.Fatalf("HashContext: %v", err)
	}
	ok, err := VerifyContext(context.Background(), strings.Repeat("a", maxPasswordLength+1), hash)
	if !errors.Is(err, ErrPasswordTooLong) {
		t.Fatalf("VerifyContext oversized error = %v", err)
	}
	if ok {
		t.Fatal("VerifyContext oversized = true")
	}
}

func TestHashContextReturnsBusyWhenQueueIsFull(t *testing.T) {
	resetArgon2Controls(t, 1, 1)
	argon2Queue <- struct{}{}

	_, err := HashContext(context.Background(), "secret")
	if !errors.Is(err, ErrPasswordKDFBusy) {
		t.Fatalf("HashContext busy error = %v", err)
	}
}

func TestHashContextCancelsWhileWaitingForKDFSlot(t *testing.T) {
	resetArgon2Controls(t, 1, 1)
	argon2Gate <- struct{}{}
	ctx, cancel := context.WithTimeout(context.Background(), time.Millisecond)
	defer cancel()

	_, err := HashContext(ctx, "secret")
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("HashContext canceled error = %v", err)
	}
	if got := len(argon2Queue); got != 0 {
		t.Fatalf("argon2Queue len = %d, want 0", got)
	}
}

func resetArgon2Controls(t *testing.T, gateSize, queueSize int) {
	t.Helper()
	oldGate := argon2Gate
	oldQueue := argon2Queue
	argon2Gate = make(chan struct{}, gateSize)
	argon2Queue = make(chan struct{}, queueSize)
	t.Cleanup(func() {
		argon2Gate = oldGate
		argon2Queue = oldQueue
	})
}
