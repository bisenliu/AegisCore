package auth

import (
	"errors"
	"testing"
)

func TestValidateTokenVersion(t *testing.T) {
	if err := ValidateTokenVersion(2, 2); err != nil {
		t.Fatalf("ValidateTokenVersion matched version: %v", err)
	}

	err := ValidateTokenVersion(1, 2)
	if !errors.Is(err, ErrTokenVersionMismatch) {
		t.Fatalf("err = %v, want ErrTokenVersionMismatch", err)
	}

	var mismatch *TokenVersionMismatchError
	if !errors.As(err, &mismatch) {
		t.Fatalf("err = %T, want TokenVersionMismatchError", err)
	}
	if mismatch.Token != 1 || mismatch.Current != 2 {
		t.Fatalf("mismatch = %#v, want token=1 current=2", mismatch)
	}
}
