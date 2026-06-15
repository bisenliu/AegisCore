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
	entuser "github.com/aegiscore/user-service/ent/user"
	entuserrole "github.com/aegiscore/user-service/ent/userrole"
	"github.com/aegiscore/user-service/internal/features/permission/application/rbacbaseline"
)

const policyWildcard = "*"

// PolicySet 包含一次全量加载得到的 Casbin policy 数据。
type PolicySet struct {
	GroupingPolicies []GroupingPolicy
	PermissionRules  []PermissionRule
}

// GroupingPolicy 描述用户 subject 到角色 subject 的 Casbin g policy。
type GroupingPolicy struct {
	UserID uuid.UUID
	RoleID uuid.UUID
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

	Client *ent.Client `name:"user_db"`
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
	groups, err := l.loadGroupingPolicies(ctx)
	if err != nil {
		return PolicySet{}, err
	}
	rules, err := l.loadPermissionRules(ctx)
	if err != nil {
		return PolicySet{}, err
	}
	rules = append(rules, PermissionRule{RoleID: l.superAdminRoleID, PathTemplate: policyWildcard, HTTPMethod: policyWildcard})
	return PolicySet{GroupingPolicies: groups, PermissionRules: rules}, nil
}

func (l *entLoader) loadGroupingPolicies(ctx context.Context) ([]GroupingPolicy, error) {
	bindings, err := l.client.UserRole.Query().
		Where(
			entuserrole.HasUserWith(entuser.DeletedAtIsNil()),
			entuserrole.HasRoleWith(entrole.ActiveEQ(true)),
		).
		WithUser().
		WithRole().
		Order(entuserrole.ByUserID(), entuserrole.ByRoleID()).
		All(ctx)
	if err != nil {
		return nil, fmt.Errorf("load casbin grouping policies: %w", err)
	}
	groups := make([]GroupingPolicy, 0, len(bindings))
	for _, binding := range bindings {
		user, err := binding.Edges.UserOrErr()
		if err != nil {
			return nil, fmt.Errorf("load casbin grouping policy user edge: %w", err)
		}
		role, err := binding.Edges.RoleOrErr()
		if err != nil {
			return nil, fmt.Errorf("load casbin grouping policy role edge: %w", err)
		}
		groups = append(groups, GroupingPolicy{UserID: user.UserID, RoleID: role.RoleID})
	}
	return groups, nil
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

func userSubject(userID uuid.UUID) string {
	return "user:" + userID.String()
}

func roleSubject(roleID uuid.UUID) string {
	return "role:" + roleID.String()
}
