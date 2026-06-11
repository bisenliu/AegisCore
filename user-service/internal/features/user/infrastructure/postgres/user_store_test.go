package postgres

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"github.com/aegiscore/user-service/ent/enttest"
	userapplication "github.com/aegiscore/user-service/internal/features/user/application"
	userdomain "github.com/aegiscore/user-service/internal/features/user/domain"
	"github.com/google/uuid"
	_ "github.com/mattn/go-sqlite3"
)

const deletedAtForTest int64 = 1710000000000

func TestUserRepositoryDomainErrors(t *testing.T) {
	repo := newTestUserStore(t)

	t.Run("get by user id returns domain not found", func(t *testing.T) {
		_, err := repo.GetByUserID(context.Background(), uuid.MustParse("018f0000-0000-7000-8000-000000000999"))

		if !errors.Is(err, userdomain.ErrUserNotFound) {
			t.Fatalf("err = %v, want userdomain.ErrUserNotFound", err)
		}
	})

	t.Run("get by user id ignores soft deleted user", func(t *testing.T) {
		ctx := context.Background()
		userID := uuid.MustParse("018f0000-0000-7000-8000-000000000010")
		createSoftDeletedUser(t, repo, userapplication.CreateUserInput{Nickname: "Deleted Profile", UserID: userID, Username: "deleted-profile", PasswordHash: "hash", Status: userdomain.UserStatusNormal})

		_, err := repo.GetByUserID(ctx, userID)

		if !errors.Is(err, userdomain.ErrUserNotFound) {
			t.Fatalf("err = %v, want userdomain.ErrUserNotFound", err)
		}
	})

	t.Run("create uniqueness violation returns domain already exists", func(t *testing.T) {
		ctx := context.Background()
		_, err := repo.Create(ctx, userapplication.CreateUserInput{Nickname: "Alice", UserID: uuid.MustParse("018f0000-0000-7000-8000-000000000001"), Username: "alice", PasswordHash: "hash", Status: userdomain.UserStatusNormal})
		if err != nil {
			t.Fatalf("Create initial user: %v", err)
		}

		_, err = repo.Create(ctx, userapplication.CreateUserInput{Nickname: "Alice 2", UserID: uuid.MustParse("018f0000-0000-7000-8000-000000000002"), Username: "alice", PasswordHash: "hash", Status: userdomain.UserStatusNormal})

		if !errors.Is(err, userdomain.ErrUserAlreadyExists) {
			t.Fatalf("err = %v, want userdomain.ErrUserAlreadyExists", err)
		}
	})

	t.Run("create duplicate user id returns domain already exists", func(t *testing.T) {
		ctx := context.Background()
		userID := uuid.MustParse("018f0000-0000-7000-8000-000000000013")
		_, err := repo.Create(ctx, userapplication.CreateUserInput{Nickname: "Duplicate ID", UserID: userID, Username: "duplicate-id", PasswordHash: "hash", Status: userdomain.UserStatusNormal})
		if err != nil {
			t.Fatalf("Create initial user: %v", err)
		}

		_, err = repo.Create(ctx, userapplication.CreateUserInput{Nickname: "Duplicate ID 2", UserID: userID, Username: "duplicate-id-2", PasswordHash: "hash", Status: userdomain.UserStatusNormal})

		if !errors.Is(err, userdomain.ErrUserAlreadyExists) {
			t.Fatalf("err = %v, want userdomain.ErrUserAlreadyExists", err)
		}
	})

	t.Run("soft deleted username remains reserved", func(t *testing.T) {
		ctx := context.Background()
		createSoftDeletedUser(t, repo, userapplication.CreateUserInput{Nickname: "Deleted Alice", UserID: uuid.MustParse("018f0000-0000-7000-8000-000000000003"), Username: "reserved-alice", PasswordHash: "hash", Status: userdomain.UserStatusNormal})

		_, err := repo.Create(ctx, userapplication.CreateUserInput{Nickname: "New Alice", UserID: uuid.MustParse("018f0000-0000-7000-8000-000000000004"), Username: "reserved-alice", PasswordHash: "hash", Status: userdomain.UserStatusNormal})

		if !errors.Is(err, userdomain.ErrUserAlreadyExists) {
			t.Fatalf("err = %v, want userdomain.ErrUserAlreadyExists", err)
		}
	})
}

