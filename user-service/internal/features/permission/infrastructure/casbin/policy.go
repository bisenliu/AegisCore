package casbin

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"
	"uuid"

	entsql "entgo.io/ent/dialect/sql"

	"github.com/aegiscore/user-service/internal/persistence/ent"
	entrbacpolicyrevision "github.com/aegiscore/user-service/internal/persistence/ent/rbacpolicyrevision"
	entrole "github.com/aegiscore/user-service/internal/persistence/ent/role"
	entrolepermission "github.com/aegiscore/user-service/internal/persistence/ent/rolepermission"
	"github.com/aegiscore/user-service/internal/shared/rbacbaseline"
)

const (
	policyWildcard      = "*"
	policySnapshotRetry = 10 * time.Millisecond
)

// PolicySet 包含一次全量加载得到的 Casbin policy 数据。
type PolicySet struct {
	Revision        int64
	PermissionRules []PermissionRule
}

// PermissionRule 描述角色 subject 到路由权限的 Casbin p policy。
type PermissionRule struct {
	RoleID       uuid.UUID
	PathTemplate string
	HTTPMethod   string
}

// Loader 定义 Casbin 引擎构造 policy 时消费的数据加载端口。
type Loader interface {
	LoadPoliciesAtLeast(ctx context.Context, targetRevision int64) (PolicySet, error)
}

type entLoader struct {
	client           *ent.Client
	superAdminRoleID uuid.UUID
}

// NewPolicyLoader 构造基于 Ent 的 Casbin policy loader。
func NewPolicyLoader(client *ent.Client) Loader {
	return &entLoader{client: client, superAdminRoleID: uuid.MustParse(rbacbaseline.SuperAdminRoleID)}
}

// LoadPoliciesAtLeast 从数据库中读取版本号不低于指定要求的权限快照；如果数据库尚未就绪则提示可稍后重试。
func (l *entLoader) LoadPoliciesAtLeast(ctx context.Context, targetRevision int64) (PolicySet, error) {
	if targetRevision < 0 {
		return PolicySet{}, fmt.Errorf("load casbin policies: target revision must not be negative: %d", targetRevision)
	}

	// 在线 policy 写入会将 revision 与投影数据在同一事务提交；若只看到较旧快照，短暂重试而不是发布不完整 policy。
	for {
		policySet, stale, err := l.loadSnapshot(ctx, targetRevision)
		if err != nil {
			return PolicySet{}, err
		}
		if !stale {
			return policySet, nil
		}
		if err := waitForPolicySnapshot(ctx); err != nil {
			return PolicySet{}, fmt.Errorf("wait for casbin policy revision %d: %w", targetRevision, err)
		}
	}
}

// loadSnapshot 读取指定版本的权限数据快照；若版本尚未达到或读取不完整则不能视为加载成功。
func (l *entLoader) loadSnapshot(ctx context.Context, targetRevision int64) (PolicySet, bool, error) {
	// revision 与绑定必须来自同一 repeatable-read 快照，否则返回的 revision 可能与规则集合不对应。
	tx, err := l.client.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelRepeatableRead, ReadOnly: true})
	if err != nil {
		return PolicySet{}, false, fmt.Errorf("begin casbin policy snapshot: %w", err)
	}

	revision, err := latestPolicyRevision(ctx, tx)
	if err != nil {
		return PolicySet{}, false, finishPolicySnapshot(tx, fmt.Errorf("load latest casbin policy revision: %w", err))
	}
	if revision < targetRevision {
		if err := finishPolicySnapshot(tx, nil); err != nil {
			return PolicySet{}, false, err
		}
		return PolicySet{}, true, nil
	}

	rules, err := l.loadPermissionRules(ctx, tx)
	if err != nil {
		return PolicySet{}, false, finishPolicySnapshot(tx, err)
	}
	// 超级管理员授权是固定基线 role 的内存 wildcard policy，不来自权限目录；route diff 和权限列表不应把它当作数据库权限记录。
	rules = append(rules, PermissionRule{RoleID: l.superAdminRoleID, PathTemplate: policyWildcard, HTTPMethod: policyWildcard})
	if err := finishPolicySnapshot(tx, nil); err != nil {
		return PolicySet{}, false, err
	}
	return PolicySet{Revision: revision, PermissionRules: rules}, false, nil
}

// latestPolicyRevision 从数据库中查询系统当前的最新权限规则版本号。
func latestPolicyRevision(ctx context.Context, tx *ent.Tx) (int64, error) {
	revision, err := tx.RbacPolicyRevision.Query().
		Order(entrbacpolicyrevision.ByID(entsql.OrderDesc())).
		FirstID(ctx)
	if ent.IsNotFound(err) {
		return 0, nil
	}
	return revision, err
}

// loadPermissionRules 查询数据库中所有当前处于启用状态的角色与功能权限绑定清单。
func (l *entLoader) loadPermissionRules(ctx context.Context, tx *ent.Tx) ([]PermissionRule, error) {
	bindings, err := tx.RolePermission.Query().
		Where(entrolepermission.HasRoleWith(entrole.ActiveEQ(true))).
		WithRole().
		WithPermission().
		Order(entrolepermission.ByRoleID(), entrolepermission.ByPermissionID()).
		All(ctx)
	if err != nil {
		return nil, fmt.Errorf("load casbin permission policies: %w", err)
	}
	rules := make([]PermissionRule, 0, len(bindings)+1)
	for _, binding := range bindings {
		role, err := binding.Edges.RoleOrErr()
		if err != nil {
			return nil, fmt.Errorf("load casbin permission policy role edge: %w", err)
		}
		permission, err := binding.Edges.PermissionOrErr()
		if err != nil {
			return nil, fmt.Errorf("load casbin permission policy permission edge: %w", err)
		}
		rules = append(rules, PermissionRule{RoleID: role.RoleID, PathTemplate: permission.PathTemplate, HTTPMethod: permission.HTTPMethod})
	}
	return rules, nil
}

// finishPolicySnapshot 整理读取完毕的权限规则列表，打包成完整的权限快照数据。
func finishPolicySnapshot(tx *ent.Tx, loadErr error) error {
	if rollbackErr := tx.Rollback(); rollbackErr != nil {
		rollbackErr = fmt.Errorf("rollback casbin policy snapshot: %w", rollbackErr)
		if loadErr != nil {
			return errors.Join(loadErr, rollbackErr)
		}
		return rollbackErr
	}
	return loadErr
}

// waitForPolicySnapshot 当检测到数据库版本尚未达到通知中的目标版本时，稍作等待以让数据库事务完成提交。
func waitForPolicySnapshot(ctx context.Context) error {
	timer := time.NewTimer(policySnapshotRetry)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}
