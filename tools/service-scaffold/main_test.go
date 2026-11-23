package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRunMissingName(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := run([]string{}, &stdout, &stderr)
	if code != exitError {
		t.Fatalf("expected exit code %d, got %d", exitError, code)
	}
	if !strings.Contains(stderr.String(), "必须通过 -name 指定新服务名称") {
		t.Fatalf("expected error message in stderr, got: %s", stderr.String())
	}
}

func TestRunInvalidName(t *testing.T) {
	testCases := []struct {
		name        string
		serviceName string
		expectedErr string
	}{
		{
			name:        "uppercase letters",
			serviceName: "OrderService",
			expectedErr: "invalid service name",
		},
		{
			name:        "underscore",
			serviceName: "order_service",
			expectedErr: "invalid service name",
		},
		{
			name:        "reserved platform",
			serviceName: "platform",
			expectedErr: "reserved system directory name",
		},
		{
			name:        "reserved deployments",
			serviceName: "deployments",
			expectedErr: "reserved system directory name",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			var stdout, stderr bytes.Buffer
			code := run([]string{"-name", tc.serviceName}, &stdout, &stderr)
			if code != exitError {
				t.Fatalf("expected exit code %d, got %d", exitError, code)
			}
			if !strings.Contains(stderr.String(), tc.expectedErr) {
				t.Fatalf("expected error containing %q, got: %s", tc.expectedErr, stderr.String())
			}
		})
	}
}

func TestRunInvalidPort(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := run([]string{"-name", "order-service", "-port", "99999"}, &stdout, &stderr)
	if code != exitError {
		t.Fatalf("expected exit code %d, got %d", exitError, code)
	}
	if !strings.Contains(stderr.String(), "must be between 1 and 65535") {
		t.Fatalf("expected port validation error, got: %s", stderr.String())
	}
}

func TestRunDryRun(t *testing.T) {
	tempDir := t.TempDir()
	var stdout, stderr bytes.Buffer

	code := run([]string{
		"-name", "order-service",
		"-repo-root", tempDir,
		"-dry-run",
	}, &stdout, &stderr)

	if code != exitOK {
		t.Fatalf("expected exit code %d, got %d (stderr: %s)", exitOK, code, stderr.String())
	}

	if !strings.Contains(stdout.String(), "[dry-run] will create file: order-service/cmd/main.go") {
		t.Fatalf("expected dry-run output, got: %s", stdout.String())
	}

	// 确认未在临时目录创建任何实际文件
	entries, err := os.ReadDir(tempDir)
	if err != nil {
		t.Fatalf("failed to read temp dir: %v", err)
	}
	if len(entries) != 0 {
		t.Fatalf("expected empty directory in dry-run mode, got %d entries", len(entries))
	}
}

func TestRunSuccessGenerateAndGoWorkUpdate(t *testing.T) {
	tempDir := t.TempDir()

	// 在临时目录构造初始 go.work 文件
	initialGoWork := `go 1.27.4

toolchain go1.27.4

use (
	./platform
	./user-service
)
`
	goWorkPath := filepath.Join(tempDir, "go.work")
	if err := os.WriteFile(goWorkPath, []byte(initialGoWork), 0644); err != nil {
		t.Fatalf("failed to write fixture go.work: %v", err)
	}

	var stdout, stderr bytes.Buffer
	code := run([]string{
		"-name", "order-service",
		"-port", "8085",
		"-repo-root", tempDir,
	}, &stdout, &stderr)

	if code != exitOK {
		t.Fatalf("expected exit code %d, got %d (stderr: %s)", exitOK, code, stderr.String())
	}

	// 校验核心生成文件存在
	expectedFiles := []string{
		"order-service/cmd/main.go",
		"order-service/cmd/serve.go",
		"order-service/internal/bootstrap/app.go",
		"order-service/internal/bootstrap/server.go",
		"order-service/internal/config/config.go",
		"order-service/internal/providers/fx.go",
		"order-service/internal/providers/observability/fx.go",
		"order-service/internal/providers/transport/fx.go",
		"order-service/internal/router/router.go",
		"order-service/internal/router/health.go",
		"order-service/internal/router/health_test.go",
		"order-service/go.mod",
		"order-service/Makefile",
		"order-service/README.md",
		"deployments/nacos/local-host/order-service.yaml",
		"deployments/nacos/local-docker/order-service.yaml",
		"deployments/docker/order-service.Dockerfile",
	}

	for _, file := range expectedFiles {
		fullPath := filepath.Join(tempDir, file)
		info, err := os.Stat(fullPath)
		if err != nil {
			t.Errorf("expected file %s to exist, err: %v", file, err)
			continue
		}
		if info.IsDir() {
			t.Errorf("expected %s to be a file, but found a directory", file)
		}
	}

	// 验证 go.mod 内容替换正确
	goModBytes, err := os.ReadFile(filepath.Join(tempDir, "order-service/go.mod"))
	if err != nil {
		t.Fatalf("read generated go.mod: %v", err)
	}
	if !strings.Contains(string(goModBytes), "module github.com/aegiscore/order-service") {
		t.Fatalf("go.mod missing expected module path: %s", string(goModBytes))
	}

	// 验证 Nacos 配置端口
	nacosBytes, err := os.ReadFile(filepath.Join(tempDir, "deployments/nacos/local-host/order-service.yaml"))
	if err != nil {
		t.Fatalf("read generated nacos config: %v", err)
	}
	if !strings.Contains(string(nacosBytes), "port: 8085") {
		t.Fatalf("nacos config missing custom port: %s", string(nacosBytes))
	}

	// 验证 go.work 注册
	updatedGoWorkBytes, err := os.ReadFile(goWorkPath)
	if err != nil {
		t.Fatalf("read updated go.work: %v", err)
	}
	if !strings.Contains(string(updatedGoWorkBytes), "\t./order-service\n") {
		t.Fatalf("go.work missing registered module: %s", string(updatedGoWorkBytes))
	}
}

func TestRunConflictDirectory(t *testing.T) {
	tempDir := t.TempDir()
	serviceDir := filepath.Join(tempDir, "order-service")
	if err := os.MkdirAll(serviceDir, 0755); err != nil {
		t.Fatalf("failed to create directory: %v", err)
	}

	// 未指定 -force 应该报错
	var stdout, stderr bytes.Buffer
	code := run([]string{
		"-name", "order-service",
		"-repo-root", tempDir,
	}, &stdout, &stderr)

	if code != exitError {
		t.Fatalf("expected exit code %d, got %d", exitError, code)
	}
	if !strings.Contains(stderr.String(), "already exists") {
		t.Fatalf("expected conflict error, got: %s", stderr.String())
	}

	// 指定 -force 应该允许执行
	stdout.Reset()
	stderr.Reset()
	code = run([]string{
		"-name", "order-service",
		"-repo-root", tempDir,
		"-force",
	}, &stdout, &stderr)

	if code != exitOK {
		t.Fatalf("expected exit code %d with -force, got %d (stderr: %s)", exitOK, code, stderr.String())
	}
}
