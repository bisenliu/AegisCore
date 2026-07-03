package main

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/fx"

	"github.com/aegiscore/common/runtime/config"
)

type testContextKey string

type testLifecycleApp struct {
	start func(context.Context) error
	stop  func(context.Context) error
}

func (a testLifecycleApp) Start(ctx context.Context) error {
	return a.start(ctx)
}

func (a testLifecycleApp) Stop(ctx context.Context) error {
	return a.stop(ctx)
}

func TestRunServeStopContextPreservesUpstreamValuesWithoutCancellation(t *testing.T) {
	originalFactory := newLifecycleApp
	originalSeed := runRBACSeed
	originalCreateSuperAdmin := runCreateSuperAdmin
	seedCalled := false
	createSuperAdminCalled := false
	t.Cleanup(func() {
		newLifecycleApp = originalFactory
		runRBACSeed = originalSeed
		runCreateSuperAdmin = originalCreateSuperAdmin
	})
	runRBACSeed = func(context.Context, string, rbacSeedOptions) error {
		seedCalled = true
		return nil
	}
	runCreateSuperAdmin = func(context.Context, string, rbacCreateSuperAdminOptions) error {
		createSuperAdminCalled = true
		return nil
	}

	key := testContextKey("trace-id")
	parent := context.WithValue(context.Background(), key, "test-trace")
	ctx, cancel := context.WithCancel(parent)

	newLifecycleApp = func(configPath string) lifecycleApp {
		require.Equal(t, "test-config.yaml", configPath)

		return testLifecycleApp{
			start: func(_ context.Context) error {
				cancel()
				return nil
			},
			stop: func(ctx context.Context) error {
				require.Equal(t, "test-trace", ctx.Value(key))
				require.NoError(t, ctx.Err())
				deadline, ok := ctx.Deadline()
				require.True(t, ok)
				remaining := time.Until(deadline)
				require.Greater(t, remaining, time.Duration(0))
				require.LessOrEqual(t, remaining, fxAppStopTimeout)
				return nil
			},
		}
	}

	require.NoError(t, runServe(ctx, "test-config.yaml"))
	require.False(t, seedCalled)
	require.False(t, createSuperAdminCalled)
}

func TestRootCommandSurface(t *testing.T) {
	root := newRootCommand()
	require.Equal(t, "aegiscore-user-services", root.Use)

	var serve *cobra.Command
	var rbac *cobra.Command
	var fxGraph *cobra.Command
	for _, cmd := range root.Commands() {
		switch cmd.Use {
		case "serve":
			serve = cmd
		case "rbac":
			rbac = cmd
		case "fxgraph":
			fxGraph = cmd
		}
	}
	require.NotNil(t, serve)

	flag := serve.Flags().Lookup("config")
	require.NotNil(t, flag)
	assert.Equal(t, "./configs/config.yaml", flag.DefValue)
	require.NotNil(t, rbac)
	flag = rbac.PersistentFlags().Lookup("config")
	require.NotNil(t, flag)
	assert.Equal(t, "./configs/config.yaml", flag.DefValue)
	assert.NotNil(t, findSubcommand(rbac, "seed"))
	assert.NotNil(t, findSubcommand(rbac, "assign-super-admin"))
	assert.NotNil(t, findSubcommand(rbac, "create-super-admin"))
	require.NotNil(t, fxGraph)
	flag = fxGraph.Flags().Lookup("config")
	require.NotNil(t, flag)
	assert.Equal(t, "./configs/config.yaml", flag.DefValue)
	flag = fxGraph.Flags().Lookup("output")
	require.NotNil(t, flag)
	assert.Equal(t, defaultFxGraphOutputPath, flag.DefValue)
}

func TestFxGraphCommandWritesGraph(t *testing.T) {
	originalWrite := writeFxGraph
	t.Cleanup(func() { writeFxGraph = originalWrite })
	called := false
	writeFxGraph = func(path string, opts ...fx.Option) (string, error) {
		called = true
		require.Equal(t, "docs/test.dot", path)
		require.NotEmpty(t, opts)
		return "digraph {}\n", nil
	}

	root := newRootCommand()
	root.SetArgs([]string{"fxgraph", "--config", "test-config.yaml", "--output", "docs/test.dot"})
	require.NoError(t, root.Execute())
	require.True(t, called)
}

func TestRBACSeedCommandFlags(t *testing.T) {
	originalSeed := runRBACSeed
	t.Cleanup(func() { runRBACSeed = originalSeed })
	called := false
	runRBACSeed = func(_ context.Context, configPath string, opts rbacSeedOptions) error {
		called = true
		require.Equal(t, "test-config.yaml", configPath)
		assert.True(t, opts.reactivateSystem)
		assert.True(t, opts.syncSystemBindings)
		return nil
	}

	root := newRootCommand()
	root.SetArgs([]string{"rbac", "--config", "test-config.yaml", "seed", "--reactivate-system", "--sync-system-bindings"})
	require.NoError(t, root.Execute())
	require.True(t, called)
}

