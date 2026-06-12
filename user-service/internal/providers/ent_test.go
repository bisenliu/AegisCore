package providers

import (
	"errors"
	"strings"
	"testing"
)

func TestCloseEntClientPreservesNamedError(t *testing.T) {
	userErr := errors.New("user close failed")

	err := closeEntClient("user_db", func() error { return userErr })
	if err == nil {
		t.Fatal("closeEntClient error = nil")
	}
	if !errors.Is(err, userErr) {
		t.Fatalf("closeEntClient error = %v, want user close error", err)
	}
	if !strings.Contains(err.Error(), "close user_db ent client") {
		t.Fatalf("closeEntClient error = %q, want user_db context", err.Error())
	}
}

func TestCloseEntClientCallsCloser(t *testing.T) {
	closed := false

	err := closeEntClient("user_db", func() error {
		closed = true
		return nil
	})
	if err != nil {
		t.Fatalf("closeEntClient: %v", err)
	}
	if !closed {
		t.Fatal("client close was not called")
	}
}
