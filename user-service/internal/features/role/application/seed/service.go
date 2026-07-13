package seed

import (
	"context"
	"fmt"

	"github.com/google/uuid"

	permissionapplication "github.com/aegiscore/user-service/internal/features/permission/application"
	permissionvalidators "github.com/aegiscore/user-service/internal/features/permission/application/validators"
	roleapplication "github.com/aegiscore/user-service/internal/features/role/application"
	rolevalidators "github.com/aegiscore/user-service/internal/features/role/application/validators"
	"github.com/aegiscore/user-service/internal/shared/rbacbaseline"
)

// Service 编排 RBAC 系统数据 seed 与超级管理员绑定。
type Service struct {
	permissions     permissionapplication.SeedPermissionStore
	roles           roleapplication.SeedRoleStore
	rolePermissions roleapplication.SeedRolePermissionStore
	userRoles       roleapplication.SeedUserRoleStore
}

// SeedOptions 控制系统 RBAC seed 的可选行为。
type SeedOptions struct {
	ReactivateSystem   bool
	SyncSystemBindings bool
}

// SeedResult 汇总本次 RBAC seed 的写入结果。
type SeedResult struct {
	RolesInserted             int
	RolesUpdated              int
	PermissionsInserted       int
	PermissionsUpdated        int
	RolePermissionBindingsAdd int
	RolePermissionBindingsDel int
}

// AssignSuperAdminResult 汇总超级管理员绑定结果。
type AssignSuperAdminResult struct {
	Added bool
}

// NewService 构造 RBAC seed 服务。
func NewService(roles roleapplication.SeedRoleStore, permissions permissionapplication.SeedPermissionStore, rolePermissions roleapplication.SeedRolePermissionStore, userRoles roleapplication.SeedUserRoleStore) *Service {
	return &Service{roles: roles, permissions: permissions, rolePermissions: rolePermissions, userRoles: userRoles}
}

// Seed 写入系统角色、系统权限和系统角色权限绑定。
func (s *Service) Seed(ctx context.Context, opts SeedOptions) (SeedResult, error) {
	roleInputs, err := buildRoleInputs(opts.ReactivateSystem)
	if err != nil {
		return SeedResult{}, err
	}
	permissionInputs, err := buildPermissionInputs(opts.ReactivateSystem)
	if err != nil {
		return SeedResult{}, err
	}
	bindings, err := buildRolePermissionInputs()
	if err != nil {
		return SeedResult{}, err
	}

	result := SeedResult{}
	for _, input := range roleInputs {
		_, inserted, err := s.roles.UpsertSystemRole(ctx, input)
		if err != nil {
			return result, err
		}
		if inserted {
			result.RolesInserted++
		} else {
			result.RolesUpdated++
		}
	}

	actualPermissions := make(map[uuid.UUID]uuid.UUID, len(permissionInputs))
	for _, input := range permissionInputs {
		permission, inserted, err := s.permissions.UpsertSystemPermission(ctx, input)
		if err != nil {
			return result, err
		}
		if inserted {
			result.PermissionsInserted++
		} else {
			result.PermissionsUpdated++
		}
		actualPermissions[input.PermissionID] = permission.PermissionID
	}

	for roleID, permissionIDs := range bindings {
		actualIDs := make([]uuid.UUID, 0, len(permissionIDs))
		for _, permissionID := range permissionIDs {
			actualID, ok := actualPermissions[permissionID]
			if !ok {
				return result, fmt.Errorf("catalog role permission references unknown permission_id %s", permissionID.String())
			}
			actualIDs = append(actualIDs, actualID)
		}
		if opts.SyncSystemBindings {
			added, removed, err := s.rolePermissions.SyncSystemBindings(ctx, roleID, actualIDs)
			if err != nil {
				return result, err
			}
			result.RolePermissionBindingsAdd += added
			result.RolePermissionBindingsDel += removed
			continue
		}
		added, err := s.rolePermissions.EnsureSystemBindings(ctx, roleID, actualIDs)
		if err != nil {
			return result, err
		}
		result.RolePermissionBindingsAdd += added
	}

	return result, nil
}

// AssignSuperAdmin 将指定用户绑定到内置超级管理员角色。
func (s *Service) AssignSuperAdmin(ctx context.Context, userID uuid.UUID) (AssignSuperAdminResult, error) {
	roleID, err := uuid.Parse(rbacbaseline.SuperAdminRoleID)
	if err != nil {
		return AssignSuperAdminResult{}, fmt.Errorf("parse super admin role id: %w", err)
	}
	added, err := s.userRoles.AssignRole(ctx, userID, roleID)
	if err != nil {
		return AssignSuperAdminResult{}, err
	}
	return AssignSuperAdminResult{Added: added}, nil
}

