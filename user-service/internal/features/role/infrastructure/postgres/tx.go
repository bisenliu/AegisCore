package postgres

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"uuid"

	entsql "entgo.io/ent/dialect/sql"

	"github.com/aegiscore/common/runtime/datastore"
	roleapplication "github.com/aegiscore/user-service/internal/features/role/application"
	"github.com/aegiscore/user-service/internal/persistence/ent"
	entrbacpolicyrevision "github.com/aegiscore/user-service/internal/persistence/ent/rbacpolicyrevision"
	entrbacpolicyrevisioncounter "github.com/aegiscore/user-service/internal/persistence/ent/rbacpolicyrevisioncounter"
	entrbacuserrolerevision "github.com/aegiscore/user-service/internal/persistence/ent/rbacuserrolerevision"
	entrbacuserrolerevisioncounter "github.com/aegiscore/user-service/internal/persistence/ent/rbacuserrolerevisioncounter"
)

// policyRevisionCounterID 是权限策略版本自增计数器在数据库中的固定记录主键 ID。
const policyRevisionCounterID int64 = 1

// userRoleRevisionCounterID 是用户角色绑定版本自增计数器在数据库中的固定记录主键 ID。
const userRoleRevisionCounterID int64 = 1

type entTxStarter struct {
	client *ent.Client
}

var _ datastore.TransactionStarter[*ent.Tx] = entTxStarter{}

// BeginTransaction 开启数据库写入事务，确保后续多步操作要么全部成功生效，要么全部回滚撤销。
func (s entTxStarter) BeginTransaction(ctx context.Context) (*ent.Tx, error) {
	return s.client.Tx(ctx)
}

type sqlTxStarter struct {
	db *sql.DB
}

var _ datastore.TransactionStarter[*sql.Tx] = sqlTxStarter{}

// BeginTransaction 开启底层数据库写入事务。
func (s sqlTxStarter) BeginTransaction(ctx context.Context) (*sql.Tx, error) {
	return s.db.BeginTx(ctx, nil)
}

// transactPolicyChange 将权限规则的业务修改与递增的版本号放在同一个数据库事务中原子提交。
// 该方法返回成功即代表数据库中的权限数据已安全保存；随后应用层会基于新版本号立即刷新内存中的权限规则。
func transactPolicyChange[T any](ctx context.Context, client *ent.Client, operation string, change roleapplication.PolicyChange, mutate func(*ent.Tx) (T, error)) (value T, result roleapplication.PolicyWriteResult, err error) {
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
		SetRoleID(nonNilUUID(change.RoleID)).
		SetPermissionID(nonNilUUID(change.PermissionID)).
		Save(ctx)
	if err != nil {
		return value, result, finish.Fail(fmt.Errorf("append rbac policy revision after %s: %w", operation, err))
	}
	if err := finish.Commit(ctx); err != nil {
		return value, result, finish.Fail(fmt.Errorf("commit %s: %w", operation, err))
	}
	return value, roleapplication.PolicyWriteResult{Revision: revision.ID}, nil
}

// transactUserRoleChange 将用户角色绑定写入和单调 user-role revision 原子提交。
func transactUserRoleChange[T any](ctx context.Context, client *ent.Client, operation string, change roleapplication.PolicyChange, mutate func(*ent.Tx) (T, error)) (value T, result roleapplication.PolicyWriteResult, err error) {
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
	counter, err := allocateUserRoleRevision(ctx, tx)
	if err != nil {
		return value, result, finish.Fail(fmt.Errorf("allocate rbac user-role revision after %s: %w", operation, err))
	}
	revision, err := tx.RbacUserRoleRevision.Create().
		SetID(counter.LastRevision).
		SetReason(change.Reason).
		SetUserID(change.UserID).
		SetRoleID(nonNilUUID(change.RoleID)).
		Save(ctx)
	if err != nil {
		return value, result, finish.Fail(fmt.Errorf("append rbac user-role revision after %s: %w", operation, err))
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

// allocateUserRoleRevision 原子递增固定 counter，并在 counter 缺失时与已提交最大 revision 幂等对齐。
func allocateUserRoleRevision(ctx context.Context, tx *ent.Tx) (*ent.RbacUserRoleRevisionCounter, error) {
	counter, err := tx.RbacUserRoleRevisionCounter.UpdateOneID(userRoleRevisionCounterID).
		AddLastRevision(1).
		Save(ctx)
	if err == nil {
		return counter, nil
	}
	if !ent.IsNotFound(err) {
		return nil, fmt.Errorf("increment rbac user-role revision counter: %w", err)
	}

	lastRevision := int64(0)
	latest, err := tx.RbacUserRoleRevision.Query().
		Order(entrbacuserrolerevision.ByID(entsql.OrderDesc())).
		First(ctx)
	switch {
	case err == nil:
		lastRevision = latest.ID
	case ent.IsNotFound(err):
	default:
		return nil, fmt.Errorf("read latest rbac user-role revision: %w", err)
	}

	err = tx.RbacUserRoleRevisionCounter.Create().
		SetID(userRoleRevisionCounterID).
		SetLastRevision(lastRevision).
		OnConflictColumns(entrbacuserrolerevisioncounter.FieldID).
		Ignore().
		Exec(ctx)
	if err != nil {
		return nil, fmt.Errorf("initialize rbac user-role revision counter: %w", err)
	}

	counter, err = tx.RbacUserRoleRevisionCounter.UpdateOneID(userRoleRevisionCounterID).
		AddLastRevision(1).
		Save(ctx)
	if err != nil {
		return nil, fmt.Errorf("increment initialized rbac user-role revision counter: %w", err)
	}
	return counter, nil
}

// nonNilUUID 将全零的无效编号转为空指针，防止把无意义的空编号误存入数据库可选字段中。
func nonNilUUID(value uuid.UUID) *uuid.UUID {
	if value == uuid.Nil() {
		return nil
	}
	return &value
}
