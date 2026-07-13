package providers

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	serviceconfig "github.com/aegiscore/user-service/internal/config"
)

func TestNewJWTServiceRejectsInvalidTokenTTLPolicy(t *testing.T) {
	cfg := &serviceconfig.Config{Auth: serviceconfig.AuthConfig{JWT: serviceconfig.JWTConfig{
		Secret:          "secret",
		AccessTokenTTL:  time.Hour,
		RefreshTokenTTL: time.Hour,
	}}}

	_, err := NewJWTService(cfg)
	require.ErrorContains(t, err, "auth jwt refresh_token_ttl must be greater than access_token_ttl")

}

func TestNewJWTServiceAcceptsValidTokenTTLPolicy(t *testing.T) {
	cfg := &serviceconfig.Config{Auth: serviceconfig.AuthConfig{JWT: serviceconfig.JWTConfig{
		Secret:          "secret",
		AccessTokenTTL:  15 * time.Minute,
		RefreshTokenTTL: time.Hour,
	}}}

	service, err := NewJWTService(cfg)
	require.NoError(t, err)
	require.NotNil(t, service)

}

func TestNewPasswordServiceUsesConfiguredKDFBudget(t *testing.T) {
	cfg := &serviceconfig.Config{Auth: serviceconfig.AuthConfig{
		PasswordKDF: serviceconfig.PasswordKDFConfig{Argon2Concurrency: 1, Argon2QueueSize: 1},
	}}

	service, err := NewPasswordService(cfg)
	require.NoError(t, err)
	require.NotNil(t, service)

}

func TestNewPasswordServiceRejectsInvalidKDFBudget(t *testing.T) {
	cfg := &serviceconfig.Config{Auth: serviceconfig.AuthConfig{
		PasswordKDF: serviceconfig.PasswordKDFConfig{Argon2Concurrency: 2, Argon2QueueSize: 1},
	}}

	_, err := NewPasswordService(cfg)
	require.ErrorContains(t, err, "password argon2 queue size must be >= concurrency")

}
