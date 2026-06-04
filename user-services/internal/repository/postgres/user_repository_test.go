package postgres

import (
	"context"
	"errors"
	"testing"

	"github.com/aegiscore/user-services/ent/enttest"
	"github.com/aegiscore/user-services/ent/user"
	"github.com/aegiscore/user-services/internal/domain"
	"github.com/aegiscore/user-services/internal/repository"
	"github.com/google/uuid"
	_ "github.com/mattn/go-sqlite3"
)

const deletedAtForTest int64 = 1710000000000

func TestUserRepositoryDomainErrors(t *testing.T) {
	repo := newTestUserRepository(t)

	t.Run("get by user id returns domain not found", func(t *testing.T) {
		_, err := repo.GetByUserID(context.Background(), uuid.MustParse("018f0000-0000-7000-8000-000000000999"))

		if !errors.Is(err, domain.ErrUserNotFound) {
			t.Fatalf("err = %v, want ErrUserNotFound", err)
		}
	})

	t.Run("get by username returns domain not found", func(t *testing.T) {
		_, err := repo.GetByUsername(context.Background(), "missing")

		if !errors.Is(err, domain.ErrUserNotFound) {
			t.Fatalf("err = %v, want ErrUserNotFound", err)
		}
	})

	t.Run("get token version returns domain not found", func(t *testing.T) {
		_, err := repo.GetTokenVersion(context.Background(), uuid.MustParse("018f0000-0000-7000-8000-000000000998"))

		if !errors.Is(err, domain.ErrUserNotFound) {
			t.Fatalf("err = %v, want ErrUserNotFound", err)
		}
	})

	t.Run("get token version ignores soft deleted user", func(t *testing.T) {
		ctx := context.Background()
		userID := uuid.MustParse("018f0000-0000-7000-8000-000000000010")
		createSoftDeletedUser(t, repo, repository.CreateUserInput{Nickname: "Deleted Token", UserID: userID, Username: "deleted-token", PasswordHash: "hash", Status: domain.UserStatusNormal})

		_, err := repo.GetTokenVersion(ctx, userID)

		if !errors.Is(err, domain.ErrUserNotFound) {
			t.Fatalf("err = %v, want ErrUserNotFound", err)
		}
	})

	t.Run("increment token version returns domain not found", func(t *testing.T) {
		_, err := repo.IncrementTokenVersion(context.Background(), uuid.MustParse("018f0000-0000-7000-8000-000000000997"))

		if !errors.Is(err, domain.ErrUserNotFound) {
			t.Fatalf("err = %v, want ErrUserNotFound", err)
		}
	})

	t.Run("increment token version ignores soft deleted user", func(t *testing.T) {
		ctx := context.Background()
		userID := uuid.MustParse("018f0000-0000-7000-8000-000000000011")
		createSoftDeletedUser(t, repo, repository.CreateUserInput{Nickname: "Deleted Increment", UserID: userID, Username: "deleted-increment", PasswordHash: "hash", Status: domain.UserStatusNormal})

		_, err := repo.IncrementTokenVersion(ctx, userID)

		if !errors.Is(err, domain.ErrUserNotFound) {
			t.Fatalf("err = %v, want ErrUserNotFound", err)
		}
	})

	t.Run("update credentials returns domain not found", func(t *testing.T) {
		_, err := repo.UpdateCredentials(context.Background(), repository.UpdateCredentialsInput{UserID: uuid.MustParse("018f0000-0000-7000-8000-000000000996"), PasswordHash: "new-hash", Status: domain.UserStatusNormal})

		if !errors.Is(err, domain.ErrUserNotFound) {
			t.Fatalf("err = %v, want ErrUserNotFound", err)
		}
	})

	t.Run("update credentials ignores soft deleted user", func(t *testing.T) {
		ctx := context.Background()
		userID := uuid.MustParse("018f0000-0000-7000-8000-000000000012")
		createSoftDeletedUser(t, repo, repository.CreateUserInput{Nickname: "Deleted Credentials", UserID: userID, Username: "deleted-credentials", PasswordHash: "old-hash", Status: domain.UserStatusMustChangePassword})

		_, err := repo.UpdateCredentials(ctx, repository.UpdateCredentialsInput{UserID: userID, PasswordHash: "new-hash", Status: domain.UserStatusNormal})

		if !errors.Is(err, domain.ErrUserNotFound) {
			t.Fatalf("err = %v, want ErrUserNotFound", err)
		}
		postgresRepo, ok := repo.(*userRepository)
		if !ok {
			t.Fatalf("repo = %T, want *userRepository", repo)
		}
		stored, err := postgresRepo.client.User.Query().Where(user.UserIDEQ(userID)).Only(ctx)
		if err != nil {
			t.Fatalf("query soft deleted user: %v", err)
		}
		if stored.PasswordHash != "old-hash" || stored.Status != int64(domain.UserStatusMustChangePassword) || stored.TokenVersion != 1 {
			t.Fatalf("soft deleted user was updated: %#v", stored)
		}
	})

	t.Run("create uniqueness violation returns domain already exists", func(t *testing.T) {
		ctx := context.Background()
		_, err := repo.Create(ctx, repository.CreateUserInput{Nickname: "Alice", UserID: uuid.MustParse("018f0000-0000-7000-8000-000000000001"), Username: "alice", PasswordHash: "hash", Status: domain.UserStatusNormal})
		if err != nil {
			t.Fatalf("Create initial user: %v", err)
		}

		_, err = repo.Create(ctx, repository.CreateUserInput{Nickname: "Alice 2", UserID: uuid.MustParse("018f0000-0000-7000-8000-000000000002"), Username: "alice", PasswordHash: "hash", Status: domain.UserStatusNormal})

		if !errors.Is(err, domain.ErrUserAlreadyExists) {
			t.Fatalf("err = %v, want ErrUserAlreadyExists", err)
		}
	})
}

