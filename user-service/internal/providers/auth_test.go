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
