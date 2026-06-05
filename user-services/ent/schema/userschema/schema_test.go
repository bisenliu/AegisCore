package userschema

import (
	"testing"

	"github.com/aegiscore/user-services/internal/domain"
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
		if defaultStatus != int64(domain.UserStatusNormal) {
			t.Fatalf("status default = %d, want domain.UserStatusNormal %d", defaultStatus, domain.UserStatusNormal)
		}
		return
	}

	t.Fatal("status field not found")
}
