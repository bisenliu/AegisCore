package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"path/filepath"

	commonopenapi "github.com/aegiscore/common/http/openapi"
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

	flag.StringVar(&inputPath, "input", "", "Swagger 2 JSON input path")
	flag.StringVar(&jsonOutputPath, "json", "", "OpenAPI 3 JSON output path")
	flag.StringVar(&yamlOutputPath, "yaml", "", "OpenAPI 3 YAML output path")
	flag.StringVar(&goOutputPath, "go", "", "OpenAPI 3 Go embed output path")
	flag.StringVar(&goPackageName, "package", defaultGoPackageName, "Go package name for the embed output")
	flag.StringVar(&openAPIVersion, "openapi-version", defaultOpenAPIVersion, "OpenAPI version for the generated document")
	flag.StringVar(&serverURL, "server", "", "default OpenAPI server URL")
	flag.StringVar(&rootServerURL, "root-server", "", "OpenAPI server URL for root paths")
	flag.Var(&rootPaths, "root-path", "path that should use the root server URL")
	flag.StringVar(&bearerAuthName, "bearer-auth-name", "", "Bearer auth security scheme name")
	flag.StringVar(&bearerAuthDescription, "bearer-auth-description", "", "Bearer auth security scheme description")
	flag.StringVar(&generatedBy, "generated-by", defaultGeneratedBy, "generator label for the Go embed file")
	flag.Parse()

	if inputPath == "" || jsonOutputPath == "" || yamlOutputPath == "" || goOutputPath == "" {
		failf("input, json, yaml and go output paths are required")
	}
	if len(rootPaths) > 0 && rootServerURL == "" {
		failf("root-server is required when root-path is set")
	}

	doc, err := commonopenapi.ConvertSwagger2File(context.Background(), inputPath, convertOptions(openAPIVersion, serverURL, rootServerURL, rootPaths, bearerAuthName, bearerAuthDescription))
	if err != nil {
		failf("%v", err)
	}

	goData, err := commonopenapi.RenderGoDocument(doc.JSON, commonopenapi.GoDocumentOptions{PackageName: goPackageName, GeneratedBy: generatedBy})
	if err != nil {
		failf("render Go output: %v", err)
	}

	if err := writeFile(jsonOutputPath, doc.JSON); err != nil {
		failf("write JSON output: %v", err)
	}
	if err := writeFile(yamlOutputPath, doc.YAML); err != nil {
		failf("write YAML output: %v", err)
	}
	if err := writeFile(goOutputPath, goData); err != nil {
		failf("write Go output: %v", err)
	}

	fmt.Printf("generated OpenAPI %s document with %d paths\n", doc.OpenAPI, doc.PathCount)
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

func failf(format string, args ...any) {
	fmt.Fprintf(os.Stderr, format+"\n", args...)
	os.Exit(1)
}
