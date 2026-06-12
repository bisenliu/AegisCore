package rediskey

import (
	"errors"
	"testing"
)

func TestBuilderKeyUsesNamespaceScopeAndHashTag(t *testing.T) {
	builder, err := NewBuilder(Options{Namespace: " aegiscore-user-services "})
	if err != nil {
		t.Fatalf("NewBuilder: %v", err)
	}
	scoped := builder.MustScoped("auth")

	got := scoped.MustKey("session", HashTag("u-123"), "s-123")
	if got != "aegiscore-user-services:auth:session:{u-123}:s-123" {
		t.Fatalf("Key = %q", got)
	}
}

func TestBuilderOmitsEmptyNamespace(t *testing.T) {
	builder, err := NewBuilder(Options{Namespace: "   "})
	if err != nil {
		t.Fatalf("NewBuilder: %v", err)
	}

	got := builder.MustKey("auth", "user", "token_version", HashTag("u-123"))
	if got != "auth:user:token_version:{u-123}" {
		t.Fatalf("Key = %q", got)
	}
}

func TestBuilderPrefixAddsTrailingSeparator(t *testing.T) {
	builder := MustBuilder(Options{Namespace: "aegiscore-user-services"}).MustScoped("auth")

	got := builder.MustPrefix("session", HashTag("u-123"))
	if got != "aegiscore-user-services:auth:session:{u-123}:" {
		t.Fatalf("Prefix = %q", got)
	}
}

func TestBuilderAllowsSeparatorInsideHashTag(t *testing.T) {
	builder := MustBuilder(Options{}).MustScoped("external")

	got := builder.MustKey("ref", HashTag("github:user:123"))
	if got != "external:ref:{github:user:123}" {
		t.Fatalf("Key = %q", got)
	}
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
			if !errors.Is(err, ErrInvalidSegment) {
				t.Fatalf("error = %v, want ErrInvalidSegment", err)
			}
		})
	}
}

func TestMustKeyPanicsWhenSegmentInvalid(t *testing.T) {
	defer func() {
		if recovered := recover(); recovered == nil {
			t.Fatal("MustKey did not panic")
		}
	}()

	builder := MustBuilder(Options{})
	_ = builder.MustKey("auth", "")
}
