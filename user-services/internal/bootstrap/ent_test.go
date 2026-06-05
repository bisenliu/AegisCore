package bootstrap

import (
	"errors"
	"strings"
	"testing"
)

func TestCloseEntClientsPreservesBothNamedErrors(t *testing.T) {
	userErr := errors.New("user close failed")
	commonErr := errors.New("common close failed")

	err := closeEntClients(
		func() error { return userErr },
		func() error { return commonErr },
	)
	if err == nil {
		t.Fatal("closeEntClients error = nil")
	}
	if !errors.Is(err, userErr) {
		t.Fatalf("closeEntClients error = %v, want user close error", err)
	}
	if !errors.Is(err, commonErr) {
		t.Fatalf("closeEntClients error = %v, want common close error", err)
	}
	if !strings.Contains(err.Error(), "close user_db ent client") {
		t.Fatalf("closeEntClients error = %q, want user_db context", err.Error())
	}
	if !strings.Contains(err.Error(), "close common_db ent client") {
		t.Fatalf("closeEntClients error = %q, want common_db context", err.Error())
	}
}

func TestCloseEntClientsClosesBothWhenOneFails(t *testing.T) {
	userErr := errors.New("user close failed")
	userClosed := false
	commonClosed := false

	err := closeEntClients(
		func() error {
			userClosed = true
			return userErr
		},
		func() error {
			commonClosed = true
			return nil
		},
	)
	if err == nil {
		t.Fatal("closeEntClients error = nil")
	}
	if !userClosed {
		t.Fatal("user close was not called")
	}
	if !commonClosed {
		t.Fatal("common close was not called")
	}
	if !errors.Is(err, userErr) {
		t.Fatalf("closeEntClients error = %v, want user close error", err)
	}
	if !strings.Contains(err.Error(), "close user_db ent client") {
		t.Fatalf("closeEntClients error = %q, want user_db context", err.Error())
	}
}