func TestUserRepositoryReturnsDomainUsers(t *testing.T) {
	repo := newTestUserStore(t)
	ctx := context.Background()
	userID := uuid.MustParse("018f0000-0000-7000-8000-000000000101")

	created, err := repo.Create(ctx, userapplication.CreateUserInput{Nickname: "Domain Alice", UserID: userID, Username: "domain-alice", PasswordHash: "hash", Status: userdomain.UserStatusMustChangePassword})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if created.ID == 0 || created.UserID != userID || created.Nickname != "Domain Alice" || created.Username != "domain-alice" || created.PasswordHash != "hash" || created.Status != userdomain.UserStatusMustChangePassword || created.TokenVersion != 1 || created.CreatedAt == 0 || created.UpdatedAt == 0 {
		t.Fatalf("created = %#v", created)
	}

	byID, err := repo.GetByUserID(ctx, userID)
	if err != nil {
		t.Fatalf("GetByUserID: %v", err)
	}
	assertSameUser(t, byID, created)

	status := userdomain.UserStatusMustChangePassword
	users, hasNext, err := repo.ListUsers(ctx, userapplication.ListUsersInput{Limit: 10, Username: "domain-alice", Status: &status})
	if err != nil {
		t.Fatalf("ListUsers: %v", err)
	}
	if hasNext || len(users) != 1 || users[0].UserID != userID || users[0].Username != "domain-alice" || users[0].Status != userdomain.UserStatusMustChangePassword {
		t.Fatalf("users=%#v hasNext=%v", users, hasNext)
	}
}

func TestUserRepositoryListUsersBoundaries(t *testing.T) {
	t.Run("empty page", func(t *testing.T) {
		repo := newTestUserStore(t)

		users, hasNext, err := repo.ListUsers(context.Background(), userapplication.ListUsersInput{Limit: 10})

		if err != nil {
			t.Fatalf("ListUsers: %v", err)
		}
		if hasNext || len(users) != 0 {
			t.Fatalf("users=%#v hasNext=%v, want empty page", users, hasNext)
		}
	})

	t.Run("paginates with stable user id order", func(t *testing.T) {
		repo := newTestUserStore(t)
		ctx := context.Background()
		first := createTestUser(t, repo, userapplication.CreateUserInput{Nickname: "Page One", UserID: uuid.MustParse("018f0000-0000-7000-8000-000000000201"), Username: "page-one", PasswordHash: "hash-1", Status: userdomain.UserStatusNormal})
		second := createTestUser(t, repo, userapplication.CreateUserInput{Nickname: "Page Two", UserID: uuid.MustParse("018f0000-0000-7000-8000-000000000202"), Username: "page-two", PasswordHash: "hash-2", Status: userdomain.UserStatusDisabled})
		third := createTestUser(t, repo, userapplication.CreateUserInput{Nickname: "Page Three", UserID: uuid.MustParse("018f0000-0000-7000-8000-000000000203"), Username: "page-three", PasswordHash: "hash-3", Status: userdomain.UserStatusMustChangePassword})

		users, hasNext, err := repo.ListUsers(ctx, userapplication.ListUsersInput{Limit: 2})

		if err != nil {
			t.Fatalf("ListUsers: %v", err)
		}
		if !hasNext || len(users) != 2 {
			t.Fatalf("users=%#v hasNext=%v, want 2 with next page", users, hasNext)
		}
		if first.UserID.String() >= second.UserID.String() || second.UserID.String() >= third.UserID.String() {
			t.Fatalf("seed user IDs not increasing: first=%s second=%s third=%s", first.UserID, second.UserID, third.UserID)
		}
		assertSameUserValue(t, users[0], first)
		assertSameUserValue(t, users[1], second)

		users, hasNext, err = repo.ListUsers(ctx, userapplication.ListUsersInput{AfterUserID: &second.UserID, Limit: 2})
		if err != nil {
			t.Fatalf("ListUsers after cursor: %v", err)
		}
		if hasNext || len(users) != 1 {
			t.Fatalf("users=%#v hasNext=%v, want final page", users, hasNext)
		}
		assertSameUserValue(t, users[0], third)
	})

	t.Run("filters by nickname username status and combined predicates", func(t *testing.T) {
		repo := newTestUserStore(t)
		ctx := context.Background()
		createTestUser(t, repo, userapplication.CreateUserInput{Nickname: "Alice Ops", UserID: uuid.MustParse("018f0000-0000-7000-8000-000000000211"), Username: "alice-ops", PasswordHash: "hash-1", Status: userdomain.UserStatusNormal})
		createTestUser(t, repo, userapplication.CreateUserInput{Nickname: "Alice Audit", UserID: uuid.MustParse("018f0000-0000-7000-8000-000000000212"), Username: "alice-audit", PasswordHash: "hash-2", Status: userdomain.UserStatusDisabled})
		createTestUser(t, repo, userapplication.CreateUserInput{Nickname: "Bob Ops", UserID: uuid.MustParse("018f0000-0000-7000-8000-000000000213"), Username: "bob-ops", PasswordHash: "hash-3", Status: userdomain.UserStatusMustChangePassword})

		assertListUsernames(t, repo, userapplication.ListUsersInput{Limit: 10, Nickname: "Alice"}, []string{"alice-ops", "alice-audit"})
		assertListUsernames(t, repo, userapplication.ListUsersInput{Limit: 10, Username: "bob-ops"}, []string{"bob-ops"})

		disabled := userdomain.UserStatusDisabled
		assertListUsernames(t, repo, userapplication.ListUsersInput{Limit: 10, Status: &disabled}, []string{"alice-audit"})
		assertListUsernames(t, repo, userapplication.ListUsersInput{Limit: 10, Nickname: "Alice", Username: "alice-audit", Status: &disabled}, []string{"alice-audit"})

		users, hasNext, err := repo.ListUsers(ctx, userapplication.ListUsersInput{Limit: 10, Nickname: "Missing"})
		if err != nil {
			t.Fatalf("ListUsers missing filter: %v", err)
		}
		if hasNext || len(users) != 0 {
			t.Fatalf("users=%#v hasNext=%v, want no matches", users, hasNext)
		}
	})

	t.Run("excludes soft deleted users from rows and has next", func(t *testing.T) {
		repo := newTestUserStore(t)
		ctx := context.Background()
		active := createTestUser(t, repo, userapplication.CreateUserInput{Nickname: "Active Alice", UserID: uuid.MustParse("018f0000-0000-7000-8000-000000000221"), Username: "active-alice", PasswordHash: "hash", Status: userdomain.UserStatusNormal})
		createSoftDeletedUser(t, repo, userapplication.CreateUserInput{Nickname: "Deleted Alice", UserID: uuid.MustParse("018f0000-0000-7000-8000-000000000222"), Username: "deleted-alice-list", PasswordHash: "hash", Status: userdomain.UserStatusNormal})

		users, hasNext, err := repo.ListUsers(ctx, userapplication.ListUsersInput{Limit: 10, Nickname: "Alice"})

		if err != nil {
			t.Fatalf("ListUsers: %v", err)
		}
		if hasNext || len(users) != 1 {
			t.Fatalf("users=%#v hasNext=%v, want only active user", users, hasNext)
		}
		assertSameUserValue(t, users[0], active)
	})
}

