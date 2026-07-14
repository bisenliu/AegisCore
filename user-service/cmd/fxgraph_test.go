package main

import (
	"testing"

	"github.com/stretchr/testify/require"
	"go.uber.org/fx"
)

func TestFxGraphCommandWritesGraph(t *testing.T) {
	called := false
	deps := testRootCommandDependencies(t)
	deps.fxGraphWriter = func(path string, opts ...fx.Option) (string, error) {
		called = true
		require.Equal(t, "docs/test.dot", path)
		require.Len(t, opts, 3)
		return "digraph {}\n", nil
	}

	root := newRootCommand(deps)
	root.SetArgs([]string{"fxgraph", "--config", "test-config.yaml", "--output", "docs/test.dot"})
	require.NoError(t, root.Execute())
	require.True(t, called)
}
