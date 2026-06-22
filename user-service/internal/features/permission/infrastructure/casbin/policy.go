package casbin

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"go.uber.org/fx"

	"github.com/aegiscore/common/runtime/config"
	"github.com/aegiscore/common/runtime/localcache"
	"github.com/aegiscore/user-service/ent"
	entpermission "github.com/aegiscore/user-service/ent/permission"
	entrole "github.com/aegiscore/user-service/ent/role"
	entrolepermission "github.com/aegiscore/user-service/ent/rolepermission"
	entuser "github.com/aegiscore/user-service/ent/user"
	entuserrole "github.com/aegiscore/user-service/ent/userrole"
	"github.com/aegiscore/user-service/internal/shared/rbacbaseline"
)

const policyWildcard = "*"

const rbacUserRolesCacheName = "rbac_user_roles"

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

// UserRoleResolver 定义授权热路径按需解析用户启用角色的端口。
type UserRoleResolver interface {
	RolesForUser(ctx context.Context, userID uuid.UUID) ([]uuid.UUID, error)
	InvalidateUserRole(userID uuid.UUID)
	InvalidateAllUserRoles()
}

// LoaderParams 包含 Ent-backed policy loader 的 Fx 输入。
type LoaderParams struct {
	fx.In

	Client *ent.Client `name:"user_db"`
}

// UserRoleResolverParams 包含用户角色 resolver 的 Fx 输入。
type UserRoleResolverParams struct {
	fx.In

	Lifecycle fx.Lifecycle
	Config    *config.Config
	Client    *ent.Client `name:"user_db"`
}

// UserRoleResolverResult 暴露 resolver 和对应的缓存统计源。
type UserRoleResolverResult struct {
	fx.Out

	Resolver UserRoleResolver
	Stats    localcache.StatsSource `name:"rbac_user_roles_cache"`
}

type entLoader struct {
	client           *ent.Client
	superAdminRoleID uuid.UUID
}

type entUserRoleResolver struct {
	cache *localcache.Cache[uuid.UUID, []uuid.UUID]
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

// NewUserRoleResolver 构造按用户 bounded TTL 缓存的角色解析器。
func NewUserRoleResolver(params UserRoleResolverParams) (UserRoleResolverResult, error) {
	cfg := params.Config.LocalCache.RBACUserRoles
	cache, err := localcache.New[uuid.UUID, []uuid.UUID](localcache.Config[uuid.UUID]{
		Name:        rbacUserRolesCacheName,
		Capacity:    cfg.Capacity,
		TTL:         cfg.TTL,
		LoadTimeout: cfg.LoadTimeout,
		KeyString:   func(userID uuid.UUID) string { return userID.String() },
		NumCounters: cfg.NumCounters,
		BufferItems: cfg.BufferItems,
	}, func(ctx context.Context, userID uuid.UUID) ([]uuid.UUID, error) {
		return loadRolesForUser(ctx, params.Client, userID)
	}, cloneRoleIDs)
	if err != nil {
		return UserRoleResolverResult{}, fmt.Errorf("create rbac user roles localcache: %w", err)
	}

	params.Lifecycle.Append(fx.Hook{OnStop: func(context.Context) error {
		cache.Close()
		return nil
	}})
	return UserRoleResolverResult{Resolver: &entUserRoleResolver{cache: cache}, Stats: cache}, nil
}

func newUserRoleResolver(cache *localcache.Cache[uuid.UUID, []uuid.UUID]) *entUserRoleResolver {
	return &entUserRoleResolver{cache: cache}
}

// RolesForUser 返回用户当前绑定的启用角色 ID。
func (r *entUserRoleResolver) RolesForUser(ctx context.Context, userID uuid.UUID) ([]uuid.UUID, error) {
	roleIDs, err := r.cache.GetOrLoad(ctx, userID)
	if err != nil {
		return nil, err
	}
	return roleIDs, nil
}

// InvalidateUserRole 删除单个用户的本地角色缓存。
func (r *entUserRoleResolver) InvalidateUserRole(userID uuid.UUID) {
	_ = r.cache.Delete(userID)
}

// InvalidateAllUserRoles 清空本实例全部用户角色缓存。
func (r *entUserRoleResolver) InvalidateAllUserRoles() {
	_ = r.cache.Clear()
}

func loadRolesForUser(ctx context.Context, client *ent.Client, userID uuid.UUID) ([]uuid.UUID, error) {
	var rows []struct {
		RoleID uuid.UUID `json:"role_id,omitempty"`
	}
	err := client.Role.Query().
		Where(
			entrole.ActiveEQ(true),
			entrole.HasUserRolesWith(
				entuserrole.HasUserWith(entuser.UserIDEQ(userID), entuser.DeletedAtIsNil()),
			),
		).
		Order(entrole.ByRoleID()).
		Select(entrole.FieldRoleID).
		Scan(ctx, &rows)
	if err != nil {
		return nil, fmt.Errorf("load user role policy subjects for user %s: %w", userID.String(), err)
	}
	roleIDs := make([]uuid.UUID, 0, len(rows))
	for _, row := range rows {
		roleIDs = append(roleIDs, row.RoleID)
	}
	return roleIDs, nil
}

func cloneRoleIDs(roleIDs []uuid.UUID) []uuid.UUID {
	if len(roleIDs) == 0 {
		return []uuid.UUID{}
	}
	cloned := make([]uuid.UUID, len(roleIDs))
	copy(cloned, roleIDs)
	return cloned
}

func roleSubject(roleID uuid.UUID) string {
	return "role:" + roleID.String()
}
