package id

import (
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

func TestNewUUIDUsesCurrentDefaultVersion(t *testing.T) {
	got, err := NewUUID()
	require.NoError(t, err)
	require.NotEqual(t, uuid.Nil, got)
	require.Equal(t, uuid.Version(7), got.Version())
}

func TestNewUUIDString(t *testing.T) {
	got, err := NewUUIDString()
	require.NoError(t, err)
	parsed, err := uuid.Parse(got)
	require.NoError(t, err)
	require.Equal(t, uuid.Version(7), parsed.Version())
}

func TestMustNewUUIDString(t *testing.T) {
	got := MustNewUUIDString()
	_, err := uuid.Parse(got)
	require.NoError(t, err)
}
