package postgres

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"github.com/aegiscore/user-services/ent/enttest"
	entuser "github.com/aegiscore/user-services/ent/user"
	"github.com/aegiscore/user-services/internal/auth"
	"github.com/aegiscore/user-services/internal/user"
	"github.com/google/uuid"
	_ "github.com/mattn/go-sqlite3"
)

const deletedAtForTest int64 = 1710000000000

func TestUserRepositoryDomainErrors(t *testing.T) {
	repo := newTestUserStore(t)

	t.Run("get by user id returns domain not found", func(t *testing.T) {
		_, err := repo.GetByUserID(context.Background(), uuid.MustParse("018f0000-0000-7000-8000-000000000999"))

		if !errors.Is(err, user.ErrUserNotFound) {
			t.Fatalf("err = %v, want user.ErrUserNotFound", err)
		}
	})

	t.Run("get by username returns domain not found", func(t *testing.T) {
		_, err := repo.GetByUsername(context.Background(), "missing")

		if !errors.Is(err, user.ErrUserNotFound) {
			t.Fatalf("err = %v, want user.ErrUserNotFound", err)
		}
	})

	t.Run("get token version returns domain not found", func(t *testing.T) {
		_, err := repo.GetTokenVersion(context.Background(), uuid.MustParse("018f0000-0000-7000-8000-000000000998"))

		if !errors.Is(err, user.ErrUserNotFound) {
			t.Fatalf("err = %v, want user.ErrUserNotFound", err)
		}
	})

	t.Run("get token version ignores soft deleted user", func(t *testing.T) {
		ctx := context.Background()
		userID := uuid.MustParse("018f0000-0000-7000-8000-000000000010")
		createSoftDeletedUser(t, repo, user.CreateUserInput{Nickname: "Deleted Token", UserID: userID, Username: "deleted-token", PasswordHash: "hash", Status: user.UserStatusNormal})

		_, err := repo.GetTokenVersion(ctx, userID)

		if !errors.Is(err, user.ErrUserNotFound) {
			t.Fatalf("err = %v, want user.ErrUserNotFound", err)
		}
	})

	t.Run("increment token version returns domain not found", func(t *testing.T) {
		_, err := repo.IncrementTokenVersion(context.Background(), uuid.MustParse("018f0000-0000-7000-8000-000000000997"))

		if !errors.Is(err, user.ErrUserNotFound) {
			t.Fatalf("err = %v, want user.ErrUserNotFound", err)
		}
	})

	t.Run("increment token version ignores soft deleted user", func(t *testing.T) {
		ctx := context.Background()
		userID := uuid.MustParse("018f0000-0000-7000-8000-000000000011")
		createSoftDeletedUser(t, repo, user.CreateUserInput{Nickname: "Deleted Increment", UserID: userID, Username: "deleted-increment", PasswordHash: "hash", Status: user.UserStatusNormal})

		_, err := repo.IncrementTokenVersion(ctx, userID)

		if !errors.Is(err, user.ErrUserNotFound) {
			t.Fatalf("err = %v, want user.ErrUserNotFound", err)
		}
	})

	t.Run("update credentials returns domain not found", func(t *testing.T) {
		_, err := repo.UpdateCredentials(context.Background(), auth.UpdateCredentialsInput{UserID: uuid.MustParse("018f0000-0000-7000-8000-000000000996"), PasswordHash: "new-hash", Status: user.UserStatusNormal})

		if !errors.Is(err, user.ErrUserNotFound) {
			t.Fatalf("err = %v, want user.ErrUserNotFound", err)
		}
	})

	t.Run("update credentials ignores soft deleted user", func(t *testing.T) {
		ctx := context.Background()
		userID := uuid.MustParse("018f0000-0000-7000-8000-000000000012")
		createSoftDeletedUser(t, repo, user.CreateUserInput{Nickname: "Deleted Credentials", UserID: userID, Username: "deleted-credentials", PasswordHash: "old-hash", Status: user.UserStatusMustChangePassword})

		_, err := repo.UpdateCredentials(ctx, auth.UpdateCredentialsInput{UserID: userID, PasswordHash: "new-hash", Status: user.UserStatusNormal})

		if !errors.Is(err, user.ErrUserNotFound) {
			t.Fatalf("err = %v, want user.ErrUserNotFound", err)
		}
		stored, err := repo.client.User.Query().Where(entuser.UserIDEQ(userID)).Only(ctx)
		if err != nil {
			t.Fatalf("query soft deleted user: %v", err)
		}
		if stored.PasswordHash != "old-hash" || stored.Status != int64(user.UserStatusMustChangePassword) || stored.TokenVersion != 1 {
			t.Fatalf("soft deleted user was updated: %#v", stored)
		}
	})

	t.Run("create uniqueness violation returns domain already exists", func(t *testing.T) {
		ctx := context.Background()
		_, err := repo.Create(ctx, user.CreateUserInput{Nickname: "Alice", UserID: uuid.MustParse("018f0000-0000-7000-8000-000000000001"), Username: "alice", PasswordHash: "hash", Status: user.UserStatusNormal})
		if err != nil {
			t.Fatalf("Create initial user: %v", err)
		}

		_, err = repo.Create(ctx, user.CreateUserInput{Nickname: "Alice 2", UserID: uuid.MustParse("018f0000-0000-7000-8000-000000000002"), Username: "alice", PasswordHash: "hash", Status: user.UserStatusNormal})

		if !errors.Is(err, user.ErrUserAlreadyExists) {
			t.Fatalf("err = %v, want user.ErrUserAlreadyExists", err)
		}
	})

	t.Run("create duplicate user id returns domain already exists", func(t *testing.T) {
		ctx := context.Background()
		userID := uuid.MustParse("018f0000-0000-7000-8000-000000000013")
		_, err := repo.Create(ctx, user.CreateUserInput{Nickname: "Duplicate ID", UserID: userID, Username: "duplicate-id", PasswordHash: "hash", Status: user.UserStatusNormal})
		if err != nil {
			t.Fatalf("Create initial user: %v", err)
		}

		_, err = repo.Create(ctx, user.CreateUserInput{Nickname: "Duplicate ID 2", UserID: userID, Username: "duplicate-id-2", PasswordHash: "hash", Status: user.UserStatusNormal})

		if !errors.Is(err, user.ErrUserAlreadyExists) {
			t.Fatalf("err = %v, want user.ErrUserAlreadyExists", err)
		}
	})

	t.Run("soft deleted username remains reserved", func(t *testing.T) {
		ctx := context.Background()
		createSoftDeletedUser(t, repo, user.CreateUserInput{Nickname: "Deleted Alice", UserID: uuid.MustParse("018f0000-0000-7000-8000-000000000003"), Username: "reserved-alice", PasswordHash: "hash", Status: user.UserStatusNormal})

		_, err := repo.Create(ctx, user.CreateUserInput{Nickname: "New Alice", UserID: uuid.MustParse("018f0000-0000-7000-8000-000000000004"), Username: "reserved-alice", PasswordHash: "hash", Status: user.UserStatusNormal})

		if !errors.Is(err, user.ErrUserAlreadyExists) {
			t.Fatalf("err = %v, want user.ErrUserAlreadyExists", err)
		}
	})
}

