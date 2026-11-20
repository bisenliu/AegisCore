package fxgraph

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
	"go.uber.org/fx"
)

func TestRenderDOTReturnsStableGraph(t *testing.T) {
	type dependency struct{}
	type service struct{}
	newDependency := func() dependency { return dependency{} }
	newService := func(dependency) service { return service{} }

	first, err := RenderDOT(
		fx.Provide(newDependency, newService),
		fx.Invoke(func(service) {}),
	)
	require.NoError(t, err)
	second, err := RenderDOT(
		fx.Provide(newDependency, newService),
		fx.Invoke(func(service) {}),
	)
	require.NoError(t, err)
	require.Equal(t, first, second, "RenderDOT output is not stable")
	require.Contains(t, first, "fxgraph.service")
	require.True(t, strings.HasSuffix(first, "\n"), "RenderDOT output does not end with newline: %q", first)
}

func TestRenderDOTDoesNotConstructUnreferencedProviders(t *testing.T) {
	type expensiveResource struct{}
	constructed := false

	dot, err := RenderDOT(
		fx.Provide(func() expensiveResource {
			constructed = true
			return expensiveResource{}
		}),
	)
	require.NoError(t, err)
	require.False(t, constructed, "RenderDOT constructed an unreferenced provider")
	require.Contains(t, dot, "fxgraph.expensiveResource")
}

func TestWriteDOTCreatesFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "graphs", "fx.dot")
	dot, err := WriteDOT(path, fx.Provide(func() string { return "ready" }))
	require.NoError(t, err)
	content, err := os.ReadFile(path)
	require.NoError(t, err)
	require.Equal(t, dot, string(content))
}

func TestWriteFileRejectsEmptyPath(t *testing.T) {
	err := WriteFile("  ", "digraph {}")
	require.Error(t, err)
	require.Contains(t, err.Error(), "output path is required")
}

func TestVisualizeErrorRejectsPlainError(t *testing.T) {
	_, err := VisualizeError(errors.New("plain error"))
	require.Error(t, err)
}
