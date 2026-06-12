package domain

import "testing"

func TestRedisKeyBuilderUsesAppNamePrefixWithNewKeyFormat(t *testing.T) {
	builder := NewRedisKeyBuilder(" aegiscore-user-services ")

	if got := builder.AuthSession("u-123", "s-123"); got != "aegiscore-user-services:auth:session:{u-123}:s-123" {
		t.Fatalf("AuthSession = %q", got)
	}
	if got := builder.AuthSessionPrefix("u-123"); got != "aegiscore-user-services:auth:session:{u-123}:" {
		t.Fatalf("AuthSessionPrefix = %q", got)
	}
	if got := builder.AuthUserTokenVersion("u-123"); got != "aegiscore-user-services:auth:user:token_version:{u-123}" {
		t.Fatalf("AuthUserTokenVersion = %q", got)
	}
	if got := builder.AuthUserSessions("u-123"); got != "aegiscore-user-services:auth:user:sessions:{u-123}" {
		t.Fatalf("AuthUserSessions = %q", got)
	}
	if got := builder.AuthUserSessionsPurge("u-123", "p-123"); got != "aegiscore-user-services:auth:user:sessions:{u-123}:purge:p-123" {
		t.Fatalf("AuthUserSessionsPurge = %q", got)
	}
}

func TestRedisKeyBuilderKeepsUnprefixedKeysWhenAppNameEmpty(t *testing.T) {
	builder := NewRedisKeyBuilder("   ")

	if got := builder.AuthSession("u-123", "s-123"); got != "auth:session:{u-123}:s-123" {
		t.Fatalf("AuthSession = %q", got)
	}
	if got := builder.AuthUserTokenVersion("u-123"); got != "auth:user:token_version:{u-123}" {
		t.Fatalf("AuthUserTokenVersion = %q", got)
	}
	if got := builder.AuthUserSessions("u-123"); got != "auth:user:sessions:{u-123}" {
		t.Fatalf("AuthUserSessions = %q", got)
	}
	if got := builder.AuthUserSessionsPurge("u-123", "p-123"); got != "auth:user:sessions:{u-123}:purge:p-123" {
		t.Fatalf("AuthUserSessionsPurge = %q", got)
	}
}
