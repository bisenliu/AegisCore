package repository

import "testing"

func TestUserListPredicates(t *testing.T) {
	if got := userListPredicates(ListUsersInput{}); len(got) != 0 {
		t.Fatalf("predicates = %d, want 0", len(got))
	}

	active := true
	got := userListPredicates(ListUsersInput{Name: "Ali", Email: "alice@example.com", Active: &active})
	if len(got) != 3 {
		t.Fatalf("predicates = %d, want 3", len(got))
	}
}
