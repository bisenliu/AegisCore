package main

import (
	"context"
	"testing"

	"github.com/spf13/cobra"
	"go.uber.org/fx"

	serviceconfig "github.com/aegiscore/user-service/internal/config"
)

type testLifecycleApp struct {
	start func(context.Context) error
	wait  <-chan fx.ShutdownSignal
	stop  func(context.Context) error
}

func (a testLifecycleApp) Start(ctx context.Context) error {
	return a.start(ctx)
}

func (a testLifecycleApp) Wait() <-chan fx.ShutdownSignal {
	return a.wait
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
		appFactory: func(*serviceconfig.Config) lifecycleApp {
			unexpected("appFactory")
			return testLifecycleApp{}
		},
		configLoader: func(context.Context) (*serviceconfig.LoadResult, error) {
			return serviceconfig.LoadFromDocuments(readRepositoryConfigDocList(t))
		},
		seedRunner: func(context.Context, rbacSeedOptions) error {
			unexpected("seedRunner")
			return nil
		},
		bootstrapSuperAdminRunner: func(context.Context, rbacBootstrapSuperAdminOptions) error {
			unexpected("bootstrapSuperAdminRunner")
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