func TestUserRepositoryReturnsDomainUsers(t *testing.T) {
	repo := newTestUserStore(t)
	ctx := context.Background()
	userID := uuid.MustParse("018f0000-0000-7000-8000-000000000101")

	created, err := repo.Create(ctx, user.CreateUserInput{Nickname: "Domain Alice", UserID: userID, Username: "domain-alice", PasswordHash: "hash", Status: user.UserStatusMustChangePassword})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if created.ID == 0 || created.UserID != userID || created.Nickname != "Domain Alice" || created.Username != "domain-alice" || created.PasswordHash != "hash" || created.Status != user.UserStatusMustChangePassword || created.TokenVersion != 1 || created.CreatedAt == 0 || created.UpdatedAt == 0 {
		t.Fatalf("created = %#v", created)
	}

	byID, err := repo.GetByUserID(ctx, userID)
	if err != nil {
		t.Fatalf("GetByUserID: %v", err)
	}
	assertSameUser(t, byID, created)

	byUsername, err := repo.GetByUsername(ctx, "domain-alice")
	if err != nil {
		t.Fatalf("GetByUsername: %v", err)
	}
	assertSameCredential(t, byUsername, created)

	status := user.UserStatusMustChangePassword
	users, total, err := repo.ListUsers(ctx, user.ListUsersInput{Limit: 10, Username: "domain-alice", Status: &status})
	if err != nil {
		t.Fatalf("ListUsers: %v", err)
	}
	if total != 1 || len(users) != 1 || users[0].UserID != userID || users[0].Username != "domain-alice" || users[0].Status != user.UserStatusMustChangePassword {
		t.Fatalf("users=%#v total=%d", users, total)
	}
}

