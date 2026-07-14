package main

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/spf13/cobra"
	"go.uber.org/fx"
)

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

func testRootCommandDependencies(t testing.TB) rootCommandDependencies {
	t.Helper()
	unexpected := func(name string) {
		t.Helper()
		t.Fatalf("unexpected root command dependency call: %s", name)
	}
	return rootCommandDependencies{
		appFactory: func(string) lifecycleApp {
			unexpected("appFactory")
			return testLifecycleApp{}
		},
		seedRunner: func(context.Context, string, rbacSeedOptions) error {
			unexpected("seedRunner")
			return nil
		},
		assignSuperAdminRunner: func(context.Context, string, uuid.UUID) error {
			unexpected("assignSuperAdminRunner")
			return nil
		},
		createSuperAdminRunner: func(context.Context, string, rbacCreateSuperAdminOptions) error {
			unexpected("createSuperAdminRunner")
			return nil
		},
		fxGraphWriter: func(string, ...fx.Option) (string, error) {
			unexpected("fxGraphWriter")
			return "", nil
		},
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
