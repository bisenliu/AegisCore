package main

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	rolebootstrap "github.com/aegiscore/user-service/internal/features/role/application/bootstrap"
	roleseed "github.com/aegiscore/user-service/internal/features/role/application/seed"
	"github.com/aegiscore/user-service/internal/shared/rbacbaseline"
)

func TestRunRBACSeedCommand(t *testing.T) {
	t.Run("success passes options and cleans up", func(t *testing.T) {
		seedCalled := false
		cleanupCalled := false
		runners := rbacCommandRunnersWithFactory(func(_ context.Context, configPath string) (rbacSeedDependencies, func() error, error) {
			require.Equal(t, "test-config.yaml", configPath)
			return rbacSeedDependencies{
					service: newRBACSeedServiceMock(t, func(_ context.Context, opts roleseed.SeedOptions) (roleseed.SeedResult, error) {
						seedCalled = true
						require.True(t, opts.ReactivateSystem)
						require.True(t, opts.SyncSystemBindings)
						return roleseed.SeedResult{
							RolesInserted:             1,
							RolesUpdated:              2,
							PermissionsInserted:       3,
							PermissionsUpdated:        4,
							RolePermissionBindingsAdd: 5,
							RolePermissionBindingsDel: 6,
						}, nil
					}),
				}, func() error {
					cleanupCalled = true
					return nil
				}, nil
		})

		out, err := captureStdout(t, func() error {
			return runners.seedRunner(context.Background(), "test-config.yaml", rbacSeedOptions{reactivateSystem: true, syncSystemBindings: true})
		})

		require.NoError(t, err)
		require.True(t, seedCalled)
		require.True(t, cleanupCalled)
		require.Contains(t, out, "RBAC seed complete: roles inserted=1 updated=2 permissions inserted=3 updated=4 bindings added=5 removed=6")
	})

	t.Run("dependency error returns before cleanup", func(t *testing.T) {
		initErr := errors.New("load config failed")
		runners := rbacCommandRunnersWithFactory(func(context.Context, string) (rbacSeedDependencies, func() error, error) {
			return rbacSeedDependencies{}, nil, initErr
		})

		err := runners.seedRunner(context.Background(), "bad.yaml", rbacSeedOptions{})

		require.ErrorIs(t, err, initErr)
	})

	t.Run("seed and cleanup errors are joined", func(t *testing.T) {
		seedErr := errors.New("seed failed")
		cleanupErr := errors.New("cleanup failed")
		runners := rbacCommandRunnersWithFactory(func(context.Context, string) (rbacSeedDependencies, func() error, error) {
			return rbacSeedDependencies{
					service: newRBACSeedServiceMock(t, func(context.Context, roleseed.SeedOptions) (roleseed.SeedResult, error) {
						return roleseed.SeedResult{}, seedErr
					}),
				}, func() error {
					return cleanupErr
				}, nil
		})

		err := runners.seedRunner(context.Background(), "test-config.yaml", rbacSeedOptions{})

		require.ErrorIs(t, err, seedErr)
		require.ErrorIs(t, err, cleanupErr)
	})
}

func TestRunBootstrapSuperAdminCommand(t *testing.T) {
	t.Run("success prints normalized identifiers", func(t *testing.T) {
		cleanupCalled := false
		store := &testBootstrapStore{}
		passwordEnv := uniquePasswordEnv(t)
		t.Setenv(passwordEnv, " long secret ")
		runners := rbacCommandRunnersWithFactory(func(_ context.Context, configPath string) (rbacSeedDependencies, func() error, error) {
			require.Equal(t, "test-config.yaml", configPath)
			return rbacSeedDependencies{
					bootstrap: rolebootstrap.NewService(store, &testBootstrapHasher{}),
				}, func() error {
					cleanupCalled = true
					return nil
				}, nil
		})

		out, err := captureStdout(t, func() error {
			return runners.bootstrapSuperAdminRunner(context.Background(), "test-config.yaml", rbacBootstrapSuperAdminOptions{username: " ADMIN ", passwordEnv: passwordEnv})
		})

		require.NoError(t, err)
		require.True(t, cleanupCalled)
		require.Equal(t, "admin", store.input.Username)
		require.Contains(t, out, "Super admin bootstrap complete: username=admin user_id="+rbacbaseline.BootstrapSuperAdminUserID)
	})

	t.Run("dependency error", func(t *testing.T) {
		initErr := errors.New("init failed")
		runners := rbacCommandRunnersWithFactory(func(context.Context, string) (rbacSeedDependencies, func() error, error) {
			return rbacSeedDependencies{}, nil, initErr
		})
		err := runners.bootstrapSuperAdminRunner(context.Background(), "bad.yaml", rbacBootstrapSuperAdminOptions{})
		require.ErrorIs(t, err, initErr)
	})

	t.Run("bootstrap and cleanup errors are joined", func(t *testing.T) {
		bootstrapErr := errors.New("bootstrap failed")
		cleanupErr := errors.New("cleanup failed")
		passwordEnv := uniquePasswordEnv(t)
		t.Setenv(passwordEnv, "long-password")
		runners := rbacCommandRunnersWithFactory(func(context.Context, string) (rbacSeedDependencies, func() error, error) {
			return rbacSeedDependencies{
					bootstrap: rolebootstrap.NewService(&testBootstrapStore{err: bootstrapErr}, &testBootstrapHasher{}),
				}, func() error {
					return cleanupErr
				}, nil
		})

		err := runners.bootstrapSuperAdminRunner(context.Background(), "test-config.yaml", rbacBootstrapSuperAdminOptions{username: "admin", passwordEnv: passwordEnv})

		require.ErrorIs(t, err, bootstrapErr)
		require.ErrorIs(t, err, cleanupErr)
	})

	t.Run("missing password environment variable", func(t *testing.T) {
		passwordEnv := uniquePasswordEnv(t)
		runners := rbacCommandRunnersWithFactory(func(context.Context, string) (rbacSeedDependencies, func() error, error) {
			return rbacSeedDependencies{
					bootstrap: rolebootstrap.NewService(&testBootstrapStore{}, &testBootstrapHasher{}),
				}, func() error {
					return nil
				}, nil
		})

		err := runners.bootstrapSuperAdminRunner(context.Background(), "test-config.yaml", rbacBootstrapSuperAdminOptions{username: "admin", passwordEnv: passwordEnv})

		require.ErrorContains(t, err, "bootstrap password environment variable "+passwordEnv+" is required")
	})
}

func uniquePasswordEnv(t *testing.T) string {
	t.Helper()
	return fmt.Sprintf("ADMIN_BOOTSTRAP_PASSWORD_%s", strings.NewReplacer("/", "_", "-", "_", " ", "_").Replace(t.Name()))
}