func TestUserRepositoryListUsersBoundaries(t *testing.T) {
	t.Run("empty page", func(t *testing.T) {
		repo := newTestUserStore(t)

		users, total, err := repo.ListUsers(context.Background(), user.ListUsersInput{Limit: 10})

		if err != nil {
			t.Fatalf("ListUsers: %v", err)
		}
		if total != 0 || len(users) != 0 {
			t.Fatalf("users=%#v total=%d, want empty page", users, total)
		}
	})

	t.Run("paginates with stable id order", func(t *testing.T) {
		repo := newTestUserStore(t)
		ctx := context.Background()
		first := createTestUser(t, repo, user.CreateUserInput{Nickname: "Page One", UserID: uuid.MustParse("018f0000-0000-7000-8000-000000000201"), Username: "page-one", PasswordHash: "hash-1", Status: user.UserStatusNormal})
		second := createTestUser(t, repo, user.CreateUserInput{Nickname: "Page Two", UserID: uuid.MustParse("018f0000-0000-7000-8000-000000000202"), Username: "page-two", PasswordHash: "hash-2", Status: user.UserStatusDisabled})
		third := createTestUser(t, repo, user.CreateUserInput{Nickname: "Page Three", UserID: uuid.MustParse("018f0000-0000-7000-8000-000000000203"), Username: "page-three", PasswordHash: "hash-3", Status: user.UserStatusMustChangePassword})

		users, total, err := repo.ListUsers(ctx, user.ListUsersInput{Limit: 2, Offset: 1})

		if err != nil {
			t.Fatalf("ListUsers: %v", err)
		}
		if total != 3 || len(users) != 2 {
			t.Fatalf("users=%#v total=%d, want 2 of 3", users, total)
		}
		if first.ID >= second.ID || second.ID >= third.ID {
			t.Fatalf("seed IDs not increasing: first=%d second=%d third=%d", first.ID, second.ID, third.ID)
		}
		assertSameUserValue(t, users[0], second)
		assertSameUserValue(t, users[1], third)
	})

	t.Run("filters by nickname username status and combined predicates", func(t *testing.T) {
		repo := newTestUserStore(t)
		ctx := context.Background()
		createTestUser(t, repo, user.CreateUserInput{Nickname: "Alice Ops", UserID: uuid.MustParse("018f0000-0000-7000-8000-000000000211"), Username: "alice-ops", PasswordHash: "hash-1", Status: user.UserStatusNormal})
		createTestUser(t, repo, user.CreateUserInput{Nickname: "Alice Audit", UserID: uuid.MustParse("018f0000-0000-7000-8000-000000000212"), Username: "alice-audit", PasswordHash: "hash-2", Status: user.UserStatusDisabled})
		createTestUser(t, repo, user.CreateUserInput{Nickname: "Bob Ops", UserID: uuid.MustParse("018f0000-0000-7000-8000-000000000213"), Username: "bob-ops", PasswordHash: "hash-3", Status: user.UserStatusMustChangePassword})

		assertListUsernames(t, repo, user.ListUsersInput{Limit: 10, Nickname: "Alice"}, []string{"alice-ops", "alice-audit"})
		assertListUsernames(t, repo, user.ListUsersInput{Limit: 10, Username: "bob-ops"}, []string{"bob-ops"})

		disabled := user.UserStatusDisabled
		assertListUsernames(t, repo, user.ListUsersInput{Limit: 10, Status: &disabled}, []string{"alice-audit"})
		assertListUsernames(t, repo, user.ListUsersInput{Limit: 10, Nickname: "Alice", Username: "alice-audit", Status: &disabled}, []string{"alice-audit"})

		users, total, err := repo.ListUsers(ctx, user.ListUsersInput{Limit: 10, Nickname: "Missing"})
		if err != nil {
			t.Fatalf("ListUsers missing filter: %v", err)
		}
		if total != 0 || len(users) != 0 {
			t.Fatalf("users=%#v total=%d, want no matches", users, total)
		}
	})

	t.Run("excludes soft deleted users from rows and total", func(t *testing.T) {
		repo := newTestUserStore(t)
		ctx := context.Background()
		active := createTestUser(t, repo, user.CreateUserInput{Nickname: "Active Alice", UserID: uuid.MustParse("018f0000-0000-7000-8000-000000000221"), Username: "active-alice", PasswordHash: "hash", Status: user.UserStatusNormal})
		createSoftDeletedUser(t, repo, user.CreateUserInput{Nickname: "Deleted Alice", UserID: uuid.MustParse("018f0000-0000-7000-8000-000000000222"), Username: "deleted-alice-list", PasswordHash: "hash", Status: user.UserStatusNormal})

		users, total, err := repo.ListUsers(ctx, user.ListUsersInput{Limit: 10, Nickname: "Alice"})

		if err != nil {
			t.Fatalf("ListUsers: %v", err)
		}
		if total != 1 || len(users) != 1 {
			t.Fatalf("users=%#v total=%d, want only active user", users, total)
		}
		assertSameUserValue(t, users[0], active)
	})
}

