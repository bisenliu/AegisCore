package id

import (
	"testing"

	"github.com/google/uuid"
)

func TestNewUUIDUsesCurrentDefaultVersion(t *testing.T) {
	got, err := NewUUID()
	if err != nil {
		t.Fatalf("NewUUID: %v", err)
	}
	if got == uuid.Nil {
		t.Fatal("NewUUID returned nil UUID")
	}
	if got.Version() != 7 {
		t.Fatalf("UUID version = %d, want 7", got.Version())
	}
}

func TestNewUUIDString(t *testing.T) {
	got, err := NewUUIDString()
	if err != nil {
		t.Fatalf("NewUUIDString: %v", err)
	}
	parsed, err := uuid.Parse(got)
	if err != nil {
		t.Fatalf("Parse generated UUID: %v", err)
	}
	if parsed.Version() != 7 {
		t.Fatalf("UUID string version = %d, want 7", parsed.Version())
	}
}

func TestMustNewUUIDString(t *testing.T) {
	got := MustNewUUIDString()
	if _, err := uuid.Parse(got); err != nil {
		t.Fatalf("Parse MustNewUUIDString: %v", err)
	}
}