func TestAssignSuperAdminCommandValidatesUserID(t *testing.T) {
	originalAssign := runAssignSuperAdmin
	t.Cleanup(func() { runAssignSuperAdmin = originalAssign })
	called := false
	runAssignSuperAdmin = func(_ context.Context, _ string, _ uuid.UUID) error {
		called = true
		return nil
	}

	root := newRootCommand()
	root.SetArgs([]string{"rbac", "assign-super-admin", "--user-id", "not-a-uuid"})
	require.ErrorContains(t, root.Execute(), "invalid")
	require.False(t, called)
}

func TestAssignSuperAdminCommandRuns(t *testing.T) {
	originalAssign := runAssignSuperAdmin
	t.Cleanup(func() { runAssignSuperAdmin = originalAssign })
	userID := uuid.MustParse("018f0000-0000-7000-8000-000000000001")
	called := false
	runAssignSuperAdmin = func(_ context.Context, configPath string, got uuid.UUID) error {
		called = true
		require.Equal(t, "test-config.yaml", configPath)
		require.Equal(t, userID, got)
		return nil
	}

	root := newRootCommand()
	root.SetArgs([]string{"rbac", "--config", "test-config.yaml", "assign-super-admin", "--user-id", userID.String()})
	require.NoError(t, root.Execute())
	require.True(t, called)
}

func TestCreateSuperAdminCommandRunsWithDefaults(t *testing.T) {
	originalCreateSuperAdmin := runCreateSuperAdmin
	t.Cleanup(func() { runCreateSuperAdmin = originalCreateSuperAdmin })
	called := false
	runCreateSuperAdmin = func(_ context.Context, configPath string, opts rbacCreateSuperAdminOptions) error {
		called = true
		require.Equal(t, "test-config.yaml", configPath)
		assert.Equal(t, defaultCreateSuperAdminUsername, opts.username)
		assert.Equal(t, defaultCreateSuperAdminNickname, opts.nickname)
		assert.Equal(t, defaultCreateSuperAdminPasswordEnv, opts.passwordEnv)
		assert.False(t, opts.resetPassword)
		return nil
	}

	root := newRootCommand()
	root.SetArgs([]string{"rbac", "--config", "test-config.yaml", "create-super-admin"})
	require.NoError(t, root.Execute())
	require.True(t, called)
}

func TestCreateSuperAdminCommandRunsWithFlags(t *testing.T) {
	originalCreateSuperAdmin := runCreateSuperAdmin
	t.Cleanup(func() { runCreateSuperAdmin = originalCreateSuperAdmin })
	called := false
	runCreateSuperAdmin = func(_ context.Context, configPath string, opts rbacCreateSuperAdminOptions) error {
		called = true
		require.Equal(t, "test-config.yaml", configPath)
		assert.Equal(t, "root", opts.username)
		assert.Equal(t, "Root", opts.nickname)
		assert.Equal(t, "ADMIN_SECRET", opts.passwordEnv)
		assert.True(t, opts.resetPassword)
		return nil
	}

	root := newRootCommand()
	root.SetArgs([]string{"rbac", "--config", "test-config.yaml", "create-super-admin", "--username", "root", "--nickname", "Root", "--password-env", "ADMIN_SECRET", "--reset-password"})
	require.NoError(t, root.Execute())
	require.True(t, called)
}

func TestNormalizeCreateSuperAdminOptionsRequiresPasswordEnv(t *testing.T) {
	t.Setenv(defaultCreateSuperAdminPasswordEnv, "")

	_, err := normalizeCreateSuperAdminOptions(rbacCreateSuperAdminOptions{username: "admin", nickname: "Admin", passwordEnv: "MISSING_ADMIN_PASSWORD"})
	require.ErrorContains(t, err, "MISSING_ADMIN_PASSWORD")
}

func TestNormalizeCreateSuperAdminOptionsReadsPasswordEnv(t *testing.T) {
	t.Setenv("ADMIN_SECRET", "  secret  ")

	opts, err := normalizeCreateSuperAdminOptions(rbacCreateSuperAdminOptions{username: " ADMIN ", nickname: " ", passwordEnv: "ADMIN_SECRET", resetPassword: true})
	require.NoError(t, err)
	assert.Equal(t, "admin", opts.username)
	assert.Equal(t, "admin", opts.nickname)
	assert.Equal(t, "secret", opts.password)
	assert.Equal(t, "ADMIN_SECRET", opts.passwordEnv)
	assert.True(t, opts.resetPassword)
}

func TestFxAppLifecycleTimeouts(t *testing.T) {
	require.Equal(t, 15*time.Second, fxAppStartTimeout)

	cfg, err := config.Load("../configs/config.yaml")
	require.NoError(t, err)
	require.GreaterOrEqual(t, fxAppStopTimeout, cfg.HTTP.ShutdownTimeout)
}

func findSubcommand(parent *cobra.Command, use string) *cobra.Command {
	for _, cmd := range parent.Commands() {
		if cmd.Use == use || cmd.Name() == use {
			return cmd
		}
	}
	return nil
}
