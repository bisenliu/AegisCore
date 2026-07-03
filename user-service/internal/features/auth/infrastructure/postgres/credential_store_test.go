package postgres

import (
	"context"
	"fmt"
	"testing"

	"github.com/google/uuid"
	_ "github.com/mattn/go-sqlite3"
	"github.com/stretchr/testify/require"

	runtimeid "github.com/aegiscore/common/runtime/id"
	"github.com/aegiscore/user-service/ent"
	"github.com/aegiscore/user-service/ent/enttest"
	entuser "github.com/aegiscore/user-service/ent/user"
	authdomain "github.com/aegiscore/user-service/internal/features/auth/domain"
	"github.com/aegiscore/user-service/internal/shared/identity"
)

const deletedAtForCredentialTest int64 = 1710000000000

type credentialTestUserInput struct {
	Nickname     string
	UserID       uuid.UUID
	Username     string
	PasswordHash string
	Status       identity.UserStatus
}

func TestCredentialStoreDomainErrors(t *testing.T) {
	repo := newTestCredentialStore(t)

	t.Run("get by username returns domain not found", func(t *testing.T) {
		_, err := repo.GetByUsername(context.Background(), "missing")
		require.ErrorIs(t, err, identity.ErrUserNotFound,
			"err = %v, want identity.ErrUserNotFound", err)

	})

	t.Run("get by username ignores soft deleted user", func(t *testing.T) {
		ctx := context.Background()
		userID := uuid.MustParse("018f0000-0000-7000-8000-000000000410")
		createSoftDeletedCredentialUser(t, repo, credentialTestUserInput{Nickname: "Deleted Username", UserID: userID, Username: "deleted-username", PasswordHash: "hash", Status: identity.UserStatusNormal})

		_, err := repo.GetByUsername(ctx, "deleted-username")
		require.ErrorIs(t, err, identity.ErrUserNotFound,
			"err = %v, want identity.ErrUserNotFound", err)

	})

	t.Run("get credential by user id returns domain not found", func(t *testing.T) {
		_, err := repo.GetCredentialByUserID(context.Background(), uuid.MustParse("018f0000-0000-7000-8000-000000000411"))
		require.ErrorIs(t, err, identity.ErrUserNotFound,
			"err = %v, want identity.ErrUserNotFound", err)

	})

	t.Run("get credential by user id ignores soft deleted user", func(t *testing.T) {
		ctx := context.Background()
		userID := uuid.MustParse("018f0000-0000-7000-8000-000000000412")
		createSoftDeletedCredentialUser(t, repo, credentialTestUserInput{Nickname: "Deleted Credential", UserID: userID, Username: "deleted-credential", PasswordHash: "hash", Status: identity.UserStatusNormal})

		_, err := repo.GetCredentialByUserID(ctx, userID)
		require.ErrorIs(t, err, identity.ErrUserNotFound,
			"err = %v, want identity.ErrUserNotFound", err)

	})

	t.Run("get token version returns domain not found", func(t *testing.T) {
		_, err := repo.GetTokenVersion(context.Background(), uuid.MustParse("018f0000-0000-7000-8000-000000000413"))
		require.ErrorIs(t, err, identity.ErrUserNotFound,
			"err = %v, want identity.ErrUserNotFound", err)

	})

	t.Run("get token version ignores soft deleted user", func(t *testing.T) {
		ctx := context.Background()
		userID := uuid.MustParse("018f0000-0000-7000-8000-000000000414")
		createSoftDeletedCredentialUser(t, repo, credentialTestUserInput{Nickname: "Deleted Token", UserID: userID, Username: "deleted-token", PasswordHash: "hash", Status: identity.UserStatusNormal})

		_, err := repo.GetTokenVersion(ctx, userID)
		require.ErrorIs(t, err, identity.ErrUserNotFound,
			"err = %v, want identity.ErrUserNotFound", err)

	})

	t.Run("increment token version returns domain not found", func(t *testing.T) {
		_, err := repo.IncrementTokenVersion(context.Background(), uuid.MustParse("018f0000-0000-7000-8000-000000000415"))
		require.ErrorIs(t, err, identity.ErrUserNotFound,
			"err = %v, want identity.ErrUserNotFound", err)

	})

	t.Run("increment token version ignores soft deleted user", func(t *testing.T) {
		ctx := context.Background()
		userID := uuid.MustParse("018f0000-0000-7000-8000-000000000416")
		createSoftDeletedCredentialUser(t, repo, credentialTestUserInput{Nickname: "Deleted Increment", UserID: userID, Username: "deleted-increment", PasswordHash: "hash", Status: identity.UserStatusNormal})

		_, err := repo.IncrementTokenVersion(ctx, userID)
		require.ErrorIs(t, err, identity.ErrUserNotFound,
			"err = %v, want identity.ErrUserNotFound", err)

	})

	t.Run("update credentials returns domain not found", func(t *testing.T) {
		_, err := repo.UpdateCredentials(context.Background(), authdomain.UpdateCredentialsInput{UserID: uuid.MustParse("018f0000-0000-7000-8000-000000000417"), PasswordHash: "new-hash", Status: identity.UserStatusNormal})
		require.ErrorIs(t, err, identity.ErrUserNotFound,
			"err = %v, want identity.ErrUserNotFound", err)

	})

	t.Run("update credentials ignores soft deleted user", func(t *testing.T) {
		ctx := context.Background()
		userID := uuid.MustParse("018f0000-0000-7000-8000-000000000418")
		createSoftDeletedCredentialUser(t, repo, credentialTestUserInput{Nickname: "Deleted Credentials", UserID: userID, Username: "deleted-credentials", PasswordHash: "old-hash", Status: identity.UserStatusMustChangePassword})

		_, err := repo.UpdateCredentials(ctx, authdomain.UpdateCredentialsInput{UserID: userID, PasswordHash: "new-hash", Status: identity.UserStatusNormal})
		require.ErrorIs(t, err, identity.ErrUserNotFound,
			"err = %v, want identity.ErrUserNotFound", err)

		stored, err := repo.client.User.Query().Where(entuser.UserIDEQ(userID)).Only(ctx)
		require.NoError(t, err,
			"query soft deleted user: %v", err)
		require.False(t, stored.PasswordHash != "old-hash" || stored.Status != int64(identity.UserStatusMustChangePassword) || stored.TokenVersion != 1,
			"soft deleted user was updated: %#v", stored)

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
		Status:       identity.UserStatusMustChangePassword,
	})

	byUsername, err := repo.GetByUsername(ctx, "credential-alice")
	require.NoError(t, err,
		"GetByUsername: %v", err)

	assertSameCredential(t, byUsername, user)

	byUserID, err := repo.GetCredentialByUserID(ctx, userID)
	require.NoError(t, err,
		"GetCredentialByUserID: %v", err)

	assertSameCredential(t, byUserID, user)
}

