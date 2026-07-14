package main

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"

	roleseed "github.com/aegiscore/user-service/internal/features/role/application/seed"
)

func newRBACSeedRunner(newDependencies rbacSeedDependencyFactory) rbacSeedRunner {
	return func(ctx context.Context, configPath string, opts rbacSeedOptions) error {
		return runRBACSeedCommand(ctx, configPath, opts, newDependencies)
	}
}

func newRBACAssignSuperAdminRunner(newDependencies rbacSeedDependencyFactory) rbacAssignSuperAdminRunner {
	return func(ctx context.Context, configPath string, userID uuid.UUID) error {
		return runAssignSuperAdminCommand(ctx, configPath, userID, newDependencies)
	}
}

func newRBACCreateSuperAdminRunner(newDependencies rbacSeedDependencyFactory) rbacCreateSuperAdminRunner {
	return func(ctx context.Context, configPath string, opts rbacCreateSuperAdminOptions) error {
		return runCreateSuperAdminCommand(ctx, configPath, opts, newDependencies)
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

	result, err := deps.service.Seed(ctx, roleseed.SeedOptions{ReactivateSystem: opts.reactivateSystem, SyncSystemBindings: opts.syncSystemBindings})
	if err != nil {
		return err
	}
	fmt.Printf("RBAC seed complete: roles inserted=%d updated=%d permissions inserted=%d updated=%d bindings added=%d removed=%d\n", result.RolesInserted, result.RolesUpdated, result.PermissionsInserted, result.PermissionsUpdated, result.RolePermissionBindingsAdd, result.RolePermissionBindingsDel)
	return nil
}

func runAssignSuperAdminCommand(ctx context.Context, configPath string, userID uuid.UUID, newDependencies rbacSeedDependencyFactory) (err error) {
	deps, cleanup, err := newDependencies(ctx, configPath)
	if err != nil {
		return err
	}
	defer func() {
		err = errors.Join(err, cleanup())
	}()

	result, err := deps.service.AssignSuperAdmin(ctx, userID)
	if err != nil {
		return err
	}
	if result.Added {
		fmt.Printf("Super admin role assigned to user %s\n", userID.String())
	} else {
		fmt.Printf("Super admin role already assigned to user %s\n", userID.String())
	}
	return nil
}

func runCreateSuperAdminCommand(ctx context.Context, configPath string, opts rbacCreateSuperAdminOptions, newDependencies rbacSeedDependencyFactory) (err error) {
	deps, cleanup, err := newDependencies(ctx, configPath)
	if err != nil {
		return err
	}
	defer func() {
		err = errors.Join(err, cleanup())
	}()

	result, err := createSuperAdmin(ctx, deps, opts)
	if err != nil {
		return err
	}
	fmt.Printf("Super admin create complete: username=%s user_id=%s created=%t password_updated=%t super_admin_role_added=%t\n", normalizeUsername(opts.username), result.userID.String(), result.created, result.passwordUpdated, result.roleAdded)
	return nil
}
