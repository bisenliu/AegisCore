package fxgraph

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

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
	if err != nil {
		t.Fatalf("RenderDOT error = %v", err)
	}
	second, err := RenderDOT(
		fx.Provide(newDependency, newService),
		fx.Invoke(func(service) {}),
	)
	if err != nil {
		t.Fatalf("RenderDOT second error = %v", err)
	}
	if first != second {
		t.Fatalf("RenderDOT output is not stable\nfirst:\n%s\nsecond:\n%s", first, second)
	}
	if !strings.Contains(first, "fxgraph.service") {
		t.Fatalf("RenderDOT output = %q, want service node", first)
	}
	if !strings.HasSuffix(first, "\n") {
		t.Fatalf("RenderDOT output does not end with newline: %q", first)
	}
}

func TestWriteDOTCreatesFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "graphs", "fx.dot")
	dot, err := WriteDOT(path, fx.Provide(func() string { return "ready" }))
	if err != nil {
		t.Fatalf("WriteDOT error = %v", err)
	}
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile error = %v", err)
	}
	if string(content) != dot {
		t.Fatalf("written DOT = %q, want %q", string(content), dot)
	}
}

func TestWriteFileRejectsEmptyPath(t *testing.T) {
	err := WriteFile("  ", "digraph {}")
	if err == nil {
		t.Fatal("WriteFile error = nil")
	}
	if !strings.Contains(err.Error(), "output path is required") {
		t.Fatalf("WriteFile error = %q, want output path", err.Error())
	}
}

func TestVisualizeErrorRejectsPlainError(t *testing.T) {
	_, err := VisualizeError(errors.New("plain error"))
	if err == nil {
		t.Fatal("VisualizeError error = nil")
	}
}
