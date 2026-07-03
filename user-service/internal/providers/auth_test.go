package providers

import (
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/aegiscore/common/runtime/config"
)

func TestNewJWTServiceRejectsInvalidTokenTTLPolicy(t *testing.T) {
	cfg := &config.Config{Auth: config.AuthConfig{JWT: config.JWTConfig{
		Secret:          "secret",
		AccessTokenTTL:  time.Hour,
		RefreshTokenTTL: time.Hour,
	}}}

	_, err := NewJWTService(cfg)
	require.Error(t, err,
		"NewJWTService error = nil")
	require.True(t, strings.Contains(err.Error(), "auth jwt refresh_token_ttl must be greater than access_token_ttl"),
		"NewJWTService error = %q, want refresh ttl policy", err.Error())

}

func TestNewJWTServiceAcceptsValidTokenTTLPolicy(t *testing.T) {
	cfg := &config.Config{Auth: config.AuthConfig{JWT: config.JWTConfig{
		Secret:          "secret",
		AccessTokenTTL:  15 * time.Minute,
		RefreshTokenTTL: time.Hour,
	}}}

	service, err := NewJWTService(cfg)
	require.NoError(t, err,
		"NewJWTService: %v", err)
	require.NotNil(t, service,
		"NewJWTService = nil")

}

func TestNewPasswordServiceUsesConfiguredKDFBudget(t *testing.T) {
	cfg := &config.Config{Auth: config.AuthConfig{
		PasswordKDF: config.PasswordKDFConfig{Argon2Concurrency: 1, Argon2QueueSize: 1},
	}}

	service, err := NewPasswordService(cfg)
	require.NoError(t, err,
		"NewPasswordService: %v", err)
	require.NotNil(t, service,
		"NewPasswordService = nil")

}

func TestNewPasswordServiceRejectsInvalidKDFBudget(t *testing.T) {
	cfg := &config.Config{Auth: config.AuthConfig{
		PasswordKDF: config.PasswordKDFConfig{Argon2Concurrency: 2, Argon2QueueSize: 1},
	}}

	_, err := NewPasswordService(cfg)
	require.Error(t, err,
		"NewPasswordService error = nil")
	require.True(t, strings.Contains(err.Error(), "password argon2 queue size must be >= concurrency"),
		"NewPasswordService error = %q, want queue policy", err.Error())

}
