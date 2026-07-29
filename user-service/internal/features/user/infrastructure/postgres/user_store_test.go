package postgres

import (
	"context"
	"fmt"
	"testing"

	"github.com/google/uuid"
	_ "github.com/mattn/go-sqlite3"
	"github.com/stretchr/testify/require"

	runtimeid "github.com/aegiscore/common/runtime/id"
	userapplication "github.com/aegiscore/user-service/internal/features/user/application"
	userdomain "github.com/aegiscore/user-service/internal/features/user/domain"
	"github.com/aegiscore/user-service/internal/persistence/ent/enttest"
	"github.com/aegiscore/user-service/internal/shared/identity"
)

const deletedAtForTest int64 = 1710000000000

func TestUserRepositoryDomainErrors(t *testing.T) {
	repo := newTestUserStore(t)

	t.Run("get by user id returns domain not found", func(t *testing.T) {
		_, err := repo.GetByUserID(context.Background(), uuid.MustParse("018f0000-0000-7000-8000-000000000999"))

		require.ErrorIs(t, err, identity.ErrUserNotFound)
	})

	t.Run("get by user id ignores soft deleted user", func(t *testing.T) {
		ctx := context.Background()
		userID := uuid.MustParse("018f0000-0000-7000-8000-000000000010")
		createSoftDeletedUser(t, repo, userapplication.CreateUserInput{Nickname: "Deleted Profile", UserID: userID, Username: "deleted-profile", PasswordHash: "hash", Status: identity.UserStatusNormal})

		_, err := repo.GetByUserID(ctx, userID)

		require.ErrorIs(t, err, identity.ErrUserNotFound)
	})

	t.Run("create uniqueness violation returns domain already exists", func(t *testing.T) {
		ctx := context.Background()
		_, err := repo.Create(ctx, userapplication.CreateUserInput{Nickname: "Alice", UserID: uuid.MustParse("018f0000-0000-7000-8000-000000000001"), Username: "alice", PasswordHash: "hash", Status: identity.UserStatusNormal})
		require.NoError(t, err)

		_, err = repo.Create(ctx, userapplication.CreateUserInput{Nickname: "Alice 2", UserID: uuid.MustParse("018f0000-0000-7000-8000-000000000002"), Username: "alice", PasswordHash: "hash", Status: identity.UserStatusNormal})

		require.ErrorIs(t, err, identity.ErrUserAlreadyExists)
	})

	t.Run("create duplicate user id returns domain already exists", func(t *testing.T) {
		ctx := context.Background()
		userID := uuid.MustParse("018f0000-0000-7000-8000-000000000013")
		_, err := repo.Create(ctx, userapplication.CreateUserInput{Nickname: "Duplicate ID", UserID: userID, Username: "duplicate-id", PasswordHash: "hash", Status: identity.UserStatusNormal})
		require.NoError(t, err)

		_, err = repo.Create(ctx, userapplication.CreateUserInput{Nickname: "Duplicate ID 2", UserID: userID, Username: "duplicate-id-2", PasswordHash: "hash", Status: identity.UserStatusNormal})

		require.ErrorIs(t, err, identity.ErrUserAlreadyExists)
	})

	t.Run("soft deleted username remains reserved", func(t *testing.T) {
		ctx := context.Background()
		createSoftDeletedUser(t, repo, userapplication.CreateUserInput{Nickname: "Deleted Alice", UserID: uuid.MustParse("018f0000-0000-7000-8000-000000000003"), Username: "reserved-alice", PasswordHash: "hash", Status: identity.UserStatusNormal})

		_, err := repo.Create(ctx, userapplication.CreateUserInput{Nickname: "New Alice", UserID: uuid.MustParse("018f0000-0000-7000-8000-000000000004"), Username: "reserved-alice", PasswordHash: "hash", Status: identity.UserStatusNormal})

		require.ErrorIs(t, err, identity.ErrUserAlreadyExists)
	})
}