func TestUserRepositoryReturnsDomainUsers(t *testing.T) {
	repo := newTestUserRepository(t)
	ctx := context.Background()
	userID := uuid.MustParse("018f0000-0000-7000-8000-000000000101")

	created, err := repo.Create(ctx, repository.CreateUserInput{Nickname: "Domain Alice", UserID: userID, Username: "domain-alice", PasswordHash: "hash", Status: domain.UserStatusMustChangePassword})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if created.ID == 0 || created.UserID != userID || created.Nickname != "Domain Alice" || created.Username != "domain-alice" || created.PasswordHash != "hash" || created.Status != domain.UserStatusMustChangePassword || created.TokenVersion != 1 || created.CreatedAt == 0 || created.UpdatedAt == 0 {
		t.Fatalf("created = %#v", created)
	}

	byID, err := repo.GetByUserID(ctx, userID)
	if err != nil {
		t.Fatalf("GetByUserID: %v", err)
	}
	if byID.UserID != userID || byID.Username != "domain-alice" || byID.Status != domain.UserStatusMustChangePassword {
		t.Fatalf("byID = %#v", byID)
	}

	byUsername, err := repo.GetByUsername(ctx, "domain-alice")
	if err != nil {
		t.Fatalf("GetByUsername: %v", err)
	}
	if byUsername.UserID != userID || byUsername.PasswordHash != "hash" || byUsername.TokenVersion != 1 {
		t.Fatalf("byUsername = %#v", byUsername)
	}

	status := domain.UserStatusMustChangePassword
	users, total, err := repo.ListUsers(ctx, repository.ListUsersInput{Limit: 10, Username: "domain-alice", Status: &status})
	if err != nil {
		t.Fatalf("ListUsers: %v", err)
	}
	if total != 1 || len(users) != 1 || users[0].UserID != userID || users[0].Username != "domain-alice" || users[0].Status != domain.UserStatusMustChangePassword {
		t.Fatalf("users=%#v total=%d", users, total)
	}
}

func TestUserListPredicates(t *testing.T) {
	if got := userListPredicates(repository.ListUsersInput{}); len(got) != 1 {
		t.Fatalf("predicates = %d, want 1", len(got))
	}

	status := domain.UserStatusNormal
	got := userListPredicates(repository.ListUsersInput{Nickname: "Ali", Username: "alice", Status: &status})
	if len(got) != 4 {
		t.Fatalf("predicates = %d, want 4", len(got))
	}
}

func newTestUserRepository(t *testing.T) repository.UserRepository {
	t.Helper()
	client := enttest.Open(t, "sqlite3", "file:user_repository_test?mode=memory&cache=shared&_fk=1")
	t.Cleanup(func() { _ = client.Close() })
	return NewUserRepository(UserRepositoryParams{Client: client})
}

func createSoftDeletedUser(t *testing.T, repo repository.UserRepository, input repository.CreateUserInput) {
	t.Helper()
	ctx := context.Background()
	created, err := repo.Create(ctx, input)
	if err != nil {
		t.Fatalf("Create soft deleted user: %v", err)
	}
	postgresRepo, ok := repo.(*userRepository)
	if !ok {
		t.Fatalf("repo = %T, want *userRepository", repo)
	}
	if _, err := postgresRepo.client.User.UpdateOneID(created.ID).SetDeletedAt(deletedAtForTest).Save(ctx); err != nil {
		t.Fatalf("soft delete user: %v", err)
	}
}