func TestCredentialStoreTokenVersionPersistence(t *testing.T) {
	t.Run("get token version returns initial version consistently", func(t *testing.T) {
		repo := newTestCredentialStore(t)
		ctx := context.Background()
		userID := uuid.MustParse("018f0000-0000-7000-8000-000000000601")
		createCredentialTestUser(t, repo, credentialTestUserInput{Nickname: "Version Alice", UserID: userID, Username: "version-alice", PasswordHash: "hash", Status: identity.UserStatusNormal})

		first, err := repo.GetTokenVersion(ctx, userID)
		require.NoError(t, err,
			"GetTokenVersion first: %v", err)

		second, err := repo.GetTokenVersion(ctx, userID)
		require.NoError(t, err,
			"GetTokenVersion second: %v", err)
		require.False(t, first != 1 || second != 1,
			"versions = %d, %d; want stable initial 1", first, second)

	})

	t.Run("increment token version persists and returns new version", func(t *testing.T) {
		repo := newTestCredentialStore(t)
		ctx := context.Background()
		userID := uuid.MustParse("018f0000-0000-7000-8000-000000000602")
		createCredentialTestUser(t, repo, credentialTestUserInput{Nickname: "Increment Alice", UserID: userID, Username: "increment-alice", PasswordHash: "hash", Status: identity.UserStatusNormal})

		version, err := repo.IncrementTokenVersion(ctx, userID)
		require.NoError(t, err,
			"IncrementTokenVersion first: %v", err)
		require.EqualValues(t, 2, version,
			"first increment version = %d, want 2", version)

		stored, err := repo.GetTokenVersion(ctx, userID)
		require.NoError(t, err,
			"GetTokenVersion after increment: %v", err)
		require.EqualValues(t, 2, stored,
			"stored version = %d, want 2", stored)

		version, err = repo.IncrementTokenVersion(ctx, userID)
		require.NoError(t, err,
			"IncrementTokenVersion second: %v", err)
		require.EqualValues(t, 3, version,
			"second increment version = %d, want 3", version)

	})
}

