package postgres

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/google/uuid"

	"github.com/aegiscore/common/runtime/datastore"
	runtimeid "github.com/aegiscore/common/runtime/id"
	roleapplication "github.com/aegiscore/user-service/internal/features/role/application"
	"github.com/aegiscore/user-service/internal/persistence/ent"
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

func transactPolicyChange[T any](ctx context.Context, client *ent.Client, operation string, change roleapplication.PolicyChange, mutate func(*ent.Tx) (T, error)) (value T, result roleapplication.PolicyWriteResult, err error) {
	eventID, err := runtimeid.NewUUID()
	if err != nil {
		return value, result, fmt.Errorf("generate rbac policy outbox event id: %w", err)
	}
	tx, finish, err := datastore.BeginTransaction(ctx, entTxStarter{client: client})
	if err != nil {
		return value, result, fmt.Errorf("begin %s: %w", operation, err)
	}
	defer func() {
		if rollbackErr := finish.RollbackUnlessCommitted(); rollbackErr != nil {
			err = errors.Join(err, rollbackErr)
		}
	}()

	value, err = mutate(tx)
	if err != nil {
		return value, result, finish.Fail(err)
	}
	revision, err := tx.RbacPolicyRevision.Create().
		SetReason(change.Reason).
		SetNillableRoleID(nonNilUUID(change.RoleID)).
		SetNillableUserID(nonNilUUID(change.UserID)).
		SetNillablePermissionID(nonNilUUID(change.PermissionID)).
		Save(ctx)
	if err != nil {
		return value, result, finish.Fail(fmt.Errorf("append rbac policy revision after %s: %w", operation, err))
	}
	_, err = tx.RbacPolicyOutboxEvent.Create().
		SetEventID(eventID).
		SetRevision(revision.ID).
		SetKind(string(change.Kind)).
		SetReason(change.Reason).
		SetNillableRoleID(nonNilUUID(change.RoleID)).
		SetNillableUserID(nonNilUUID(change.UserID)).
		SetNillablePermissionID(nonNilUUID(change.PermissionID)).
		SetIdempotencyKey(fmt.Sprintf("rbac-policy-revision:%d", revision.ID)).
		SetPolicyRevisionID(revision.ID).
		Save(ctx)
	if err != nil {
		return value, result, finish.Fail(fmt.Errorf("append rbac policy outbox event after %s: %w", operation, err))
	}
	if err := finish.Commit(ctx); err != nil {
		return value, result, finish.Fail(fmt.Errorf("commit %s: %w", operation, err))
	}
	return value, roleapplication.PolicyWriteResult{Revision: revision.ID}, nil
}

func nonNilUUID(value uuid.UUID) *uuid.UUID {
	if value == uuid.Nil {
		return nil
	}
	return &value
}
