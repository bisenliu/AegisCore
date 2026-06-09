package domain

import (
	"testing"

	"github.com/aegiscore/common/runtime/config"
)

func TestRedisKeyBuilderUsesAppNamePrefix(t *testing.T) {
	builder := NewRedisKeyBuilder(&config.Config{App: config.AppConfig{Name: " aegiscore-user-services "}})

	if got := builder.AuthSession("s-123"); got != "aegiscore-user-services:auth:session:s-123" {
		t.Fatalf("AuthSession = %q", got)
	}
	if got := builder.AuthUserTokenVersion("u-123"); got != "aegiscore-user-services:auth:user:u-123:token_version" {
		t.Fatalf("AuthUserTokenVersion = %q", got)
	}
	if got := builder.AuthUserSessions("u-123"); got != "aegiscore-user-services:auth:user:u-123:sessions" {
		t.Fatalf("AuthUserSessions = %q", got)
	}
}

func TestRedisKeyBuilderKeepsUnprefixedKeysWhenAppNameEmpty(t *testing.T) {
	builder := NewRedisKeyBuilder(&config.Config{App: config.AppConfig{Name: "   "}})

	if got := builder.AuthSession("s-123"); got != "auth:session:s-123" {
		t.Fatalf("AuthSession = %q", got)
	}
	if got := builder.AuthUserTokenVersion("u-123"); got != "auth:user:u-123:token_version" {
		t.Fatalf("AuthUserTokenVersion = %q", got)
	}
	if got := builder.AuthUserSessions("u-123"); got != "auth:user:u-123:sessions" {
		t.Fatalf("AuthUserSessions = %q", got)
	}
}