func TestCredentialStoreUpdateCredentials(t *testing.T) {
	repo := newTestCredentialStore(t)
	ctx := context.Background()
	userID := uuid.MustParse("018f0000-0000-7000-8000-000000000701")
	createCredentialTestUser(t, repo, credentialTestUserInput{Nickname: "Update Alice", UserID: userID, Username: "update-alice", PasswordHash: "old-hash", Status: identity.UserStatusMustChangePassword})

	version, err := repo.UpdateCredentials(ctx, authdomain.UpdateCredentialsInput{UserID: userID, PasswordHash: "new-hash", Status: identity.UserStatusNormal})
	require.NoError(t, err,
		"UpdateCredentials: %v", err)
	require.EqualValues(t, 2, version,
		"version = %d, want 2", version)

	stored, err := repo.client.User.Query().Where(entuser.UserIDEQ(userID)).Only(ctx)
	require.NoError(t, err,
		"query updated user: %v", err)
	require.False(t, stored.PasswordHash != "new-hash" || stored.Status != int64(identity.UserStatusNormal) || stored.TokenVersion != 2,
		"stored user = %#v, want updated credentials", stored)

}

func newTestCredentialStore(t *testing.T) *CredentialStore {
	t.Helper()
	client := enttest.Open(t, "sqlite3", fmt.Sprintf("file:credential_store_test_%s?mode=memory&cache=shared&_fk=1", runtimeid.MustNewUUIDString()))
	t.Cleanup(func() { _ = client.Close() })
	return NewCredentialStore(CredentialStoreParams{Client: client})
}

func createCredentialTestUser(t *testing.T, repo *CredentialStore, input credentialTestUserInput) *ent.User {
	t.Helper()
	created, err := repo.client.User.Create().
		SetNickname(input.Nickname).
		SetUserID(input.UserID).
		SetUsername(input.Username).
		SetPasswordHash(input.PasswordHash).
		SetStatus(int64(input.Status)).
		Save(context.Background())
	require.NoError(t, err,
		"Create user %q: %v", input.Username, err)

	return created
}

func createSoftDeletedCredentialUser(t *testing.T, repo *CredentialStore, input credentialTestUserInput) {
	t.Helper()
	ctx := context.Background()
	created := createCredentialTestUser(t, repo, input)
	{
		_, err := repo.client.User.UpdateOneID(created.ID).SetDeletedAt(deletedAtForCredentialTest).Save(ctx)
		require.NoError(t, err,
			"soft delete user: %v", err)
	}

}

func assertSameCredential(t *testing.T, got *authdomain.UserCredential, want *ent.User) {
	t.Helper()
	require.NotNil(t, got)
	require.NotNil(t, want)
	require.Equal(t, want.UserID, got.UserID)
	require.Equal(t, want.Username, got.Username)
	require.Equal(t, want.PasswordHash, got.PasswordHash)
	require.Equal(t, identity.UserStatus(want.Status), got.Status)
	require.Equal(t, want.TokenVersion, got.TokenVersion)
}
