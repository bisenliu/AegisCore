package postgres

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"github.com/aegiscore/user-services/ent"
	"github.com/aegiscore/user-services/ent/enttest"
	entuser "github.com/aegiscore/user-services/ent/user"
	authdomain "github.com/aegiscore/user-services/internal/features/auth/domain"
	userdomain "github.com/aegiscore/user-services/internal/features/user/domain"
	"github.com/google/uuid"
	_ "github.com/mattn/go-sqlite3"
)

const deletedAtForCredentialTest int64 = 1710000000000

type credentialTestUserInput struct {
	Nickname     string
	UserID       uuid.UUID
	Username     string
	PasswordHash string
	Status       userdomain.UserStatus
}

func TestCredentialStoreDomainErrors(t *testing.T) {
	repo := newTestCredentialStore(t)

	t.Run("get by username returns domain not found", func(t *testing.T) {
		_, err := repo.GetByUsername(context.Background(), "missing")

		if !errors.Is(err, userdomain.ErrUserNotFound) {
			t.Fatalf("err = %v, want userdomain.ErrUserNotFound", err)
		}
	})

	t.Run("get by username ignores soft deleted user", func(t *testing.T) {
		ctx := context.Background()
		userID := uuid.MustParse("018f0000-0000-7000-8000-000000000410")
		createSoftDeletedCredentialUser(t, repo, credentialTestUserInput{Nickname: "Deleted Username", UserID: userID, Username: "deleted-username", PasswordHash: "hash", Status: userdomain.UserStatusNormal})

		_, err := repo.GetByUsername(ctx, "deleted-username")

		if !errors.Is(err, userdomain.ErrUserNotFound) {
			t.Fatalf("err = %v, want userdomain.ErrUserNotFound", err)
		}
	})

	t.Run("get credential by user id returns domain not found", func(t *testing.T) {
		_, err := repo.GetCredentialByUserID(context.Background(), uuid.MustParse("018f0000-0000-7000-8000-000000000411"))

		if !errors.Is(err, userdomain.ErrUserNotFound) {
			t.Fatalf("err = %v, want userdomain.ErrUserNotFound", err)
		}
	})

	t.Run("get credential by user id ignores soft deleted user", func(t *testing.T) {
		ctx := context.Background()
		userID := uuid.MustParse("018f0000-0000-7000-8000-000000000412")
		createSoftDeletedCredentialUser(t, repo, credentialTestUserInput{Nickname: "Deleted Credential", UserID: userID, Username: "deleted-credential", PasswordHash: "hash", Status: userdomain.UserStatusNormal})

		_, err := repo.GetCredentialByUserID(ctx, userID)

		if !errors.Is(err, userdomain.ErrUserNotFound) {
			t.Fatalf("err = %v, want userdomain.ErrUserNotFound", err)
		}
	})

	t.Run("get token version returns domain not found", func(t *testing.T) {
		_, err := repo.GetTokenVersion(context.Background(), uuid.MustParse("018f0000-0000-7000-8000-000000000413"))

		if !errors.Is(err, userdomain.ErrUserNotFound) {
			t.Fatalf("err = %v, want userdomain.ErrUserNotFound", err)
		}
	})

	t.Run("get token version ignores soft deleted user", func(t *testing.T) {
		ctx := context.Background()
		userID := uuid.MustParse("018f0000-0000-7000-8000-000000000414")
		createSoftDeletedCredentialUser(t, repo, credentialTestUserInput{Nickname: "Deleted Token", UserID: userID, Username: "deleted-token", PasswordHash: "hash", Status: userdomain.UserStatusNormal})

		_, err := repo.GetTokenVersion(ctx, userID)

		if !errors.Is(err, userdomain.ErrUserNotFound) {
			t.Fatalf("err = %v, want userdomain.ErrUserNotFound", err)
		}
	})

	t.Run("increment token version returns domain not found", func(t *testing.T) {
		_, err := repo.IncrementTokenVersion(context.Background(), uuid.MustParse("018f0000-0000-7000-8000-000000000415"))

		if !errors.Is(err, userdomain.ErrUserNotFound) {
			t.Fatalf("err = %v, want userdomain.ErrUserNotFound", err)
		}
	})

	t.Run("increment token version ignores soft deleted user", func(t *testing.T) {
		ctx := context.Background()
		userID := uuid.MustParse("018f0000-0000-7000-8000-000000000416")
		createSoftDeletedCredentialUser(t, repo, credentialTestUserInput{Nickname: "Deleted Increment", UserID: userID, Username: "deleted-increment", PasswordHash: "hash", Status: userdomain.UserStatusNormal})

		_, err := repo.IncrementTokenVersion(ctx, userID)

		if !errors.Is(err, userdomain.ErrUserNotFound) {
			t.Fatalf("err = %v, want userdomain.ErrUserNotFound", err)
		}
	})

	t.Run("update credentials returns domain not found", func(t *testing.T) {
		_, err := repo.UpdateCredentials(context.Background(), authdomain.UpdateCredentialsInput{UserID: uuid.MustParse("018f0000-0000-7000-8000-000000000417"), PasswordHash: "new-hash", Status: userdomain.UserStatusNormal})

		if !errors.Is(err, userdomain.ErrUserNotFound) {
			t.Fatalf("err = %v, want userdomain.ErrUserNotFound", err)
		}
	})

	t.Run("update credentials ignores soft deleted user", func(t *testing.T) {
		ctx := context.Background()
		userID := uuid.MustParse("018f0000-0000-7000-8000-000000000418")
		createSoftDeletedCredentialUser(t, repo, credentialTestUserInput{Nickname: "Deleted Credentials", UserID: userID, Username: "deleted-credentials", PasswordHash: "old-hash", Status: userdomain.UserStatusMustChangePassword})

		_, err := repo.UpdateCredentials(ctx, authdomain.UpdateCredentialsInput{UserID: userID, PasswordHash: "new-hash", Status: userdomain.UserStatusNormal})

		if !errors.Is(err, userdomain.ErrUserNotFound) {
			t.Fatalf("err = %v, want userdomain.ErrUserNotFound", err)
		}
		stored, err := repo.client.User.Query().Where(entuser.UserIDEQ(userID)).Only(ctx)
		if err != nil {
			t.Fatalf("query soft deleted user: %v", err)
		}
		if stored.PasswordHash != "old-hash" || stored.Status != int64(userdomain.UserStatusMustChangePassword) || stored.TokenVersion != 1 {
			t.Fatalf("soft deleted user was updated: %#v", stored)
		}
	})
}

