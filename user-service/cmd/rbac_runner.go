package main

import (
	"context"
	"errors"
	"fmt"

	roleseed "github.com/aegiscore/user-service/internal/features/role/application/seed"
)

func newRBACSeedRunner(newDependencies rbacSeedDependencyFactory) rbacSeedRunner {
	return func(ctx context.Context, configPath string, opts rbacSeedOptions) error {
		return runRBACSeedCommand(ctx, configPath, opts, newDependencies)
	}
}

func newRBACBootstrapSuperAdminRunner(newDependencies rbacSeedDependencyFactory) rbacBootstrapSuperAdminRunner {
	return func(ctx context.Context, configPath string, opts rbacBootstrapSuperAdminOptions) error {
		return runBootstrapSuperAdminCommand(ctx, configPath, opts, newDependencies)
	}
}

func runRBACSeedCommand(ctx context.Context, configPath string, opts rbacSeedOptions, newDependencies rbacSeedDependencyFactory) (err error) {
	deps, cleanup, err := newDependencies(ctx, configPath)
	if err != nil {
		return err
	}
	defer func() {
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

func runBootstrapSuperAdminCommand(ctx context.Context, configPath string, opts rbacBootstrapSuperAdminOptions, newDependencies rbacSeedDependencyFactory) (err error) {
	deps, cleanup, err := newDependencies(ctx, configPath)
	if err != nil {
		return err
	}
	defer func() {
		err = errors.Join(err, cleanup())
	}()

	result, err := bootstrapSuperAdmin(ctx, deps, opts)
	if err != nil {
		return err
	}
	fmt.Printf("Super admin bootstrap complete: username=%s user_id=%s super_admin_role_id=%s\n", result.Username, result.UserID.String(), result.RoleID.String())
	return nil
}
