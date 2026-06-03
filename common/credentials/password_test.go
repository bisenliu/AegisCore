package credentials

import (
	"errors"
	"strings"
	"testing"
)

func TestHashAndVerifyPassword(t *testing.T) {
	hash, err := HashPassword("secret")
	if err != nil {
		t.Fatalf("HashPassword: %v", err)
	}
	if hash == "secret" {
		t.Fatal("hash must not equal plain password")
	}
	if !strings.HasPrefix(hash, "$argon2id$v=19$m=65536,t=3,p=4$") {
		t.Fatalf("unexpected hash format: %q", hash)
	}

	ok, err := VerifyPassword("secret", hash)
	if err != nil {
		t.Fatalf("VerifyPassword matching: %v", err)
	}
	if !ok {
		t.Fatal("VerifyPassword matching = false")
	}
}

func TestVerifyPasswordRejectsWrongPassword(t *testing.T) {
	hash, err := HashPassword("secret")
	if err != nil {
		t.Fatalf("HashPassword: %v", err)
	}
	ok, err := VerifyPassword("wrong", hash)
	if err != nil {
		t.Fatalf("VerifyPassword wrong: %v", err)
	}
	if ok {
		t.Fatal("VerifyPassword wrong = true")
	}
}

func TestVerifyPasswordRejectsMalformedHash(t *testing.T) {
	ok, err := VerifyPassword("secret", "not-a-hash")
	if err == nil {
		t.Fatal("VerifyPassword malformed error = nil")
	}
	if !errors.Is(err, ErrInvalidHash) {
		t.Fatalf("VerifyPassword malformed error = %v", err)
	}
	if ok {
		t.Fatal("VerifyPassword malformed = true")
	}
}

func TestVerifyPasswordRejectsOversizedHash(t *testing.T) {
	ok, err := VerifyPassword("secret", "$argon2id$v=19$m=65536,t=3,p=4$"+strings.Repeat("a", 513))
	if !errors.Is(err, ErrInvalidHash) {
		t.Fatalf("VerifyPassword oversized error = %v", err)
	}
	if ok {
		t.Fatal("VerifyPassword oversized = true")
	}
}

func TestVerifyPasswordRejectsMalformedParams(t *testing.T) {
	hash, err := HashPassword("secret")
	if err != nil {
		t.Fatalf("HashPassword: %v", err)
	}

	malformed := strings.Replace(hash, "m=65536,t=3,p=4", "m=65536,t=3,p=4=bad", 1)
	ok, err := VerifyPassword("secret", malformed)
	if !errors.Is(err, ErrInvalidHash) {
		t.Fatalf("VerifyPassword malformed params error = %v", err)
	}
	if ok {
		t.Fatal("VerifyPassword malformed params = true")
	}
}

func TestHashPasswordRejectsEmptyPassword(t *testing.T) {
	_, err := HashPassword("")
	if !errors.Is(err, ErrEmptyPassword) {
		t.Fatalf("HashPassword empty error = %v", err)
	}
}

func TestVerifyPasswordRejectsEmptyPassword(t *testing.T) {
	hash, err := HashPassword("secret")
	if err != nil {
		t.Fatalf("HashPassword: %v", err)
	}
	ok, err := VerifyPassword("", hash)
	if !errors.Is(err, ErrEmptyPassword) {
		t.Fatalf("VerifyPassword empty error = %v", err)
	}
	if ok {
		t.Fatal("VerifyPassword empty = true")
	}
}
