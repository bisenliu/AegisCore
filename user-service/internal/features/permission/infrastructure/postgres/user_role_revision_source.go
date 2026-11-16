package postgres

import (
	"context"
	"fmt"

	permissionapplication "github.com/aegiscore/user-service/internal/features/permission/application"
	"github.com/aegiscore/user-service/internal/persistence/ent"
	entrbacuserrolerevision "github.com/aegiscore/user-service/internal/persistence/ent/rbacuserrolerevision"
)

// UserRoleRevisionSource 从 PostgreSQL 读取已提交的 user-role revision。
type UserRoleRevisionSource struct {
	client *ent.Client
}

var _ permissionapplication.UserRoleRevisionSource = (*UserRoleRevisionSource)(nil)

// NewUserRoleRevisionSource 构造数据库 user-role revision 查询适配器。
func NewUserRoleRevisionSource(client *ent.Client) *UserRoleRevisionSource {
	return &UserRoleRevisionSource{client: client}
}

// UserRoleRevisionsAfter 返回高于 afterRevision 的有界 user-role revision。
// 查询到 limit+1 行时标记截断，调用方必须全量失效缓存，不能只使用返回的部分用户。
func (s *UserRoleRevisionSource) UserRoleRevisionsAfter(ctx context.Context, afterRevision int64, limit int) (permissionapplication.UserRoleRevisionBatch, error) {
	if limit <= 0 {
		limit = permissionapplication.DefaultUserRoleRevisionBatchSize
	}
	rows, err := s.client.RbacUserRoleRevision.Query().
		Where(entrbacuserrolerevision.IDGT(afterRevision)).
		Order(entrbacuserrolerevision.ByID()).
		Limit(limit + 1).
		All(ctx)
	if err != nil {
		return permissionapplication.UserRoleRevisionBatch{}, fmt.Errorf("query rbac user-role revisions after %d: %w", afterRevision, err)
	}
	truncated := len(rows) > limit
	if truncated {
		rows = rows[:limit]
	}
	changes := make([]permissionapplication.UserRoleRevisionChange, 0, len(rows))
	latest := afterRevision
	for _, row := range rows {
		changes = append(changes, permissionapplication.UserRoleRevisionChange{Revision: row.ID, UserID: row.UserID})
		latest = row.ID
	}
	return permissionapplication.UserRoleRevisionBatch{Changes: changes, LatestRevision: latest, Truncated: truncated}, nil
}
