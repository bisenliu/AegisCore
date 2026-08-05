package main

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

var errWriterUnavailable = errors.New("writer unavailable")

type unavailableWriter struct{}

func (unavailableWriter) Write([]byte) (int, error) {
	return 0, errWriterUnavailable
}

func TestRunRequiresInputAndOutputPaths(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	code := run(context.Background(), []string{"-input", "swagger.json"}, &stdout, &stderr)

	require.Equal(t, exitError, code)
	require.Empty(t, stdout.String())
	require.Contains(t, stderr.String(), "input, json, yaml and go output paths are required")
}

func TestRunRequiresRootServerWhenRootPathIsSet(t *testing.T) {
	dir := t.TempDir()
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	code := run(context.Background(), []string{
		"-input", filepath.Join(dir, "swagger.json"),
		"-json", filepath.Join(dir, "openapi.json"),
		"-yaml", filepath.Join(dir, "openapi.yaml"),
		"-go", filepath.Join(dir, "openapi.go"),
		"-root-path", "/livez",
	}, &stdout, &stderr)

	require.Equal(t, exitError, code)
	require.Empty(t, stdout.String())
	require.Contains(t, stderr.String(), "root-server is required when root-path is set")
}

func TestRunReturnsErrorForMissingInput(t *testing.T) {
	dir := t.TempDir()
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	code := run(context.Background(), []string{
		"-input", filepath.Join(dir, "missing.json"),
		"-json", filepath.Join(dir, "out", "openapi.json"),
		"-yaml", filepath.Join(dir, "out", "openapi.yaml"),
		"-go", filepath.Join(dir, "out", "openapi.go"),
	}, &stdout, &stderr)

	require.Equal(t, exitError, code)
	require.Empty(t, stdout.String())
	require.Contains(t, stderr.String(), "read Swagger input")
}

func TestRunReturnsErrorForUnwritableOutput(t *testing.T) {
	dir := t.TempDir()
	inputPath := filepath.Join(dir, "swagger.json")
	writeMinimalSwagger(t, inputPath)
	require.NoError(t, os.Mkdir(filepath.Join(dir, "openapi.json"), 0o755))
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	code := run(context.Background(), []string{
		"-input", inputPath,
		"-json", filepath.Join(dir, "openapi.json"),
		"-yaml", filepath.Join(dir, "openapi.yaml"),
		"-go", filepath.Join(dir, "openapi.go"),
	}, &stdout, &stderr)

	require.Equal(t, exitError, code)
	require.Empty(t, stdout.String())
	require.Contains(t, stderr.String(), "write JSON output")
}

func TestRunGeneratesOpenAPIFiles(t *testing.T) {
	dir := t.TempDir()
	inputPath := filepath.Join(dir, "swagger.json")
	jsonPath := filepath.Join(dir, "docs", "openapi.json")
	yamlPath := filepath.Join(dir, "docs", "openapi.yaml")
	goPath := filepath.Join(dir, "docs", "openapi.go")
	writeMinimalSwagger(t, inputPath)
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	code := run(context.Background(), []string{
		"-input", inputPath,
		"-json", jsonPath,
		"-yaml", yamlPath,
		"-go", goPath,
		"-package", "apidocs",
		"-server", "/api/v1",
		"-root-server", "/",
		"-root-path", "/livez",
		"-bearer-auth-name", "BearerAuth",
		"-bearer-auth-description", "JWT bearer token",
	}, &stdout, &stderr)

	require.Equal(t, exitOK, code, stderr.String())
	require.Empty(t, stderr.String())
	require.Contains(t, stdout.String(), "generated OpenAPI 3.0.3 document with 2 paths")
	jsonData, err := os.ReadFile(jsonPath)
	require.NoError(t, err)
	require.Contains(t, string(jsonData), `"openapi": "3.0.3"`)
	require.Contains(t, string(jsonData), `"/api/v1/users"`)
	require.Contains(t, string(jsonData), `"BearerAuth"`)
	yamlData, err := os.ReadFile(yamlPath)
	require.NoError(t, err)
	require.Contains(t, string(yamlData), "openapi: 3.0.3")
	require.Contains(t, string(yamlData), "/livez:")
	goData, err := os.ReadFile(goPath)
	require.NoError(t, err)
	require.Contains(t, string(goData), "package apidocs")
	require.Contains(t, string(goData), "tools/openapi-convert")
}

func TestRunReturnsErrorWhenSuccessOutputFails(t *testing.T) {
	dir := t.TempDir()
	inputPath := filepath.Join(dir, "swagger.json")
	writeMinimalSwagger(t, inputPath)
	var stderr bytes.Buffer

	code := run(context.Background(), []string{
		"-input", inputPath,
		"-json", filepath.Join(dir, "openapi.json"),
		"-yaml", filepath.Join(dir, "openapi.yaml"),
		"-go", filepath.Join(dir, "openapi.go"),
	}, unavailableWriter{}, &stderr)

	require.Equal(t, exitError, code)
	require.Contains(t, stderr.String(), "write success output: writer unavailable")
}

func writeMinimalSwagger(t *testing.T, path string) {
	t.Helper()
	data := []byte(`{
		"swagger": "2.0",
		"info": {"title": "Test API", "version": "1.0.0"},
		"paths": {
			"/api/v1/users": {
				"get": {
					"responses": {"200": {"description": "ok"}}
				}
			},
			"/livez": {
				"get": {
					"responses": {"200": {"description": "ok"}}
				}
			}
		}
	}`)
	require.NoError(t, os.WriteFile(path, data, 0o644))
}