func TestCredentialStoreReturnsCredentials(t *testing.T) {
	repo := newTestCredentialStore(t)
	ctx := context.Background()
	userID := uuid.MustParse("018f0000-0000-7000-8000-000000000501")
	user := createCredentialTestUser(t, repo, credentialTestUserInput{
		Nickname:     "Credential Alice",
		UserID:       userID,
		Username:     "credential-alice",
		PasswordHash: "hash",
		Status:       userdomain.UserStatusMustChangePassword,
	})

	byUsername, err := repo.GetByUsername(ctx, "credential-alice")
	if err != nil {
		t.Fatalf("GetByUsername: %v", err)
	}
	assertSameCredential(t, byUsername, user)

	byUserID, err := repo.GetCredentialByUserID(ctx, userID)
	if err != nil {
		t.Fatalf("GetCredentialByUserID: %v", err)
	}
	assertSameCredential(t, byUserID, user)
}

func TestCredentialStoreTokenVersionPersistence(t *testing.T) {
	t.Run("get token version returns initial version consistently", func(t *testing.T) {
		repo := newTestCredentialStore(t)
		ctx := context.Background()
		userID := uuid.MustParse("018f0000-0000-7000-8000-000000000601")
		createCredentialTestUser(t, repo, credentialTestUserInput{Nickname: "Version Alice", UserID: userID, Username: "version-alice", PasswordHash: "hash", Status: userdomain.UserStatusNormal})

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
		repo := newTestCredentialStore(t)
		ctx := context.Background()
		userID := uuid.MustParse("018f0000-0000-7000-8000-000000000602")
		createCredentialTestUser(t, repo, credentialTestUserInput{Nickname: "Increment Alice", UserID: userID, Username: "increment-alice", PasswordHash: "hash", Status: userdomain.UserStatusNormal})

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

func TestCredentialStoreUpdateCredentials(t *testing.T) {
	repo := newTestCredentialStore(t)
	ctx := context.Background()
	userID := uuid.MustParse("018f0000-0000-7000-8000-000000000701")
	createCredentialTestUser(t, repo, credentialTestUserInput{Nickname: "Update Alice", UserID: userID, Username: "update-alice", PasswordHash: "old-hash", Status: userdomain.UserStatusMustChangePassword})

	version, err := repo.UpdateCredentials(ctx, authdomain.UpdateCredentialsInput{UserID: userID, PasswordHash: "new-hash", Status: userdomain.UserStatusNormal})
	if err != nil {
		t.Fatalf("UpdateCredentials: %v", err)
	}
	if version != 2 {
		t.Fatalf("version = %d, want 2", version)
	}
	stored, err := repo.client.User.Query().Where(entuser.UserIDEQ(userID)).Only(ctx)
	if err != nil {
		t.Fatalf("query updated user: %v", err)
	}
	if stored.PasswordHash != "new-hash" || stored.Status != int64(userdomain.UserStatusNormal) || stored.TokenVersion != 2 {
		t.Fatalf("stored user = %#v, want updated credentials", stored)
	}
}

func newTestCredentialStore(t *testing.T) *credentialStore {
	t.Helper()
	client := enttest.Open(t, "sqlite3", fmt.Sprintf("file:credential_store_test_%s?mode=memory&cache=shared&_fk=1", uuid.NewString()))
	t.Cleanup(func() { _ = client.Close() })
	return NewCredentialStore(CredentialStoreParams{Client: client})
}

func createCredentialTestUser(t *testing.T, repo *credentialStore, input credentialTestUserInput) *ent.User {
	t.Helper()
	created, err := repo.client.User.Create().
		SetNickname(input.Nickname).
		SetUserID(input.UserID).
		SetUsername(input.Username).
		SetPasswordHash(input.PasswordHash).
		SetStatus(int64(input.Status)).
		Save(context.Background())
	if err != nil {
		t.Fatalf("Create user %q: %v", input.Username, err)
	}
	return created
}

func createSoftDeletedCredentialUser(t *testing.T, repo *credentialStore, input credentialTestUserInput) {
	t.Helper()
	ctx := context.Background()
	created := createCredentialTestUser(t, repo, input)
	if _, err := repo.client.User.UpdateOneID(created.ID).SetDeletedAt(deletedAtForCredentialTest).Save(ctx); err != nil {
		t.Fatalf("soft delete user: %v", err)
	}
}

func assertSameCredential(t *testing.T, got *authdomain.UserCredential, want *ent.User) {
	t.Helper()
	if got == nil || want == nil {
		t.Fatalf("got=%#v want=%#v", got, want)
	}
	if got.UserID != want.UserID || got.Username != want.Username || got.PasswordHash != want.PasswordHash || got.Status != userdomain.UserStatus(want.Status) || got.TokenVersion != want.TokenVersion {
		t.Fatalf("got credential = %#v, want user fields from %#v", got, want)
	}
}
