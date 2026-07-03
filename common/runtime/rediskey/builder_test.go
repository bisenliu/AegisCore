package rediskey

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestBuilderKeyUsesNamespaceScopeAndHashTag(t *testing.T) {
	builder, err := NewBuilder(Options{Namespace: " aegiscore-user-services "})
	require.NoError(t, err)
	scoped := builder.MustScoped("auth")

	got := scoped.MustKey("session", HashTag("u-123"), "s-123")
	require.Equal(t, "aegiscore-user-services:auth:session:{u-123}:s-123", got)
}

func TestBuilderOmitsEmptyNamespace(t *testing.T) {
	builder, err := NewBuilder(Options{Namespace: "   "})
	require.NoError(t, err)

	got := builder.MustKey("auth", "user", "token_version", HashTag("u-123"))
	require.Equal(t, "auth:user:token_version:{u-123}", got)
}

func TestBuilderPrefixAddsTrailingSeparator(t *testing.T) {
	builder := MustBuilder(Options{Namespace: "aegiscore-user-services"}).MustScoped("auth")

	got := builder.MustPrefix("session", HashTag("u-123"))
	require.Equal(t, "aegiscore-user-services:auth:session:{u-123}:", got)
}

func TestBuilderAllowsSeparatorInsideHashTag(t *testing.T) {
	builder := MustBuilder(Options{}).MustScoped("external")

	got := builder.MustKey("ref", HashTag("github:user:123"))
	require.Equal(t, "external:ref:{github:user:123}", got)
}

func TestBuilderRejectsInvalidSegments(t *testing.T) {
	tests := []struct {
		name  string
		build func() error
	}{
		{
			name: "namespace with separator",
			build: func() error {
				_, err := NewBuilder(Options{Namespace: "bad:namespace"})
				return err
			},
		},
		{
			name: "empty part",
			build: func() error {
				builder := MustBuilder(Options{})
				_, err := builder.Key("auth", "")
				return err
			},
		},
		{
			name: "part with separator",
			build: func() error {
				builder := MustBuilder(Options{})
				_, err := builder.Key("auth:user")
				return err
			},
		},
		{
			name: "empty hash tag",
			build: func() error {
				builder := MustBuilder(Options{})
				_, err := builder.Key("auth", HashTag(""))
				return err
			},
		},
		{
			name: "malformed hash tag",
			build: func() error {
				builder := MustBuilder(Options{})
				_, err := builder.Key("auth", "user{1}")
				return err
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.build()
			require.ErrorIs(t, err, ErrInvalidSegment)
		})
	}
}

func TestMustKeyPanicsWhenSegmentInvalid(t *testing.T) {
	builder := MustBuilder(Options{})
	require.Panics(t, func() { _ = builder.MustKey("auth", "") })
}
