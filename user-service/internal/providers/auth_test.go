package providers

import (
	"strings"
	"testing"
	"time"

	"github.com/aegiscore/common/runtime/config"
)

func TestNewJWTServiceRejectsInvalidTokenTTLPolicy(t *testing.T) {
	cfg := &config.Config{Auth: config.AuthConfig{JWT: config.JWTConfig{
		Secret:          "secret",
		AccessTokenTTL:  time.Hour,
		RefreshTokenTTL: time.Hour,
	}}}

	_, err := NewJWTService(cfg)
	if err == nil {
		t.Fatal("NewJWTService error = nil")
	}
	if !strings.Contains(err.Error(), "auth jwt refresh_token_ttl must be greater than access_token_ttl") {
		t.Fatalf("NewJWTService error = %q, want refresh ttl policy", err.Error())
	}
}

func TestNewJWTServiceAcceptsValidTokenTTLPolicy(t *testing.T) {
	cfg := &config.Config{Auth: config.AuthConfig{JWT: config.JWTConfig{
		Secret:          "secret",
		AccessTokenTTL:  15 * time.Minute,
		RefreshTokenTTL: time.Hour,
	}}}

	service, err := NewJWTService(cfg)
	if err != nil {
		t.Fatalf("NewJWTService: %v", err)
	}
	if service == nil {
		t.Fatal("NewJWTService = nil")
	}
}

func TestNewPasswordServiceUsesConfiguredKDFBudget(t *testing.T) {
	cfg := &config.Config{Auth: config.AuthConfig{
		PasswordKDF: config.PasswordKDFConfig{Argon2Concurrency: 1, Argon2QueueSize: 1},
	}}

	service, err := NewPasswordService(cfg)
	if err != nil {
		t.Fatalf("NewPasswordService: %v", err)
	}
	if service == nil {
		t.Fatal("NewPasswordService = nil")
	}
}

func TestNewPasswordServiceRejectsInvalidKDFBudget(t *testing.T) {
	cfg := &config.Config{Auth: config.AuthConfig{
		PasswordKDF: config.PasswordKDFConfig{Argon2Concurrency: 2, Argon2QueueSize: 1},
	}}

	_, err := NewPasswordService(cfg)
	if err == nil {
		t.Fatal("NewPasswordService error = nil")
	}
	if !strings.Contains(err.Error(), "password argon2 queue size must be >= concurrency") {
		t.Fatalf("NewPasswordService error = %q, want queue policy", err.Error())
	}
}
