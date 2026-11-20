package id

import (
	"encoding/json"
	"testing"
	"uuid"

	"github.com/stretchr/testify/require"
)

func TestNewUUIDUsesCurrentDefaultVersion(t *testing.T) {
	got, err := NewUUID()
	require.NoError(t, err)
	require.NotEqual(t, uuid.Nil(), got)
	require.Equal(t, byte(7), got[6]>>4)
	encoded, err := json.Marshal(got)
	require.NoError(t, err)
	require.Equal(t, `"`+got.String()+`"`, string(encoded))
	var decoded uuid.UUID
	require.NoError(t, json.Unmarshal(encoded, &decoded))
	require.Equal(t, got, decoded)
}

func TestNewUUIDString(t *testing.T) {
	got, err := NewUUIDString()
	require.NoError(t, err)
	parsed, err := uuid.Parse(got)
	require.NoError(t, err)
	require.Equal(t, byte(7), parsed[6]>>4)
	require.Equal(t, parsed.String(), got)
}

func TestMustNewUUIDString(t *testing.T) {
	got := MustNewUUIDString()
	_, err := uuid.Parse(got)
	require.NoError(t, err)
}

func TestInvalidUUIDString(t *testing.T) {
	_, err := uuid.Parse("invalid")
	require.Error(t, err)
}
