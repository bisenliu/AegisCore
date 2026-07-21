package redis

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestKeyCatalogUsesAppNamePrefixWithNewKeyFormat(t *testing.T) {
	keys := mustKeyCatalog(" aegiscore-user-service ")
	{

		got := keys.AuthSession("u-123", "s-123")
		require.Equal(t, "aegiscore-user-service:auth:session:{u-123}:s-123", got,
			"AuthSession = %q", got)
	}
	{

		got := keys.PasswordChangeSession("u-123", "pc-123")
		require.Equal(t, "aegiscore-user-service:auth:password_change_session:{u-123}:pc-123", got,
			"PasswordChangeSession = %q", got)
	}
	{

		got := keys.AuthSessionPrefix("u-123")
		require.Equal(t, "aegiscore-user-service:auth:session:{u-123}:", got,
			"AuthSessionPrefix = %q", got)
	}
	{

		got := keys.AuthUserTokenVersion("u-123")
		require.Equal(t, "aegiscore-user-service:auth:user:token_version:{u-123}", got,
			"AuthUserTokenVersion = %q", got)
	}
	{

		got := keys.AuthUserSessions("u-123")
		require.Equal(t, "aegiscore-user-service:auth:user:sessions:{u-123}", got,
			"AuthUserSessions = %q", got)
	}
	{

		got := keys.AuthUserSessionsPurge("u-123", "p-123")
		require.Equal(t, "aegiscore-user-service:auth:user:sessions:{u-123}:purge:p-123", got,
			"AuthUserSessionsPurge = %q", got)
	}

}

func TestKeyCatalogKeepsUnprefixedKeysWhenAppNameEmpty(t *testing.T) {
	keys := mustKeyCatalog("   ")
	{

		got := keys.AuthSession("u-123", "s-123")
		require.Equal(t, "auth:session:{u-123}:s-123", got,
			"AuthSession = %q", got)
	}
	{

		got := keys.PasswordChangeSession("u-123", "pc-123")
		require.Equal(t, "auth:password_change_session:{u-123}:pc-123", got,
			"PasswordChangeSession = %q", got)
	}
	{

		got := keys.AuthUserTokenVersion("u-123")
		require.Equal(t, "auth:user:token_version:{u-123}", got,
			"AuthUserTokenVersion = %q", got)
	}
	{

		got := keys.AuthUserSessions("u-123")
		require.Equal(t, "auth:user:sessions:{u-123}", got,
			"AuthUserSessions = %q", got)
	}
	{

		got := keys.AuthUserSessionsPurge("u-123", "p-123")
		require.Equal(t, "auth:user:sessions:{u-123}:purge:p-123", got,
			"AuthUserSessionsPurge = %q", got)
	}

}

func mustKeyCatalog(appName string) KeyCatalog {
	catalog, err := NewKeyCatalog(appName)
	if err != nil {
		panic(err)
	}
	return catalog
}
