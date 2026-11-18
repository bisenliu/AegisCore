package main

import (
	"context"
	"errors"
	"fmt"

	roleseed "github.com/aegiscore/user-service/internal/features/role/application/seed"
)

// newRBACSeedRunner 创建写入系统内置角色与权限清单的操作执行器。
func newRBACSeedRunner(newDependencies rbacSeedDependencyFactory) rbacSeedRunner {
	return func(ctx context.Context, opts rbacSeedOptions) error {
		return runRBACSeedCommand(ctx, opts, newDependencies)
	}
}

// newRBACBootstrapSuperAdminRunner 创建初始化首个超级管理员账号的操作执行器。
func newRBACBootstrapSuperAdminRunner(newDependencies rbacSeedDependencyFactory) rbacBootstrapSuperAdminRunner {
	return func(ctx context.Context, opts rbacBootstrapSuperAdminOptions) error {
		return runBootstrapSuperAdminCommand(ctx, opts, newDependencies)
	}
}

// runRBACSeedCommand 将系统内置的标准角色、功能权限及其默认授权关系保存到数据库中。
func runRBACSeedCommand(ctx context.Context, opts rbacSeedOptions, newDependencies rbacSeedDependencyFactory) (err error) {
	deps, cleanup, err := newDependencies(ctx)
	if err != nil {
		return err
	}
	defer func() {
		// 清理错误与业务错误同时返回，避免连接关闭失败掩盖 seed 结果或被静默丢弃。
		err = errors.Join(err, cleanup())
	}()
	ctx = contextWithRBACLogger(ctx, deps)

	result, err := deps.service.Seed(ctx, roleseed.SeedOptions{ReactivateSystem: opts.reactivateSystem, SyncSystemBindings: opts.syncSystemBindings})
	if err != nil {
		return err
	}
	fmt.Printf("RBAC seed complete: roles inserted=%d updated=%d permissions inserted=%d updated=%d bindings added=%d removed=%d\n", result.RolesInserted, result.RolesUpdated, result.PermissionsInserted, result.PermissionsUpdated, result.RolePermissionBindingsAdd, result.RolePermissionBindingsDel)
	return nil
}

// runBootstrapSuperAdminCommand 在数据库中创建首位超级管理员账号并绑定管理角色；若账号已存在则拒绝覆盖。
func runBootstrapSuperAdminCommand(ctx context.Context, opts rbacBootstrapSuperAdminOptions, newDependencies rbacSeedDependencyFactory) (err error) {
	deps, cleanup, err := newDependencies(ctx)
	if err != nil {
		return err
	}
	defer func() {
		// bootstrap 已失败时仍保留资源清理错误，便于诊断 CLI 进程退出阶段的问题。
		err = errors.Join(err, cleanup())
	}()

	result, err := bootstrapSuperAdmin(ctx, deps, opts)
	if err != nil {
		return err
	}
	fmt.Printf("Super admin bootstrap complete: username=%s user_id=%s super_admin_role_id=%s\n", result.Username, result.UserID.String(), result.RoleID.String())
	return nil
}
