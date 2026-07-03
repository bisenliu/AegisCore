package auth

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestValidateTokenVersion(t *testing.T) {
	require.NoError(t, ValidateTokenVersion(2, 2))

	err := ValidateTokenVersion(1, 2)
	require.ErrorIs(t, err, ErrTokenVersionMismatch)

	var mismatch *TokenVersionMismatchError
	require.ErrorAs(t, err, &mismatch)
	require.Equal(t, int64(1), mismatch.Token)
	require.Equal(t, int64(2), mismatch.Current)
}
