package password

import (
	"errors"
	"strings"
	"testing"
)

func TestHashAndVerify(t *testing.T) {
	hash, err := Hash("secret")
	if err != nil {
		t.Fatalf("Hash: %v", err)
	}
	if hash == "secret" {
		t.Fatal("hash must not equal plain password")
	}
	if !strings.HasPrefix(hash, "$argon2id$v=19$m=65536,t=3,p=4$") {
		t.Fatalf("unexpected hash format: %q", hash)
	}

	ok, err := Verify("secret", hash)
	if err != nil {
		t.Fatalf("Verify matching: %v", err)
	}
	if !ok {
		t.Fatal("Verify matching = false")
	}
}

func TestVerifyRejectsWrongPassword(t *testing.T) {
	hash, err := Hash("secret")
	if err != nil {
		t.Fatalf("Hash: %v", err)
	}
	ok, err := Verify("wrong", hash)
	if err != nil {
		t.Fatalf("Verify wrong: %v", err)
	}
	if ok {
		t.Fatal("Verify wrong = true")
	}
}

func TestVerifyRejectsMalformedHash(t *testing.T) {
	ok, err := Verify("secret", "not-a-hash")
	if err == nil {
		t.Fatal("Verify malformed error = nil")
	}
	if !errors.Is(err, ErrInvalidHash) {
		t.Fatalf("Verify malformed error = %v", err)
	}
	if ok {
		t.Fatal("Verify malformed = true")
	}
}

func TestVerifyRejectsOversizedHash(t *testing.T) {
	ok, err := Verify("secret", "$argon2id$v=19$m=65536,t=3,p=4$"+strings.Repeat("a", 513))
	if !errors.Is(err, ErrInvalidHash) {
		t.Fatalf("Verify oversized error = %v", err)
	}
	if ok {
		t.Fatal("Verify oversized = true")
	}
}

func TestVerifyRejectsMalformedParams(t *testing.T) {
	hash, err := Hash("secret")
	if err != nil {
		t.Fatalf("Hash: %v", err)
	}

	malformed := strings.Replace(hash, "m=65536,t=3,p=4", "m=65536,t=3,p=4=bad", 1)
	ok, err := Verify("secret", malformed)
	if !errors.Is(err, ErrInvalidHash) {
		t.Fatalf("Verify malformed params error = %v", err)
	}
	if ok {
		t.Fatal("Verify malformed params = true")
	}
}

func TestHashRejectsEmptyPassword(t *testing.T) {
	_, err := Hash("")
	if !errors.Is(err, ErrEmptyPassword) {
		t.Fatalf("Hash empty error = %v", err)
	}
}

func TestVerifyRejectsEmptyPassword(t *testing.T) {
	hash, err := Hash("secret")
	if err != nil {
		t.Fatalf("Hash: %v", err)
	}
	ok, err := Verify("", hash)
	if !errors.Is(err, ErrEmptyPassword) {
		t.Fatalf("Verify empty error = %v", err)
	}
	if ok {
		t.Fatal("Verify empty = true")
	}
}