func TestUserRepositoryTokenVersionPersistence(t *testing.T) {
	t.Run("get token version returns initial version consistently", func(t *testing.T) {
		repo := newTestUserStore(t)
		ctx := context.Background()
		userID := uuid.MustParse("018f0000-0000-7000-8000-000000000301")
		createTestUser(t, repo, user.CreateUserInput{Nickname: "Version Alice", UserID: userID, Username: "version-alice", PasswordHash: "hash", Status: user.UserStatusNormal})

		first, err := repo.GetTokenVersion(ctx, userID)
		if err != nil {
			t.Fatalf("GetTokenVersion first: %v", err)
		}
		second, err := repo.GetTokenVersion(ctx, userID)
		if err != nil {
			t.Fatalf("GetTokenVersion second: %v", err)
		}
		if first != 1 || second != 1 {
			t.Fatalf("versions = %d, %d; want stable initial 1", first, second)
		}
	})

	t.Run("increment token version persists and returns new version", func(t *testing.T) {
		repo := newTestUserStore(t)
		ctx := context.Background()
		userID := uuid.MustParse("018f0000-0000-7000-8000-000000000302")
		createTestUser(t, repo, user.CreateUserInput{Nickname: "Increment Alice", UserID: userID, Username: "increment-alice", PasswordHash: "hash", Status: user.UserStatusNormal})

		version, err := repo.IncrementTokenVersion(ctx, userID)
		if err != nil {
			t.Fatalf("IncrementTokenVersion first: %v", err)
		}
		if version != 2 {
			t.Fatalf("first increment version = %d, want 2", version)
		}

		stored, err := repo.GetTokenVersion(ctx, userID)
		if err != nil {
			t.Fatalf("GetTokenVersion after increment: %v", err)
		}
		if stored != 2 {
			t.Fatalf("stored version = %d, want 2", stored)
		}

		version, err = repo.IncrementTokenVersion(ctx, userID)
		if err != nil {
			t.Fatalf("IncrementTokenVersion second: %v", err)
		}
		if version != 3 {
			t.Fatalf("second increment version = %d, want 3", version)
		}
	})
}

func TestUserListPredicates(t *testing.T) {
	if got := buildListPredicates(user.ListUsersInput{}); len(got) != 1 {
		t.Fatalf("predicates = %d, want 1", len(got))
	}

	status := user.UserStatusNormal
	got := buildListPredicates(user.ListUsersInput{Nickname: "Ali", Username: "alice", Status: &status})
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

func createTestUser(t *testing.T, repo *userStore, input user.CreateUserInput) *user.User {
	t.Helper()
	created, err := repo.Create(context.Background(), input)
	if err != nil {
		t.Fatalf("Create user %q: %v", input.Username, err)
	}
	return created
}

func createSoftDeletedUser(t *testing.T, repo *userStore, input user.CreateUserInput) {
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

func assertSameUser(t *testing.T, got, want *user.User) {
	t.Helper()
	if got == nil || want == nil {
		t.Fatalf("got=%#v want=%#v", got, want)
	}
	assertSameUserValue(t, *got, want)
}

func assertSameUserValue(t *testing.T, got user.User, want *user.User) {
	t.Helper()
	if want == nil {
		t.Fatal("want user is nil")
	}
	if got.ID != want.ID || got.UserID != want.UserID || got.Nickname != want.Nickname || got.Username != want.Username || got.PasswordHash != want.PasswordHash || got.Status != want.Status || got.TokenVersion != want.TokenVersion || got.CreatedAt != want.CreatedAt || got.UpdatedAt != want.UpdatedAt {
		t.Fatalf("got user = %#v, want %#v", got, want)
	}
}

func assertSameCredential(t *testing.T, got *auth.UserCredential, want *user.User) {
	t.Helper()
	if got == nil || want == nil {
		t.Fatalf("got=%#v want=%#v", got, want)
	}
	if got.UserID != want.UserID || got.Username != want.Username || got.PasswordHash != want.PasswordHash || got.Status != want.Status || got.TokenVersion != want.TokenVersion {
		t.Fatalf("got credential = %#v, want user fields from %#v", got, want)
	}
}

func assertListUsernames(t *testing.T, repo *userStore, input user.ListUsersInput, want []string) {
	t.Helper()
	users, total, err := repo.ListUsers(context.Background(), input)
	if err != nil {
		t.Fatalf("ListUsers(%#v): %v", input, err)
	}
	if total != len(want) || len(users) != len(want) {
		t.Fatalf("users=%#v total=%d, want usernames %v", users, total, want)
	}
	for i := range want {
		if users[i].Username != want[i] {
			t.Fatalf("users[%d].Username = %q, want %q; users=%#v", i, users[i].Username, want[i], users)
		}
	}
}