func buildRoleInputs(reactivate bool) ([]roleapplication.SeedRoleInput, error) {
	seen := make(map[uuid.UUID]struct{})
	roles := rbacbaseline.DefaultRoles()
	inputs := make([]roleapplication.SeedRoleInput, 0, len(roles))
	for _, spec := range roles {
		roleID, err := uuid.Parse(spec.RoleID)
		if err != nil {
			return nil, fmt.Errorf("parse role_id %q: %w", spec.RoleID, err)
		}
		if _, ok := seen[roleID]; ok {
			return nil, fmt.Errorf("duplicate role_id %s in role catalog", roleID.String())
		}
		seen[roleID] = struct{}{}
		name, description, err := rolevalidators.NormalizeRoleFields(spec.Name, spec.Description)
		if err != nil {
			return nil, err
		}
		inputs = append(inputs, roleapplication.SeedRoleInput{RoleID: roleID, Name: name, Description: description, Active: true, ReactivateSystem: reactivate})
	}
	return inputs, nil
}

func buildPermissionInputs(reactivate bool) ([]permissionapplication.SeedPermissionInput, error) {
	seenIDs := make(map[uuid.UUID]struct{})
	seenRoutes := make(map[string]struct{})
	permissions := rbacbaseline.DefaultPermissions()
	inputs := make([]permissionapplication.SeedPermissionInput, 0, len(permissions))
	for _, spec := range permissions {
		permissionID, err := uuid.Parse(spec.PermissionID)
		if err != nil {
			return nil, fmt.Errorf("parse permission_id %q: %w", spec.PermissionID, err)
		}
		if _, ok := seenIDs[permissionID]; ok {
			return nil, fmt.Errorf("duplicate permission_id %s in permission catalog", permissionID.String())
		}
		seenIDs[permissionID] = struct{}{}
		name, description, module, identity, err := permissionvalidators.NormalizePermissionFields(spec.Name, spec.Description, spec.Module, spec.Method, spec.PathTemplate)
		if err != nil {
			return nil, err
		}
		if _, ok := seenRoutes[identity.Key()]; ok {
			return nil, fmt.Errorf("duplicate route identity %s in permission catalog", identity.Key())
		}
		seenRoutes[identity.Key()] = struct{}{}
		inputs = append(inputs, permissionapplication.SeedPermissionInput{PermissionID: permissionID, Name: name, Description: description, Module: module, HTTPMethod: identity.Method, PathTemplate: identity.PathTemplate, Active: true, ReactivateSystem: reactivate})
	}
	return inputs, nil
}

func buildRolePermissionInputs() (map[uuid.UUID][]uuid.UUID, error) {
	roles := make(map[uuid.UUID]struct{})
	for _, spec := range rbacbaseline.DefaultRoles() {
		roleID, err := uuid.Parse(spec.RoleID)
		if err != nil {
			return nil, fmt.Errorf("parse role_id %q: %w", spec.RoleID, err)
		}
		roles[roleID] = struct{}{}
	}
	permissions := make(map[uuid.UUID]struct{})
	for _, spec := range rbacbaseline.DefaultPermissions() {
		permissionID, err := uuid.Parse(spec.PermissionID)
		if err != nil {
			return nil, fmt.Errorf("parse permission_id %q: %w", spec.PermissionID, err)
		}
		permissions[permissionID] = struct{}{}
	}

	seen := make(map[string]struct{})
	bindings := make(map[uuid.UUID][]uuid.UUID)
	for _, spec := range rbacbaseline.DefaultRolePermissions() {
		roleID, err := uuid.Parse(spec.RoleID)
		if err != nil {
			return nil, fmt.Errorf("parse role permission role_id %q: %w", spec.RoleID, err)
		}
		if _, ok := roles[roleID]; !ok {
			return nil, fmt.Errorf("role permission references unknown role_id %s", roleID.String())
		}
		permissionID, err := uuid.Parse(spec.PermissionID)
		if err != nil {
			return nil, fmt.Errorf("parse role permission permission_id %q: %w", spec.PermissionID, err)
		}
		if _, ok := permissions[permissionID]; !ok {
			return nil, fmt.Errorf("role permission references unknown permission_id %s", permissionID.String())
		}
		key := roleID.String() + ":" + permissionID.String()
		if _, ok := seen[key]; ok {
			return nil, fmt.Errorf("duplicate role permission binding %s", key)
		}
		seen[key] = struct{}{}
		bindings[roleID] = append(bindings[roleID], permissionID)
	}
	return bindings, nil
}
