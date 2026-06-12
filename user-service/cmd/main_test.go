package main

import (
	"context"
	"testing"
	"time"

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
	t.Cleanup(func() {
		newLifecycleApp = originalFactory
	})

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
	for _, cmd := range root.Commands() {
		if cmd.Use == "serve" {
			serve = cmd
			break
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
