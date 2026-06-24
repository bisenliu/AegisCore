// Package fxgraph 提供业务中立的 Fx 依赖图生成能力。
package fxgraph

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"go.uber.org/fx"
)

// RenderDOT 根据传入 Fx options 生成 DOT 依赖图文本。
func RenderDOT(opts ...fx.Option) (string, error) {
	var graph fx.DotGraph
	allOpts := make([]fx.Option, 0, len(opts)+2)
	allOpts = append(allOpts, fx.NopLogger)
	allOpts = append(allOpts, opts...)
	allOpts = append(allOpts, fx.Populate(&graph))

	app := fx.New(allOpts...)
	if err := app.Err(); err != nil {
		return "", err
	}
	return normalizeDOT(string(graph)), nil
}

// VisualizeError 将 Fx 构图错误转换为 DOT 图文本。
func VisualizeError(err error) (string, error) {
	if err == nil {
		return "", fmt.Errorf("fx error is required")
	}
	dot, visualizeErr := fx.VisualizeError(err)
	if visualizeErr != nil {
		return "", visualizeErr
	}
	return normalizeDOT(dot), nil
}

// WriteDOT 生成 Fx DOT 依赖图并写入目标文件。
func WriteDOT(path string, opts ...fx.Option) (string, error) {
	dot, err := RenderDOT(opts...)
	if err != nil {
		return "", err
	}
	if err := WriteFile(path, dot); err != nil {
		return "", err
	}
	return dot, nil
}

// WriteFile 将 DOT 图文本写入文件，并按需创建父目录。
func WriteFile(path string, dot string) error {
	path = strings.TrimSpace(path)
	if path == "" {
		return fmt.Errorf("fx graph output path is required")
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("create fx graph output directory: %w", err)
	}
	if err := os.WriteFile(path, []byte(normalizeDOT(dot)), 0o644); err != nil {
		return fmt.Errorf("write fx graph output: %w", err)
	}
	return nil
}

func normalizeDOT(dot string) string {
	dot = strings.ReplaceAll(dot, "\r\n", "\n")
	dot = strings.TrimRight(dot, "\n")
	return dot + "\n"
}
