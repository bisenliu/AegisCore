package repository

import (
	"testing"

	"github.com/aegiscore/user-services/internal/domain"
)

func TestUserListPredicates(t *testing.T) {
	if got := userListPredicates(ListUsersInput{}); len(got) != 1 {
		t.Fatalf("predicates = %d, want 1", len(got))
	}

	status := domain.UserStatusNormal
	got := userListPredicates(ListUsersInput{Nickname: "Ali", Username: "alice", Status: &status})
	if len(got) != 4 {
		t.Fatalf("predicates = %d, want 4", len(got))
	}
}
