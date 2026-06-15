package main

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/spf13/cobra"

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
	t.Cleanup(func() {
		newLifecycleApp = originalFactory
		runRBACSeed = originalSeed
	})
	runRBACSeed = func(context.Context, string, rbacSeedOptions) error {
		t.Fatal("serve must not invoke RBAC seed")
		return nil
	}

	key := testContextKey("trace-id")
	parent := context.WithValue(context.Background(), key, "test-trace")
	ctx, cancel := context.WithCancel(parent)

	newLifecycleApp = func(configPath string) lifecycleApp {
		if configPath != "test-config.yaml" {
			t.Fatalf("configPath = %q, want test-config.yaml", configPath)
		}

		return testLifecycleApp{
			start: func(_ context.Context) error {
				cancel()
				return nil
			},
			stop: func(ctx context.Context) error {
				if got := ctx.Value(key); got != "test-trace" {
					t.Fatalf("stop context value = %v, want test-trace", got)
				}
				if err := ctx.Err(); err != nil {
					t.Fatalf("stop context is already canceled: %v", err)
				}
				deadline, ok := ctx.Deadline()
				if !ok {
					t.Fatal("stop context has no deadline")
				}
				remaining := time.Until(deadline)
				if remaining <= 0 || remaining > fxAppStopTimeout {
					t.Fatalf("stop context remaining timeout = %s, want within %s", remaining, fxAppStopTimeout)
				}
				return nil
			},
		}
	}

	if err := runServe(ctx, "test-config.yaml"); err != nil {
		t.Fatalf("runServe: %v", err)
	}
}

func TestRootCommandSurface(t *testing.T) {
	root := newRootCommand()
	if root.Use != "aegiscore-user-services" {
		t.Fatalf("root Use = %q, want aegiscore-user-services", root.Use)
	}

	var serve *cobra.Command
	var rbac *cobra.Command
	for _, cmd := range root.Commands() {
		switch cmd.Use {
		case "serve":
			serve = cmd
		case "rbac":
			rbac = cmd
		}
	}
	if serve == nil {
		t.Fatal("serve command not registered")
	}

	flag := serve.Flags().Lookup("config")
	if flag == nil {
		t.Fatal("serve --config flag not registered")
	}
	if flag.DefValue != "./configs/config.yaml" {
		t.Fatalf("serve --config default = %q, want ./configs/config.yaml", flag.DefValue)
	}
	if rbac == nil {
		t.Fatal("rbac command not registered")
	}
	if flag := rbac.PersistentFlags().Lookup("config"); flag == nil || flag.DefValue != "./configs/config.yaml" {
		t.Fatalf("rbac --config flag = %#v", flag)
	}
	if findSubcommand(rbac, "seed") == nil {
		t.Fatal("rbac seed command not registered")
	}
	if findSubcommand(rbac, "assign-super-admin") == nil {
		t.Fatal("rbac assign-super-admin command not registered")
	}
}

func TestRBACSeedCommandFlags(t *testing.T) {
	originalSeed := runRBACSeed
	t.Cleanup(func() { runRBACSeed = originalSeed })
	called := false
	runRBACSeed = func(_ context.Context, configPath string, opts rbacSeedOptions) error {
		called = true
		if configPath != "test-config.yaml" {
			t.Fatalf("configPath = %q", configPath)
		}
		if !opts.reactivateSystem || !opts.syncSystemBindings {
			t.Fatalf("opts = %#v", opts)
		}
		return nil
	}

	root := newRootCommand()
	root.SetArgs([]string{"rbac", "--config", "test-config.yaml", "seed", "--reactivate-system", "--sync-system-bindings"})
	if err := root.Execute(); err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if !called {
		t.Fatal("runRBACSeed not called")
	}
}

func TestAssignSuperAdminCommandValidatesUserID(t *testing.T) {
	originalAssign := runAssignSuperAdmin
	t.Cleanup(func() { runAssignSuperAdmin = originalAssign })
	runAssignSuperAdmin = func(_ context.Context, _ string, _ uuid.UUID) error {
		t.Fatal("runAssignSuperAdmin should not be called for invalid UUID")
		return nil
	}

	root := newRootCommand()
	root.SetArgs([]string{"rbac", "assign-super-admin", "--user-id", "not-a-uuid"})
	if err := root.Execute(); err == nil {
		t.Fatal("Execute err = nil, want invalid UUID error")
	}
}

func TestAssignSuperAdminCommandRuns(t *testing.T) {
	originalAssign := runAssignSuperAdmin
	t.Cleanup(func() { runAssignSuperAdmin = originalAssign })
	userID := uuid.MustParse("018f0000-0000-7000-8000-000000000001")
	called := false
	runAssignSuperAdmin = func(_ context.Context, configPath string, got uuid.UUID) error {
		called = true
		if configPath != "test-config.yaml" {
			t.Fatalf("configPath = %q", configPath)
		}
		if got != userID {
			t.Fatalf("userID = %s", got)
		}
		return nil
	}

	root := newRootCommand()
	root.SetArgs([]string{"rbac", "--config", "test-config.yaml", "assign-super-admin", "--user-id", userID.String()})
	if err := root.Execute(); err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if !called {
		t.Fatal("runAssignSuperAdmin not called")
	}
}

func TestFxAppLifecycleTimeouts(t *testing.T) {
	if fxAppStartTimeout != 15*time.Second {
		t.Fatalf("fxAppStartTimeout = %s, want 15s", fxAppStartTimeout)
	}

	cfg, err := config.Load("../configs/config.yaml")
	if err != nil {
		t.Fatalf("Load default config: %v", err)
	}
	if fxAppStopTimeout < cfg.HTTP.ShutdownTimeout {
		t.Fatalf("fxAppStopTimeout = %s, want at least configured http.shutdown_timeout %s", fxAppStopTimeout, cfg.HTTP.ShutdownTimeout)
	}
}

func findSubcommand(parent *cobra.Command, use string) *cobra.Command {
	for _, cmd := range parent.Commands() {
		if cmd.Use == use || cmd.Name() == use {
			return cmd
		}
	}
	return nil
}
