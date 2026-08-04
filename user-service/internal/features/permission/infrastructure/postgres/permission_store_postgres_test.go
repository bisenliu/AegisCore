package postgres

import (
	"context"
	"database/sql"
	"fmt"
	"sync/atomic"
	"testing"
	"time"

	"entgo.io/ent/dialect"
	entsql "entgo.io/ent/dialect/sql"
	"github.com/google/uuid"
	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/stretchr/testify/require"

	"github.com/aegiscore/common/testing/containers"
	"github.com/aegiscore/user-service/internal/persistence/ent"
)

func TestPermissionStoreGetByPermissionIDsUsesOnePostgresQuery(t *testing.T) {
	client, store, counter := newPostgresPermissionStore(t)
	ctx := context.Background()

	for _, size := range []int{100, 1000} {
		t.Run(fmt.Sprintf("%d permission IDs", size), func(t *testing.T) {
			permissionIDs := createBulkPermissions(ctx, t, client, size)
			counter.Reset()

			permissions, err := store.GetByPermissionIDs(ctx, permissionIDs)

			require.NoError(t, err)
			require.Equal(t, permissionIDs, permissionDomainIDs(permissions))
			require.Equal(t, int64(1), counter.Count())
		})
	}
}

type queryCountingDriver struct {
	dialect.Driver
	queryCount atomic.Int64
}

func (d *queryCountingDriver) Query(ctx context.Context, query string, args, value any) error {
	d.queryCount.Add(1)
	return d.Driver.Query(ctx, query, args, value)
}

func (d *queryCountingDriver) Count() int64 {
	return d.queryCount.Load()
}

func (d *queryCountingDriver) Reset() {
	d.queryCount.Store(0)
}

func newPostgresPermissionStore(t *testing.T) (*ent.Client, *PermissionStore, *queryCountingDriver) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	t.Cleanup(cancel)

	postgres := containers.StartPostgres(ctx, t, containers.PostgresOptions{})
	db, err := sql.Open("pgx", postgres.DSN)
	require.NoError(t, err)
	_, err = db.ExecContext(ctx, "CREATE EXTENSION IF NOT EXISTS pg_trgm")
	require.NoError(t, err)

	driver := &queryCountingDriver{Driver: entsql.OpenDB(dialect.Postgres, db)}
	client := ent.NewClient(ent.Driver(driver))
	require.NoError(t, client.Schema.Create(ctx))
	t.Cleanup(func() { _ = client.Close() })
	return client, NewPermissionStore(client), driver
}

func createBulkPermissions(ctx context.Context, t *testing.T, client *ent.Client, size int) []uuid.UUID {
	t.Helper()
	permissionIDs := make([]uuid.UUID, 0, size)
	builders := make([]*ent.PermissionCreate, 0, size)
	for i := range size {
		permissionID := uuid.New()
		permissionIDs = append(permissionIDs, permissionID)
		builders = append(builders, client.Permission.Create().
			SetPermissionID(permissionID).
			SetName(fmt.Sprintf("Bulk Permission %d", i)).
			SetModule("role").
			SetHTTPMethod("GET").
			SetPathTemplate(fmt.Sprintf("/api/v1/bulk/%s/%d", permissionID.String(), i)))
	}
	_, err := client.Permission.CreateBulk(builders...).Save(ctx)
	require.NoError(t, err)
	return permissionIDs
}
