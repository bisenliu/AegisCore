package casbin

import (
	"context"
	"fmt"
	"strconv"
	"sync"
	"time"

	"github.com/google/uuid"
	"go.uber.org/fx"
	"golang.org/x/sync/singleflight"

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

const defaultUserRoleCacheTTL = 5 * time.Second

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

type entUserRoleResolver struct {
	cache   *localcache.Cache[uuid.UUID, []uuid.UUID]
	client  *ent.Client
	group   singleflight.Group
	mu      sync.RWMutex
	ttl     time.Duration
	version uint64
}

// NewUserRoleResolver 构造按用户短 TTL 缓存的角色解析器。
func NewUserRoleResolver(params LoaderParams) UserRoleResolver {
	return newUserRoleResolver(params.Client, defaultUserRoleCacheTTL)
}

func newUserRoleResolver(client *ent.Client, ttl time.Duration) *entUserRoleResolver {
	if ttl <= 0 {
		ttl = defaultUserRoleCacheTTL
	}
	return &entUserRoleResolver{client: client, ttl: ttl, cache: localcache.New[uuid.UUID, []uuid.UUID](ttl)}
}

// RolesForUser 返回用户当前绑定的启用角色 ID。
func (r *entUserRoleResolver) RolesForUser(ctx context.Context, userID uuid.UUID) ([]uuid.UUID, error) {
	cache, version := r.currentCache()
	if roleIDs, ok := cache.Get(userID); ok {
		return cloneRoleIDs(roleIDs), nil
	}
	value, err, _ := r.group.Do(userRoleCacheFlightKey(userID, version), func() (any, error) {
		cache, version := r.currentCache()
		if roleIDs, ok := cache.Get(userID); ok {
			return cloneRoleIDs(roleIDs), nil
		}
		roleIDs, err := r.loadRolesForUser(ctx, userID)
		if err != nil {
			return nil, err
		}
		r.setCacheIfCurrent(userID, roleIDs, version)
		return cloneRoleIDs(roleIDs), nil
	})
	if err != nil {
		return nil, err
	}
	return cloneRoleIDs(value.([]uuid.UUID)), nil
}

// InvalidateUserRole 删除单个用户的本地角色缓存。
func (r *entUserRoleResolver) InvalidateUserRole(userID uuid.UUID) {
	r.mu.Lock()
	version := r.version
	r.cache.Delete(userID)
	r.version++
	r.mu.Unlock()
	r.group.Forget(userRoleCacheFlightKey(userID, version))
}

// InvalidateAllUserRoles 清空本实例全部用户角色缓存。
func (r *entUserRoleResolver) InvalidateAllUserRoles() {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.cache = localcache.New[uuid.UUID, []uuid.UUID](r.ttl)
	r.version++
}

func (r *entUserRoleResolver) currentCache() (*localcache.Cache[uuid.UUID, []uuid.UUID], uint64) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.cache, r.version
}

func (r *entUserRoleResolver) setCacheIfCurrent(userID uuid.UUID, roleIDs []uuid.UUID, version uint64) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	if r.version != version {
		return
	}
	r.cache.Set(userID, cloneRoleIDs(roleIDs))
}

func (r *entUserRoleResolver) loadRolesForUser(ctx context.Context, userID uuid.UUID) ([]uuid.UUID, error) {
	var rows []struct {
		RoleID uuid.UUID `json:"role_id,omitempty"`
	}
	err := r.client.Role.Query().
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

func userRoleCacheFlightKey(userID uuid.UUID, version uint64) string {
	return userID.String() + ":" + strconv.FormatUint(version, 10)
}

func roleSubject(roleID uuid.UUID) string {
	return "role:" + roleID.String()
}
