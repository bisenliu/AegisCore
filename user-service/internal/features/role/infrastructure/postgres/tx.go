package postgres

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	entsql "entgo.io/ent/dialect/sql"
	"github.com/google/uuid"

	"github.com/aegiscore/common/runtime/datastore"
	runtimeid "github.com/aegiscore/common/runtime/id"
	roleapplication "github.com/aegiscore/user-service/internal/features/role/application"
	"github.com/aegiscore/user-service/internal/persistence/ent"
	entrbacpolicyrevision "github.com/aegiscore/user-service/internal/persistence/ent/rbacpolicyrevision"
	entrbacpolicyrevisioncounter "github.com/aegiscore/user-service/internal/persistence/ent/rbacpolicyrevisioncounter"
)

const policyRevisionCounterID int64 = 1

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

// transactPolicyChange 将业务写入、单调 revision 和 outbox event 原子提交。
// 返回成功即表示数据库事实和跨实例补偿事件均已持久化；本实例即时 reload 在事务提交后由 application 层触发。
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
	counter, err := allocatePolicyRevision(ctx, tx)
	if err != nil {
		return value, result, finish.Fail(fmt.Errorf("allocate rbac policy revision after %s: %w", operation, err))
	}
	revision, err := tx.RbacPolicyRevision.Create().
		SetID(counter.LastRevision).
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

// allocatePolicyRevision 原子递增固定 counter，并在 counter 缺失时与已提交最大 revision 幂等对齐。
func allocatePolicyRevision(ctx context.Context, tx *ent.Tx) (*ent.RbacPolicyRevisionCounter, error) {
	counter, err := tx.RbacPolicyRevisionCounter.UpdateOneID(policyRevisionCounterID).
		AddLastRevision(1).
		Save(ctx)
	if err == nil {
		return counter, nil
	}
	if !ent.IsNotFound(err) {
		return nil, fmt.Errorf("increment rbac policy revision counter: %w", err)
	}

	lastRevision := int64(0)
	latest, err := tx.RbacPolicyRevision.Query().
		Order(entrbacpolicyrevision.ByID(entsql.OrderDesc())).
		First(ctx)
	switch {
	case err == nil:
		lastRevision = latest.ID
	case ent.IsNotFound(err):
	default:
		return nil, fmt.Errorf("read latest rbac policy revision: %w", err)
	}

	err = tx.RbacPolicyRevisionCounter.Create().
		SetID(policyRevisionCounterID).
		SetLastRevision(lastRevision).
		OnConflictColumns(entrbacpolicyrevisioncounter.FieldID).
		Ignore().
		Exec(ctx)
	if err != nil {
		return nil, fmt.Errorf("initialize rbac policy revision counter: %w", err)
	}

	// 并发初始化只有一个事务会创建 counter；冲突被忽略后所有事务都通过行更新串行分配 revision。
	counter, err = tx.RbacPolicyRevisionCounter.UpdateOneID(policyRevisionCounterID).
		AddLastRevision(1).
		Save(ctx)
	if err != nil {
		return nil, fmt.Errorf("increment initialized rbac policy revision counter: %w", err)
	}
	return counter, nil
}

func nonNilUUID(value uuid.UUID) *uuid.UUID {
	if value == uuid.Nil {
		return nil
	}
	return &value
}
