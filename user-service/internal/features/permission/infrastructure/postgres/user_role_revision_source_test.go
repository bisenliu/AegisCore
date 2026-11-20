package postgres

import (
	"context"
	"fmt"
	"testing"
	"time"
	"uuid"

	_ "github.com/mattn/go-sqlite3"
	"github.com/stretchr/testify/require"

	runtimeid "github.com/aegiscore/platform/runtime/id"
	permissionapplication "github.com/aegiscore/user-service/internal/features/permission/application"
	"github.com/aegiscore/user-service/internal/persistence/ent/enttest"
)

func TestUserRoleRevisionSourceReturnsBoundedChangesAfterWatermark(t *testing.T) {
	client := enttest.Open(t, "sqlite3", fmt.Sprintf("file:user_role_revision_source_%s?mode=memory&cache=shared&_fk=1", runtimeid.MustNewUUIDString()))
	t.Cleanup(func() { _ = client.Close() })
	source := NewUserRoleRevisionSource(client)
	firstUser := uuid.MustParse("018f0000-0000-7000-8000-000000000911")
	secondUser := uuid.MustParse("018f0000-0000-7000-8000-000000000912")
	thirdUser := uuid.MustParse("018f0000-0000-7000-8000-000000000913")
	for revision, userID := range map[int64]uuid.UUID{2: firstUser, 4: secondUser, 7: thirdUser} {
		_, err := client.RbacUserRoleRevision.Create().
			SetID(revision).
			SetReason("user_role_added").
			SetUserID(userID).
			SetCreatedAt(time.Now().UnixMilli()).
			Save(context.Background())
		require.NoError(t, err)
	}

	empty, err := source.UserRoleRevisionsAfter(context.Background(), 7, 1)
	require.NoError(t, err)
	require.Empty(t, empty.Changes)
	require.Equal(t, int64(7), empty.LatestRevision)
	require.False(t, empty.Truncated)

	batch, err := source.UserRoleRevisionsAfter(context.Background(), 2, 1)
	require.NoError(t, err)
	require.Equal(t, permissionapplication.UserRoleRevisionBatch{
		Changes:        []permissionapplication.UserRoleRevisionChange{{Revision: 4, UserID: secondUser}},
		LatestRevision: 4,
		Truncated:      true,
	}, batch)
}
