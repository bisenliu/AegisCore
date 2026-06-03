package postgres

import (
	"context"
	"errors"
	"testing"

	"github.com/aegiscore/user-services/ent/enttest"
	"github.com/aegiscore/user-services/internal/domain"
	"github.com/aegiscore/user-services/internal/repository"
	"github.com/google/uuid"
	_ "github.com/mattn/go-sqlite3"
)

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