func TestUserRepositoryReturnsDomainUsers(t *testing.T) {
	repo := newTestUserStore(t)
	ctx := context.Background()
	userID := uuid.MustParse("018f0000-0000-7000-8000-000000000101")

	created, err := repo.Create(ctx, userapplication.CreateUserInput{Nickname: "Domain Alice", UserID: userID, Username: "domain-alice", PasswordHash: "hash", Status: identity.UserStatusMustChangePassword})
	require.NoError(t, err)
	require.NotZero(t, created.ID)
	require.Equal(t, userID, created.UserID)
	require.Equal(t, "Domain Alice", created.Nickname)
	require.Equal(t, "domain-alice", created.Username)
	require.Equal(t, "hash", created.PasswordHash)
	require.Equal(t, identity.UserStatusMustChangePassword, created.Status)
	require.Equal(t, int64(1), created.TokenVersion)
	require.NotZero(t, created.CreatedAt)
	require.NotZero(t, created.UpdatedAt)

	byID, err := repo.GetByUserID(ctx, userID)
	require.NoError(t, err)
	assertSameUser(t, byID, created)

	status := identity.UserStatusMustChangePassword
	users, hasNext, err := repo.ListUsers(ctx, userapplication.ListUsersInput{Limit: 10, Username: "domain-alice", Status: &status})
	require.NoError(t, err)
	require.False(t, hasNext)
	require.Len(t, users, 1)
	require.Equal(t, userID, users[0].UserID)
	require.Equal(t, "domain-alice", users[0].Username)
	require.Equal(t, identity.UserStatusMustChangePassword, users[0].Status)
}

func TestUserRepositoryListUsersBoundaries(t *testing.T) {
	t.Run("empty page", func(t *testing.T) {
		repo := newTestUserStore(t)

		users, hasNext, err := repo.ListUsers(context.Background(), userapplication.ListUsersInput{Limit: 10})

		require.NoError(t, err)
		require.False(t, hasNext)
		require.Empty(t, users)
	})

	t.Run("paginates with stable user id order", func(t *testing.T) {
		repo := newTestUserStore(t)
		ctx := context.Background()
		first := createTestUser(t, repo, userapplication.CreateUserInput{Nickname: "Page One", UserID: uuid.MustParse("018f0000-0000-7000-8000-000000000201"), Username: "page-one", PasswordHash: "hash-1", Status: identity.UserStatusNormal})
		second := createTestUser(t, repo, userapplication.CreateUserInput{Nickname: "Page Two", UserID: uuid.MustParse("018f0000-0000-7000-8000-000000000202"), Username: "page-two", PasswordHash: "hash-2", Status: identity.UserStatusDisabled})
		third := createTestUser(t, repo, userapplication.CreateUserInput{Nickname: "Page Three", UserID: uuid.MustParse("018f0000-0000-7000-8000-000000000203"), Username: "page-three", PasswordHash: "hash-3", Status: identity.UserStatusMustChangePassword})

		users, hasNext, err := repo.ListUsers(ctx, userapplication.ListUsersInput{Limit: 2})

		require.NoError(t, err)
		require.True(t, hasNext)
		require.Len(t, users, 2)
		require.Less(t, first.UserID.String(), second.UserID.String())
		require.Less(t, second.UserID.String(), third.UserID.String())
		assertSameUserValue(t, users[0], first)
		assertSameUserValue(t, users[1], second)

		users, hasNext, err = repo.ListUsers(ctx, userapplication.ListUsersInput{AfterUserID: &second.UserID, Limit: 2})
		require.NoError(t, err)
		require.False(t, hasNext)
		require.Len(t, users, 1)
		assertSameUserValue(t, users[0], third)
	})

	t.Run("filters by nickname username status and combined predicates", func(t *testing.T) {
		repo := newTestUserStore(t)
		ctx := context.Background()
		createTestUser(t, repo, userapplication.CreateUserInput{Nickname: "Alice Ops", UserID: uuid.MustParse("018f0000-0000-7000-8000-000000000211"), Username: "alice-ops", PasswordHash: "hash-1", Status: identity.UserStatusNormal})
		createTestUser(t, repo, userapplication.CreateUserInput{Nickname: "Alice Audit", UserID: uuid.MustParse("018f0000-0000-7000-8000-000000000212"), Username: "alice-audit", PasswordHash: "hash-2", Status: identity.UserStatusDisabled})
		createTestUser(t, repo, userapplication.CreateUserInput{Nickname: "Bob Ops", UserID: uuid.MustParse("018f0000-0000-7000-8000-000000000213"), Username: "bob-ops", PasswordHash: "hash-3", Status: identity.UserStatusMustChangePassword})

		assertListUsernames(t, repo, userapplication.ListUsersInput{Limit: 10, Nickname: "Alice"}, []string{"alice-ops", "alice-audit"})
		assertListUsernames(t, repo, userapplication.ListUsersInput{Limit: 10, Username: "bob-ops"}, []string{"bob-ops"})

		disabled := identity.UserStatusDisabled
		assertListUsernames(t, repo, userapplication.ListUsersInput{Limit: 10, Status: &disabled}, []string{"alice-audit"})
		assertListUsernames(t, repo, userapplication.ListUsersInput{Limit: 10, Nickname: "Alice", Username: "alice-audit", Status: &disabled}, []string{"alice-audit"})

		users, hasNext, err := repo.ListUsers(ctx, userapplication.ListUsersInput{Limit: 10, Nickname: "Missing"})
		require.NoError(t, err)
		require.False(t, hasNext)
		require.Empty(t, users)
	})

	t.Run("excludes soft deleted users from rows and has next", func(t *testing.T) {
		repo := newTestUserStore(t)
		ctx := context.Background()
		active := createTestUser(t, repo, userapplication.CreateUserInput{Nickname: "Active Alice", UserID: uuid.MustParse("018f0000-0000-7000-8000-000000000221"), Username: "active-alice", PasswordHash: "hash", Status: identity.UserStatusNormal})
		createSoftDeletedUser(t, repo, userapplication.CreateUserInput{Nickname: "Deleted Alice", UserID: uuid.MustParse("018f0000-0000-7000-8000-000000000222"), Username: "deleted-alice-list", PasswordHash: "hash", Status: identity.UserStatusNormal})

		users, hasNext, err := repo.ListUsers(ctx, userapplication.ListUsersInput{Limit: 10, Nickname: "Alice"})

		require.NoError(t, err)
		require.False(t, hasNext)
		require.Len(t, users, 1)
		assertSameUserValue(t, users[0], active)
	})
}

