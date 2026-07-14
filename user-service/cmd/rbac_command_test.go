package main

import (
	"context"
	"testing"

	"github.com/google/uuid"
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

func TestAssignSuperAdminCommandValidatesUserID(t *testing.T) {
	called := false
	deps := testRootCommandDependencies(t)
	deps.assignSuperAdminRunner = func(_ context.Context, _ string, _ uuid.UUID) error {
		called = true
		return nil
	}

	root := newRootCommand(deps)
	root.SetArgs([]string{"rbac", "assign-super-admin", "--user-id", "not-a-uuid"})
	require.ErrorContains(t, root.Execute(), "invalid")
	require.False(t, called)
}

func TestAssignSuperAdminCommandRuns(t *testing.T) {
	userID := uuid.MustParse("018f0000-0000-7000-8000-000000000001")
	called := false
	deps := testRootCommandDependencies(t)
	deps.assignSuperAdminRunner = func(_ context.Context, configPath string, got uuid.UUID) error {
		called = true
		require.Equal(t, "test-config.yaml", configPath)
		require.Equal(t, userID, got)
		return nil
	}

	root := newRootCommand(deps)
	root.SetArgs([]string{"rbac", "--config", "test-config.yaml", "assign-super-admin", "--user-id", userID.String()})
	require.NoError(t, root.Execute())
	require.True(t, called)
}

func TestCreateSuperAdminCommandRunsWithDefaults(t *testing.T) {
	called := false
	deps := testRootCommandDependencies(t)
	deps.createSuperAdminRunner = func(_ context.Context, configPath string, opts rbacCreateSuperAdminOptions) error {
		called = true
		require.Equal(t, "test-config.yaml", configPath)
		assert.Equal(t, defaultCreateSuperAdminUsername, opts.username)
		assert.Equal(t, defaultCreateSuperAdminNickname, opts.nickname)
		assert.Equal(t, defaultCreateSuperAdminPasswordEnv, opts.passwordEnv)
		assert.False(t, opts.resetPassword)
		return nil
	}

	root := newRootCommand(deps)
	root.SetArgs([]string{"rbac", "--config", "test-config.yaml", "create-super-admin"})
	require.NoError(t, root.Execute())
	require.True(t, called)
}

func TestCreateSuperAdminCommandRunsWithFlags(t *testing.T) {
	called := false
	deps := testRootCommandDependencies(t)
	deps.createSuperAdminRunner = func(_ context.Context, configPath string, opts rbacCreateSuperAdminOptions) error {
		called = true
		require.Equal(t, "test-config.yaml", configPath)
		assert.Equal(t, "root", opts.username)
		assert.Equal(t, "Root", opts.nickname)
		assert.Equal(t, "ADMIN_SECRET", opts.passwordEnv)
		assert.True(t, opts.resetPassword)
		return nil
	}

	root := newRootCommand(deps)
	root.SetArgs([]string{"rbac", "--config", "test-config.yaml", "create-super-admin", "--username", "root", "--nickname", "Root", "--password-env", "ADMIN_SECRET", "--reset-password"})
	require.NoError(t, root.Execute())
	require.True(t, called)
}
