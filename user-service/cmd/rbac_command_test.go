package main

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRBACSeedCommandFlags(t *testing.T) {
	called := false
	deps := testRootCommandDependencies(t)
	deps.seedRunner = func(_ context.Context, configPath string, opts rbacSeedOptions) error {
		called = true
		require.Equal(t, "test-config.yaml", configPath)
		assert.True(t, opts.reactivateSystem)
		assert.True(t, opts.syncSystemBindings)
		return nil
	}

	root := newRootCommand(deps)
	root.SetArgs([]string{"rbac", "--config", "test-config.yaml", "seed", "--reactivate-system", "--sync-system-bindings"})
	require.NoError(t, root.Execute())
	require.True(t, called)
}

func TestRBACDeletedSuperAdminCommandsAreUnavailable(t *testing.T) {
	for _, args := range [][]string{
		{"rbac", "assign-super-admin", "--user-id", "not-a-uuid"},
		{"rbac", "create-super-admin"},
		{"rbac", "bootstrap-super-admin", "--username", "root", "--reset-password"},
	} {
		root := newRootCommand(testRootCommandDependencies(t))
		root.SetArgs(args)
		require.Error(t, root.Execute())
	}
}

func TestBootstrapSuperAdminCommandRequiresUsername(t *testing.T) {
	called := false
	deps := testRootCommandDependencies(t)
	deps.bootstrapSuperAdminRunner = func(_ context.Context, _ string, _ rbacBootstrapSuperAdminOptions) error {
		called = true
		return nil
	}

	root := newRootCommand(deps)
	root.SetArgs([]string{"rbac", "bootstrap-super-admin"})
	require.ErrorContains(t, root.Execute(), "required flag")
	require.False(t, called)
}

func TestBootstrapSuperAdminCommandRuns(t *testing.T) {
	called := false
	deps := testRootCommandDependencies(t)
	deps.bootstrapSuperAdminRunner = func(_ context.Context, configPath string, opts rbacBootstrapSuperAdminOptions) error {
		called = true
		require.Equal(t, "test-config.yaml", configPath)
		assert.Equal(t, "root", opts.username)
		assert.Equal(t, "Root", opts.nickname)
		assert.Equal(t, "CUSTOM_ADMIN_PASSWORD", opts.passwordEnv)
		return nil
	}

	root := newRootCommand(deps)
	root.SetArgs([]string{"rbac", "--config", "test-config.yaml", "bootstrap-super-admin", "--username", "root", "--nickname", "Root", "--password-env", "CUSTOM_ADMIN_PASSWORD"})
	require.NoError(t, root.Execute())
	require.True(t, called)
}