func TestUserListPredicates(t *testing.T) {
	if got := buildListPredicates(userapplication.ListUsersInput{}); len(got) != 1 {
		t.Fatalf("predicates = %d, want 1", len(got))
	}

	status := userdomain.UserStatusNormal
	got := buildListPredicates(userapplication.ListUsersInput{Nickname: "Ali", Username: "alice", Status: &status})
	if len(got) != 4 {
		t.Fatalf("predicates = %d, want 4", len(got))
	}
}

func newTestUserStore(t *testing.T) *userStore {
	t.Helper()
	// The store currently uses portable Ent query/update semantics, so SQLite
	// covers the integration boundary without requiring Docker-only PostgreSQL tests.
	client := enttest.Open(t, "sqlite3", fmt.Sprintf("file:user_store_test_%s?mode=memory&cache=shared&_fk=1", uuid.NewString()))
	t.Cleanup(func() { _ = client.Close() })
	return NewUserStore(UserStoreParams{Client: client})
}

func createTestUser(t *testing.T, repo *userStore, input userapplication.CreateUserInput) *userdomain.User {
	t.Helper()
	created, err := repo.Create(context.Background(), input)
	if err != nil {
		t.Fatalf("Create user %q: %v", input.Username, err)
	}
	return created
}

func createSoftDeletedUser(t *testing.T, repo *userStore, input userapplication.CreateUserInput) {
	t.Helper()
	ctx := context.Background()
	created, err := repo.Create(ctx, input)
	if err != nil {
		t.Fatalf("Create soft deleted user: %v", err)
	}
	if _, err := repo.client.User.UpdateOneID(created.ID).SetDeletedAt(deletedAtForTest).Save(ctx); err != nil {
		t.Fatalf("soft delete user: %v", err)
	}
}

func assertSameUser(t *testing.T, got, want *userdomain.User) {
	t.Helper()
	if got == nil || want == nil {
		t.Fatalf("got=%#v want=%#v", got, want)
	}
	assertSameUserValue(t, *got, want)
}

func assertSameUserValue(t *testing.T, got userdomain.User, want *userdomain.User) {
	t.Helper()
	if want == nil {
		t.Fatal("want user is nil")
	}
	if got.ID != want.ID || got.UserID != want.UserID || got.Nickname != want.Nickname || got.Username != want.Username || got.PasswordHash != want.PasswordHash || got.Status != want.Status || got.TokenVersion != want.TokenVersion || got.CreatedAt != want.CreatedAt || got.UpdatedAt != want.UpdatedAt {
		t.Fatalf("got user = %#v, want %#v", got, want)
	}
}

func assertListUsernames(t *testing.T, repo *userStore, input userapplication.ListUsersInput, want []string) {
	t.Helper()
	users, hasNext, err := repo.ListUsers(context.Background(), input)
	if err != nil {
		t.Fatalf("ListUsers(%#v): %v", input, err)
	}
	if hasNext || len(users) != len(want) {
		t.Fatalf("users=%#v hasNext=%v, want usernames %v", users, hasNext, want)
	}
	for i := range want {
		if users[i].Username != want[i] {
			t.Fatalf("users[%d].Username = %q, want %q; users=%#v", i, users[i].Username, want[i], users)
		}
	}
}
