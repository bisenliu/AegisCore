package postgres

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"entgo.io/ent"
	_ "github.com/mattn/go-sqlite3"
	"github.com/stretchr/testify/require"

	runtimeid "github.com/aegiscore/common/runtime/id"
	"github.com/aegiscore/user-service/internal/persistence/ent/enttest"
)

func TestPolicyRevisionSourceReturnsZeroForEmptyTableAndLatestRevision(t *testing.T) {
	client := enttest.Open(t, "sqlite3", fmt.Sprintf("file:permission_revision_source_%s?mode=memory&cache=shared&_fk=1", runtimeid.MustNewUUIDString()))
	t.Cleanup(func() { _ = client.Close() })
	source := NewPolicyRevisionSource(client)

	revision, err := source.LatestPolicyRevision(context.Background())
	require.NoError(t, err)
	require.Zero(t, revision)

	for _, value := range []int64{2, 7, 4} {
		_, err = client.RbacPolicyRevision.Create().
			SetID(value).
			SetReason("role_updated").
			SetCreatedAt(time.Now().UnixMilli()).
			Save(context.Background())
		require.NoError(t, err)
	}
	revision, err = source.LatestPolicyRevision(context.Background())
	require.NoError(t, err)
	require.Equal(t, int64(7), revision)
}

func TestPolicyRevisionSourcePreservesQueryError(t *testing.T) {
	queryErr := errors.New("revision query failed")
	client := enttest.Open(t, "sqlite3", fmt.Sprintf("file:permission_revision_source_error_%s?mode=memory&cache=shared&_fk=1", runtimeid.MustNewUUIDString()))
	t.Cleanup(func() { _ = client.Close() })
	client.RbacPolicyRevision.Intercept(ent.InterceptFunc(func(ent.Querier) ent.Querier {
		return ent.QuerierFunc(func(context.Context, ent.Query) (ent.Value, error) {
			return nil, queryErr
		})
	}))

	_, err := NewPolicyRevisionSource(client).LatestPolicyRevision(context.Background())
	require.ErrorIs(t, err, queryErr)
}

func TestPolicyRevisionSourceHonorsCanceledContext(t *testing.T) {
	client := enttest.Open(t, "sqlite3", fmt.Sprintf("file:permission_revision_source_context_%s?mode=memory&cache=shared&_fk=1", runtimeid.MustNewUUIDString()))
	t.Cleanup(func() { _ = client.Close() })
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := NewPolicyRevisionSource(client).LatestPolicyRevision(ctx)
	require.ErrorIs(t, err, context.Canceled)
}
