package casbin

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"go.uber.org/fx"

	"github.com/aegiscore/user-service/ent"
	entpermission "github.com/aegiscore/user-service/ent/permission"
	entrole "github.com/aegiscore/user-service/ent/role"
	entrolepermission "github.com/aegiscore/user-service/ent/rolepermission"
	"github.com/aegiscore/user-service/internal/shared/rbacbaseline"
)

const policyWildcard = "*"

// PolicySet 包含一次全量加载得到的 Casbin policy 数据。
type PolicySet struct {
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
	LoadPolicies(ctx context.Context) (PolicySet, error)
}

// LoaderParams 包含 Ent-backed policy loader 的 Fx 输入。
type LoaderParams struct {
	fx.In

	Client *ent.Client `name:"primary_db"`
}

type entLoader struct {
	client           *ent.Client
	superAdminRoleID uuid.UUID
}

// NewPolicyLoader 构造基于 Ent 的 Casbin policy loader。
func NewPolicyLoader(params LoaderParams) Loader {
	return &entLoader{client: params.Client, superAdminRoleID: uuid.MustParse(rbacbaseline.SuperAdminRoleID)}
}

func (l *entLoader) LoadPolicies(ctx context.Context) (PolicySet, error) {
	rules, err := l.loadPermissionRules(ctx)
	if err != nil {
		return PolicySet{}, err
	}
	// 超级管理员授权是固定基线 role 的内存 wildcard policy，不来自权限目录；route diff 和权限列表不应把它当作数据库权限记录。
	rules = append(rules, PermissionRule{RoleID: l.superAdminRoleID, PathTemplate: policyWildcard, HTTPMethod: policyWildcard})
	return PolicySet{PermissionRules: rules}, nil
}

func (l *entLoader) loadPermissionRules(ctx context.Context) ([]PermissionRule, error) {
	bindings, err := l.client.RolePermission.Query().
		Where(
			entrolepermission.HasRoleWith(entrole.ActiveEQ(true)),
			entrolepermission.HasPermissionWith(entpermission.ActiveEQ(true)),
		).
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
