package schema

import (
	"testing"

	"github.com/aegiscore/user-service/internal/shared/identity"
)

func TestUserStatusDefaultMatchesDomainNormalStatus(t *testing.T) {
	for _, userField := range (User{}).Fields() {
		desc := userField.Descriptor()
		if desc.Name != "status" {
			continue
		}

		defaultStatus, ok := desc.Default.(int64)
		if !ok {
			t.Fatalf("status default has type %T, want int64", desc.Default)
		}
		if defaultStatus != int64(identity.UserStatusNormal) {
			t.Fatalf("status default = %d, want UserStatusNormal %d", defaultStatus, identity.UserStatusNormal)
		}
		return
	}

	t.Fatal("status field not found")
}
