package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"

	commonopenapi "github.com/aegiscore/common/http/openapi"
)

const (
	exitOK    = 0
	exitError = 1
)

const (
	defaultOpenAPIVersion   = "3.0.3"
	defaultGoPackageName    = "docs"
	defaultGeneratedBy      = "tools/openapi-convert"
	defaultBearerAuthType   = "http"
	defaultBearerAuthScheme = "bearer"
	defaultBearerAuthFormat = "JWT"
)

type stringList []string

func (values *stringList) String() string {
	return fmt.Sprint([]string(*values))
}

func (values *stringList) Set(value string) error {
	*values = append(*values, value)
	return nil
}

func main() {
	os.Exit(run(context.Background(), os.Args[1:], os.Stdout, os.Stderr))
}

func run(ctx context.Context, args []string, stdout io.Writer, stderr io.Writer) int {
	var inputPath string
	var jsonOutputPath string
	var yamlOutputPath string
	var goOutputPath string
	var goPackageName string
	var openAPIVersion string
	var serverURL string
	var rootServerURL string
	var bearerAuthName string
	var bearerAuthDescription string
	var generatedBy string
	rootPaths := stringList{}

	flags := flag.NewFlagSet("openapi-convert", flag.ContinueOnError)
	flags.SetOutput(stderr)
	flags.StringVar(&inputPath, "input", "", "Swagger 2 JSON input path")
	flags.StringVar(&jsonOutputPath, "json", "", "OpenAPI 3 JSON output path")
	flags.StringVar(&yamlOutputPath, "yaml", "", "OpenAPI 3 YAML output path")
	flags.StringVar(&goOutputPath, "go", "", "OpenAPI 3 Go embed output path")
	flags.StringVar(&goPackageName, "package", defaultGoPackageName, "Go package name for the embed output")
	flags.StringVar(&openAPIVersion, "openapi-version", defaultOpenAPIVersion, "OpenAPI version for the generated document")
	flags.StringVar(&serverURL, "server", "", "default OpenAPI server URL")
	flags.StringVar(&rootServerURL, "root-server", "", "OpenAPI server URL for root paths")
	flags.Var(&rootPaths, "root-path", "path that should use the root server URL")
	flags.StringVar(&bearerAuthName, "bearer-auth-name", "", "Bearer auth security scheme name")
	flags.StringVar(&bearerAuthDescription, "bearer-auth-description", "", "Bearer auth security scheme description")
	flags.StringVar(&generatedBy, "generated-by", defaultGeneratedBy, "generator label for the Go embed file")
	if err := flags.Parse(args); err != nil {
		return exitError
	}

	if inputPath == "" || jsonOutputPath == "" || yamlOutputPath == "" || goOutputPath == "" {
		failf(stderr, "input, json, yaml and go output paths are required")
		return exitError
	}
	if len(rootPaths) > 0 && rootServerURL == "" {
		failf(stderr, "root-server is required when root-path is set")
		return exitError
	}

	doc, err := commonopenapi.ConvertSwagger2File(ctx, inputPath, convertOptions(openAPIVersion, serverURL, rootServerURL, rootPaths, bearerAuthName, bearerAuthDescription))
	if err != nil {
		failf(stderr, "%v", err)
		return exitError
	}

	goData, err := commonopenapi.RenderGoDocument(doc.JSON, commonopenapi.GoDocumentOptions{PackageName: goPackageName, GeneratedBy: generatedBy})
	if err != nil {
		failf(stderr, "render Go output: %v", err)
		return exitError
	}

	if err := writeFile(jsonOutputPath, doc.JSON); err != nil {
		failf(stderr, "write JSON output: %v", err)
		return exitError
	}
	if err := writeFile(yamlOutputPath, doc.YAML); err != nil {
		failf(stderr, "write YAML output: %v", err)
		return exitError
	}
	if err := writeFile(goOutputPath, goData); err != nil {
		failf(stderr, "write Go output: %v", err)
		return exitError
	}

	fmt.Fprintf(stdout, "generated OpenAPI %s document with %d paths\n", doc.OpenAPI, doc.PathCount)
	return exitOK
}

func convertOptions(openAPIVersion string, serverURL string, rootServerURL string, rootPaths []string, bearerAuthName string, bearerAuthDescription string) commonopenapi.ConvertOptions {
	var servers []commonopenapi.Server
	if serverURL != "" {
		servers = []commonopenapi.Server{{URL: serverURL}}
	}

	pathServers := make(map[string][]commonopenapi.Server, len(rootPaths))
	for _, path := range rootPaths {
		pathServers[path] = []commonopenapi.Server{{URL: rootServerURL}}
	}

	securitySchemes := map[string]commonopenapi.SecurityScheme{}
	if bearerAuthName != "" {
		securitySchemes[bearerAuthName] = commonopenapi.SecurityScheme{
			Type:         defaultBearerAuthType,
			Scheme:       defaultBearerAuthScheme,
			BearerFormat: defaultBearerAuthFormat,
			Description:  bearerAuthDescription,
		}
	}

	return commonopenapi.ConvertOptions{
		OpenAPIVersion:  openAPIVersion,
		Servers:         servers,
		PathServers:     pathServers,
		SecuritySchemes: securitySchemes,
	}
}

func writeFile(path string, data []byte) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o644)
}

func failf(stderr io.Writer, format string, args ...any) {
	fmt.Fprintf(stderr, format+"\n", args...)
}
