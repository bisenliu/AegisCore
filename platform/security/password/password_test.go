package password

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
	"golang.org/x/crypto/bcrypt"
)

func TestHashContextAndVerifyContext(t *testing.T) {
	service := newTestService(t)
	hash, err := service.HashContext(context.Background(), "secret")
	require.NoError(t, err)
	require.NotEqual(t, "secret", hash)
	require.True(t, strings.HasPrefix(hash, "$2"))

	cost, err := bcrypt.Cost([]byte(hash))
	require.NoError(t, err)
	require.Equal(t, defaultBcryptCost, cost)

	ok, err := service.VerifyContext(context.Background(), "secret", hash)
	require.NoError(t, err)
	require.True(t, ok)
}

func TestVerifyContextRejectsWrongPassword(t *testing.T) {
	service := newTestService(t)
	hash, err := service.HashContext(context.Background(), "secret")
	require.NoError(t, err)

	ok, err := service.VerifyContext(context.Background(), "wrong", hash)
	require.NoError(t, err)
	require.False(t, ok)
}

func TestVerifyContextRejectsMalformedHash(t *testing.T) {
	service := newTestService(t)
	ok, err := service.VerifyContext(context.Background(), "secret", "not-a-hash")
	require.ErrorIs(t, err, ErrInvalidHash)
	require.False(t, ok)
}

func TestVerifyContextRejectsArgon2idHash(t *testing.T) {
	service := newTestService(t)
	argon2idHash := "$argon2id$v=19$m=65536,t=3,p=4$AAAAAAAAAAAAAAAAAAAAAA$AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA"

	ok, err := service.VerifyContext(context.Background(), "secret", argon2idHash)
	require.ErrorIs(t, err, ErrInvalidHash)
	require.False(t, ok)
}

func TestVerifyContextRejectsOversizedHash(t *testing.T) {
	service := newTestService(t)
	ok, err := service.VerifyContext(context.Background(), "secret", strings.Repeat("a", maxEncodedHashLength+1))
	require.ErrorIs(t, err, ErrInvalidHash)
	require.False(t, ok)
}

func TestHashContextRejectsEmptyPassword(t *testing.T) {
	service := newTestService(t)
	_, err := service.HashContext(context.Background(), "")
	require.ErrorIs(t, err, ErrEmptyPassword)
}

func TestVerifyContextRejectsEmptyPassword(t *testing.T) {
	service := newTestService(t)
	hash, err := service.HashContext(context.Background(), "secret")
	require.NoError(t, err)

	ok, err := service.VerifyContext(context.Background(), "", hash)
	require.ErrorIs(t, err, ErrEmptyPassword)
	require.False(t, ok)
}

func TestHashContextRejectsOversizedPassword(t *testing.T) {
	service := newTestService(t)
	_, err := service.HashContext(context.Background(), strings.Repeat("a", maxPasswordLength+1))
	require.ErrorIs(t, err, ErrPasswordTooLong)
}

func TestVerifyContextRejectsOversizedPassword(t *testing.T) {
	service := newTestService(t)
	hash, err := service.HashContext(context.Background(), "secret")
	require.NoError(t, err)

	ok, err := service.VerifyContext(context.Background(), strings.Repeat("a", maxPasswordLength+1), hash)
	require.ErrorIs(t, err, ErrPasswordTooLong)
	require.False(t, ok)
}

func TestHashContextReturnsContextErrorBeforeBcrypt(t *testing.T) {
	service := newTestService(t)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := service.HashContext(ctx, "secret")
	require.ErrorIs(t, err, context.Canceled)
}

func TestVerifyContextReturnsContextErrorBeforeBcrypt(t *testing.T) {
	service := newTestService(t)
	hash, err := service.HashContext(context.Background(), "secret")
	require.NoError(t, err)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	ok, err := service.VerifyContext(ctx, "secret", hash)
	require.ErrorIs(t, err, context.Canceled)
	require.False(t, ok)
}

func TestPasswordErrorsRemainStable(t *testing.T) {
	require.True(t, errors.Is(ErrEmptyPassword, ErrEmptyPassword))
	require.True(t, errors.Is(ErrPasswordTooLong, ErrPasswordTooLong))
	require.True(t, errors.Is(ErrInvalidHash, ErrInvalidHash))
}

func newTestService(t *testing.T) *Service {
	t.Helper()
	service, err := NewService()
	require.NoError(t, err)
	return service
}
