package postgres

import (
	"context"
	"database/sql"

	"github.com/aegiscore/common/runtime/datastore"
	"github.com/aegiscore/user-service/ent"
)

type entTxStarter struct {
	client *ent.Client
}

var _ datastore.TransactionStarter[*ent.Tx] = entTxStarter{}

func (s entTxStarter) BeginTransaction(ctx context.Context) (*ent.Tx, error) {
	return s.client.Tx(ctx)
}

type sqlTxStarter struct {
	db *sql.DB
}

var _ datastore.TransactionStarter[*sql.Tx] = sqlTxStarter{}

func (s sqlTxStarter) BeginTransaction(ctx context.Context) (*sql.Tx, error) {
	return s.db.BeginTx(ctx, nil)
}
