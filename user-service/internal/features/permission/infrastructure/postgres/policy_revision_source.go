package postgres

import (
	"context"
	"fmt"

	entsql "entgo.io/ent/dialect/sql"

	permissionapplication "github.com/aegiscore/user-service/internal/features/permission/application"
	"github.com/aegiscore/user-service/internal/persistence/ent"
	entrbacpolicyrevision "github.com/aegiscore/user-service/internal/persistence/ent/rbacpolicyrevision"
)

// PolicyRevisionSource 从 PostgreSQL 读取最新已提交的 RBAC policy revision。
type PolicyRevisionSource struct {
	client *ent.Client
}

var _ permissionapplication.LatestPolicyRevisionSource = (*PolicyRevisionSource)(nil)

// NewPolicyRevisionSource 构造数据库 policy revision 查询适配器。
func NewPolicyRevisionSource(client *ent.Client) *PolicyRevisionSource {
	return &PolicyRevisionSource{client: client}
}

// LatestPolicyRevision 返回数据库当前可见的最新 policy revision；空表返回 0。
func (s *PolicyRevisionSource) LatestPolicyRevision(ctx context.Context) (int64, error) {
	revision, err := s.client.RbacPolicyRevision.Query().
		Order(entrbacpolicyrevision.ByID(entsql.OrderDesc())).
		FirstID(ctx)
	if ent.IsNotFound(err) {
		return 0, nil
	}
	if err != nil {
		return 0, fmt.Errorf("query latest rbac policy revision: %w", err)
	}
	return revision, nil
}
