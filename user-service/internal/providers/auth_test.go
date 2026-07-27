package providers

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	serviceconfig "github.com/aegiscore/user-service/internal/config"
)

func TestNewJWTServiceRejectsInvalidTokenTTLPolicy(t *testing.T) {
	settings := serviceconfig.AuthSettings{JWT: serviceconfig.JWTConfig{
		Secret:          "secret",
		AccessTokenTTL:  time.Hour,
		RefreshTokenTTL: time.Hour,
	}}

	_, err := NewJWTService(settings)
	require.ErrorContains(t, err, "auth jwt refresh_token_ttl must be greater than access_token_ttl")

}

func TestNewJWTServiceAcceptsValidTokenTTLPolicy(t *testing.T) {
	settings := serviceconfig.AuthSettings{JWT: serviceconfig.JWTConfig{
		Secret:          "secret",
		AccessTokenTTL:  15 * time.Minute,
		RefreshTokenTTL: time.Hour,
	}}

	service, err := NewJWTService(settings)
	require.NoError(t, err)
	require.NotNil(t, service)

}