func TestUserListPredicates(t *testing.T) {
	require.Len(t, buildListPredicates(userapplication.ListUsersInput{}), 1)

	status := identity.UserStatusNormal
	got := buildListPredicates(userapplication.ListUsersInput{Nickname: "Ali", Username: "alice", Status: &status})
	require.Len(t, got, 4)
}

func newTestUserStore(t *testing.T) *UserStore {
	t.Helper()
	// 当前 store 只使用可移植的 Ent query/update 语义，因此 SQLite
	// 足以覆盖 integration boundary，不需要依赖只能通过 Docker 运行的 PostgreSQL 测试。
	client := enttest.Open(t, "sqlite3", fmt.Sprintf("file:user_store_test_%s?mode=memory&cache=shared&_fk=1", runtimeid.MustNewUUIDString()))
	t.Cleanup(func() { _ = client.Close() })
	return NewUserStore(client)
}

func createTestUser(t *testing.T, repo *UserStore, input userapplication.CreateUserInput) *userdomain.User {
	t.Helper()
	created, err := repo.Create(context.Background(), input)
	require.NoError(t, err)
	return created
}

func createSoftDeletedUser(t *testing.T, repo *UserStore, input userapplication.CreateUserInput) {
	t.Helper()
	ctx := context.Background()
	created, err := repo.Create(ctx, input)
	require.NoError(t, err)
	_, err = repo.client.User.UpdateOneID(created.ID).SetDeletedAt(deletedAtForTest).Save(ctx)
	require.NoError(t, err)
}

func assertSameUser(t *testing.T, got, want *userdomain.User) {
	t.Helper()
	require.NotNil(t, got)
	require.NotNil(t, want)
	assertSameUserValue(t, *got, want)
}

func assertSameUserValue(t *testing.T, got userdomain.User, want *userdomain.User) {
	t.Helper()
	require.NotNil(t, want)
	require.Equal(t, *want, got)
}

func assertListUsernames(t *testing.T, repo *UserStore, input userapplication.ListUsersInput, want []string) {
	t.Helper()
	users, hasNext, err := repo.ListUsers(context.Background(), input)
	require.NoError(t, err)
	require.False(t, hasNext)
	require.Len(t, users, len(want))
	got := make([]string, 0, len(users))
	for i := range want {
		got = append(got, users[i].Username)
	}
	require.Equal(t, want, got)
}
