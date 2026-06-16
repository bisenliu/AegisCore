package redis

import "testing"

func TestKeyCatalogBuildsRBACPolicyKeys(t *testing.T) {
	catalog, err := NewKeyCatalog(" aegiscore-user-services ")
	if err != nil {
		t.Fatalf("NewKeyCatalog: %v", err)
	}

	if got := catalog.PolicyVersionKey(); got != "aegiscore-user-services:rbac:policy:version" {
		t.Fatalf("PolicyVersionKey = %q", got)
	}
	if got := catalog.PolicyChannel(); got != "aegiscore-user-services:rbac:policy:refresh" {
		t.Fatalf("PolicyChannel = %q", got)
	}
}
