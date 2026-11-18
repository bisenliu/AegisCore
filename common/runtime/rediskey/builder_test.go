package rediskey

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestBuilderKeyUsesNamespaceScopeAndHashTag(t *testing.T) {
	builder, err := NewBuilder(Options{
		Namespace: " aegiscore-user-service ",
		Scope:     []string{"auth"},
	})
	require.NoError(t, err)

	got := builder.MustKey("session", HashTag("u-123"), "s-123")
	require.Equal(t, "aegiscore-user-service:auth:session:{u-123}:s-123", got)
}

func TestBuilderScopedDerivation(t *testing.T) {
	builder, err := NewBuilder(Options{Namespace: "aegiscore"})
	require.NoError(t, err)

	scoped := builder.MustScoped("auth")
	got := scoped.MustKey("session", "s-1")
	require.Equal(t, "aegiscore:auth:session:s-1", got)

	// 空分段追加应返回原实例
	noop, err := scoped.Scoped()
	require.NoError(t, err)
	require.Equal(t, scoped.MustKey("test"), noop.MustKey("test"))
}

func TestBuilderOmitsEmptyNamespace(t *testing.T) {
	builder, err := NewBuilder(Options{Namespace: "   "})
	require.NoError(t, err)

	got := builder.MustKey("auth", "user", "token_version", HashTag("u-123"))
	require.Equal(t, "auth:user:token_version:{u-123}", got)
}

func TestBuilderPrefixAddsTrailingSeparator(t *testing.T) {
	builder := MustBuilder(Options{
		Namespace: "aegiscore-user-service",
		Scope:     []string{"auth"},
	})

	got := builder.MustPrefix("session", HashTag("u-123"))
	require.Equal(t, "aegiscore-user-service:auth:session:{u-123}:", got)

	// 无参数时返回固定前缀带冒号
	gotBase := builder.MustPrefix()
	require.Equal(t, "aegiscore-user-service:auth:", gotBase)
}

func TestBuilderAllowsSeparatorInsideHashTag(t *testing.T) {
	builder := MustBuilder(Options{Scope: []string{"external"}})

	got := builder.MustKey("ref", HashTag("github:user:123"))
	require.Equal(t, "external:ref:{github:user:123}", got)
}

func TestBuilderZeroValueFallback(t *testing.T) {
	var builder Builder
	got, err := builder.Key("plain", "key")
	require.NoError(t, err)
	require.Equal(t, "plain:key", got)

	// 零值且无参数返回错误
	_, err = builder.Key()
	require.ErrorIs(t, err, ErrInvalidSegment)
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
			name: "scope with separator in options",
			build: func() error {
				_, err := NewBuilder(Options{Scope: []string{"bad:scope"}})
				return err
			},
		},
		{
			name: "scope with separator in Scoped",
			build: func() error {
				builder := MustBuilder(Options{})
				_, err := builder.Scoped("bad:scope")
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
			name: "malformed hash tag missing prefix brace",
			build: func() error {
				builder := MustBuilder(Options{})
				_, err := builder.Key("auth", "user{1}")
				return err
			},
		},
		{
			name: "malformed hash tag missing suffix brace",
			build: func() error {
				builder := MustBuilder(Options{})
				_, err := builder.Key("auth", "{1}user")
				return err
			},
		},
		{
			name: "malformed hash tag multiple braces",
			build: func() error {
				builder := MustBuilder(Options{})
				_, err := builder.Key("auth", "{1}{2}")
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

func TestMustPanics(t *testing.T) {
	require.Panics(t, func() { _ = MustBuilder(Options{Namespace: "bad:name"}) })
	require.Panics(t, func() { _ = MustBuilder(Options{}).MustScoped("bad:scope") })
	require.Panics(t, func() { _ = MustBuilder(Options{}).MustKey("auth", "") })
	require.Panics(t, func() { _ = MustBuilder(Options{}).MustPrefix("auth", "") })
}

func TestPrefixError(t *testing.T) {
	builder := MustBuilder(Options{})
	_, err := builder.Prefix("bad:part")
	require.ErrorIs(t, err, ErrInvalidSegment)
}

func BenchmarkBuilderKey(b *testing.B) {
	builder := MustBuilder(Options{
		Namespace: "aegiscore-user-service",
		Scope:     []string{"auth"},
	})
	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		_ = builder.MustKey("session", HashTag("u-123"), "s-123")
	}
}

func BenchmarkBuilderKey_Plain(b *testing.B) {
	builder := MustBuilder(Options{
		Namespace: "aegiscore-user-service",
		Scope:     []string{"auth"},
	})
	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		_ = builder.MustKey("session", "user", "s-123")
	}
}
