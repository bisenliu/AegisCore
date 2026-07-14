package main

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	authdomain "github.com/aegiscore/user-service/internal/features/auth/domain"
	roleseed "github.com/aegiscore/user-service/internal/features/role/application/seed"
	"github.com/aegiscore/user-service/internal/shared/identity"
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
					}, nil),
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
					}, nil),
				}, func() error {
					return cleanupErr
				}, nil
		})

		err := runners.seedRunner(context.Background(), "test-config.yaml", rbacSeedOptions{})

		require.ErrorIs(t, err, seedErr)
		require.ErrorIs(t, err, cleanupErr)
	})
}

func TestRunAssignSuperAdminCommand(t *testing.T) {
	userID := uuid.MustParse("018f0000-0000-7000-8000-000000000101")

	for _, tc := range []struct {
		name       string
		added      bool
		wantOutput string
	}{
		{name: "new binding", added: true, wantOutput: "Super admin role assigned to user " + userID.String()},
		{name: "existing binding", added: false, wantOutput: "Super admin role already assigned to user " + userID.String()},
	} {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			assignCalled := false
			cleanupCalled := false
			runners := rbacCommandRunnersWithFactory(func(_ context.Context, configPath string) (rbacSeedDependencies, func() error, error) {
				require.Equal(t, "test-config.yaml", configPath)
				return rbacSeedDependencies{
						service: newRBACSeedServiceMock(t, nil, func(_ context.Context, got uuid.UUID) (roleseed.AssignSuperAdminResult, error) {
							assignCalled = true
							require.Equal(t, userID, got)
							return roleseed.AssignSuperAdminResult{Added: tc.added}, nil
						}),
					}, func() error {
						cleanupCalled = true
						return nil
					}, nil
			})

			out, err := captureStdout(t, func() error {
				return runners.assignSuperAdminRunner(context.Background(), "test-config.yaml", userID)
			})

			require.NoError(t, err)
			require.True(t, assignCalled)
			require.True(t, cleanupCalled)
			require.Contains(t, out, tc.wantOutput)
		})
	}

	t.Run("dependency error", func(t *testing.T) {
		initErr := errors.New("postgres config missing")
		runners := rbacCommandRunnersWithFactory(func(context.Context, string) (rbacSeedDependencies, func() error, error) {
			return rbacSeedDependencies{}, nil, initErr
		})
		err := runners.assignSuperAdminRunner(context.Background(), "bad.yaml", userID)
		require.ErrorIs(t, err, initErr)
	})

	t.Run("service error still cleans up", func(t *testing.T) {
		assignErr := errors.New("assign failed")
		cleanupCalled := false
		runners := rbacCommandRunnersWithFactory(func(context.Context, string) (rbacSeedDependencies, func() error, error) {
			return rbacSeedDependencies{
					service: newRBACSeedServiceMock(t, nil, func(context.Context, uuid.UUID) (roleseed.AssignSuperAdminResult, error) {
						return roleseed.AssignSuperAdminResult{}, assignErr
					}),
				}, func() error {
					cleanupCalled = true
					return nil
				}, nil
		})

		err := runners.assignSuperAdminRunner(context.Background(), "test-config.yaml", userID)

		require.ErrorIs(t, err, assignErr)
		require.True(t, cleanupCalled)
	})
}

func TestRunCreateSuperAdminCommand(t *testing.T) {
	userID := uuid.MustParse("018f0000-0000-7000-8000-000000000201")

	t.Run("success prints normalized username", func(t *testing.T) {
		t.Setenv("ADMIN_SECRET", " secret ")
		cleanupCalled := false
		runners := rbacCommandRunnersWithFactory(func(_ context.Context, configPath string) (rbacSeedDependencies, func() error, error) {
			require.Equal(t, "test-config.yaml", configPath)
			return rbacSeedDependencies{
					service: newRBACSeedServiceMock(t, nil, func(_ context.Context, got uuid.UUID) (roleseed.AssignSuperAdminResult, error) {
						require.Equal(t, userID, got)
						return roleseed.AssignSuperAdminResult{Added: true}, nil
					}),
					credentials: newRBACCredentialStoreMock(t, func(_ context.Context, username string) (*authdomain.UserCredential, error) {
						require.Equal(t, "admin", username)
						return &authdomain.UserCredential{UserID: userID, Username: username, Status: identity.UserStatusNormal}, nil
					}, nil),
				}, func() error {
					cleanupCalled = true
					return nil
				}, nil
		})

		out, err := captureStdout(t, func() error {
			return runners.createSuperAdminRunner(context.Background(), "test-config.yaml", rbacCreateSuperAdminOptions{username: " ADMIN ", passwordEnv: "ADMIN_SECRET"})
		})

		require.NoError(t, err)
		require.True(t, cleanupCalled)
		require.Contains(t, out, "Super admin create complete: username=admin user_id="+userID.String()+" created=false password_updated=false super_admin_role_added=true")
	})

	t.Run("dependency error", func(t *testing.T) {
		initErr := errors.New("init failed")
		runners := rbacCommandRunnersWithFactory(func(context.Context, string) (rbacSeedDependencies, func() error, error) {
			return rbacSeedDependencies{}, nil, initErr
		})
		err := runners.createSuperAdminRunner(context.Background(), "bad.yaml", rbacCreateSuperAdminOptions{})
		require.ErrorIs(t, err, initErr)
	})

	t.Run("create and cleanup errors are joined", func(t *testing.T) {
		t.Setenv("ADMIN_SECRET", "secret")
		getErr := errors.New("credential read failed")
		cleanupErr := errors.New("cleanup failed")
		runners := rbacCommandRunnersWithFactory(func(context.Context, string) (rbacSeedDependencies, func() error, error) {
			return rbacSeedDependencies{
					credentials: newRBACCredentialStoreMock(t, func(context.Context, string) (*authdomain.UserCredential, error) {
						return nil, getErr
					}, nil),
				}, func() error {
					return cleanupErr
				}, nil
		})

		err := runners.createSuperAdminRunner(context.Background(), "test-config.yaml", rbacCreateSuperAdminOptions{username: "admin", passwordEnv: "ADMIN_SECRET"})

		require.ErrorIs(t, err, getErr)
		require.ErrorIs(t, err, cleanupErr)
	})
}
