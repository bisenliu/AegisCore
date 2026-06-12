package redis

import "testing"

func TestKeyCatalogUsesAppNamePrefixWithNewKeyFormat(t *testing.T) {
	keys := MustKeyCatalog(" aegiscore-user-services ")

	if got := keys.AuthSession("u-123", "s-123"); got != "aegiscore-user-services:auth:session:{u-123}:s-123" {
		t.Fatalf("AuthSession = %q", got)
	}
	if got := keys.AuthSessionPrefix("u-123"); got != "aegiscore-user-services:auth:session:{u-123}:" {
		t.Fatalf("AuthSessionPrefix = %q", got)
	}
	if got := keys.AuthUserTokenVersion("u-123"); got != "aegiscore-user-services:auth:user:token_version:{u-123}" {
		t.Fatalf("AuthUserTokenVersion = %q", got)
	}
	if got := keys.AuthUserSessions("u-123"); got != "aegiscore-user-services:auth:user:sessions:{u-123}" {
		t.Fatalf("AuthUserSessions = %q", got)
	}
	if got := keys.AuthUserSessionsPurge("u-123", "p-123"); got != "aegiscore-user-services:auth:user:sessions:{u-123}:purge:p-123" {
		t.Fatalf("AuthUserSessionsPurge = %q", got)
	}
}

func TestKeyCatalogKeepsUnprefixedKeysWhenAppNameEmpty(t *testing.T) {
	keys := MustKeyCatalog("   ")

	if got := keys.AuthSession("u-123", "s-123"); got != "auth:session:{u-123}:s-123" {
		t.Fatalf("AuthSession = %q", got)
	}
	if got := keys.AuthUserTokenVersion("u-123"); got != "auth:user:token_version:{u-123}" {
		t.Fatalf("AuthUserTokenVersion = %q", got)
	}
	if got := keys.AuthUserSessions("u-123"); got != "auth:user:sessions:{u-123}" {
		t.Fatalf("AuthUserSessions = %q", got)
	}
	if got := keys.AuthUserSessionsPurge("u-123", "p-123"); got != "auth:user:sessions:{u-123}:purge:p-123" {
		t.Fatalf("AuthUserSessionsPurge = %q", got)
	}
}
