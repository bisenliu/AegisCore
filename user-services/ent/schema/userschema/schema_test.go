package userschema

import (
	"testing"

	"github.com/aegiscore/user-services/internal/user"
)

func TestUserStatusDefaultMatchesDomainNormalStatus(t *testing.T) {
	for _, userField := range Fields() {
		desc := userField.Descriptor()
		if desc.Name != "status" {
			continue
		}

		defaultStatus, ok := desc.Default.(int64)
		if !ok {
			t.Fatalf("status default has type %T, want int64", desc.Default)
		}
		if defaultStatus != int64(user.UserStatusNormal) {
			t.Fatalf("status default = %d, want UserStatusNormal %d", defaultStatus, user.UserStatusNormal)
		}
		return
	}

	t.Fatal("status field not found")
}
