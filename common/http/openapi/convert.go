package openapi

import (
	"context"
	"encoding/json"
	"fmt"
	"os"

	"github.com/getkin/kin-openapi/openapi2"
	"github.com/getkin/kin-openapi/openapi2conv"
	"github.com/getkin/kin-openapi/openapi3"
	"gopkg.in/yaml.v3"
)

// ConvertSwagger2File 从文件读取 Swagger 2 JSON，并转换为 OpenAPI 3 文档。
func ConvertSwagger2File(ctx context.Context, inputPath string, opts ConvertOptions) (*Document, error) {
	data, err := os.ReadFile(inputPath)
	if err != nil {
		return nil, fmt.Errorf("read Swagger input: %w", err)
	}
	return ConvertSwagger2JSON(ctx, data, opts)
}

// ConvertSwagger2JSON 将 Swagger 2 JSON 转换为 OpenAPI 3 文档和 JSON/YAML 输出。
func ConvertSwagger2JSON(ctx context.Context, data []byte, opts ConvertOptions) (*Document, error) {
	var doc2 openapi2.T
	if err := json.Unmarshal(data, &doc2); err != nil {
		return nil, fmt.Errorf("decode Swagger input: %w", err)
	}

	doc3, err := openapi2conv.ToV3(&doc2)
	if err != nil {
		return nil, fmt.Errorf("convert to OpenAPI 3: %w", err)
	}

	normalizeDocument(doc3, opts)
	if err := doc3.Validate(ctx); err != nil {
		return nil, fmt.Errorf("validate OpenAPI 3 document: %w", err)
	}

	jsonData, err := json.MarshalIndent(doc3, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("marshal OpenAPI JSON: %w", err)
	}
	jsonData = append(jsonData, '\n')

	yamlData, err := yaml.Marshal(doc3)
	if err != nil {
		return nil, fmt.Errorf("marshal OpenAPI YAML: %w", err)
	}

	return &Document{
		OpenAPI:   doc3.OpenAPI,
		PathCount: doc3.Paths.Len(),
		JSON:      jsonData,
		YAML:      yamlData,
	}, nil
}

func normalizeDocument(doc *openapi3.T, opts ConvertOptions) {
	doc.OpenAPI = opts.openAPIVersion()

	if len(opts.Servers) > 0 {
		doc.Servers = toOpenAPIServers(opts.Servers)
	}

	for path, servers := range opts.PathServers {
		item := doc.Paths.Find(path)
		if item == nil {
			continue
		}
		item.Servers = toOpenAPIServers(servers)
	}

	if len(opts.SecuritySchemes) == 0 {
		return
	}
	if doc.Components == nil {
		doc.Components = &openapi3.Components{}
	}
	if doc.Components.SecuritySchemes == nil {
		doc.Components.SecuritySchemes = openapi3.SecuritySchemes{}
	}
	for name, scheme := range opts.SecuritySchemes {
		doc.Components.SecuritySchemes[name] = &openapi3.SecuritySchemeRef{Value: toOpenAPISecurityScheme(scheme)}
	}
}

func toOpenAPIServers(servers []Server) openapi3.Servers {
	result := make(openapi3.Servers, 0, len(servers))
	for _, server := range servers {
		result = append(result, &openapi3.Server{URL: server.URL, Description: server.Description})
	}
	return result
}

func toOpenAPISecurityScheme(scheme SecurityScheme) *openapi3.SecurityScheme {
	return &openapi3.SecurityScheme{
		Type:         scheme.Type,
		Scheme:       scheme.Scheme,
		BearerFormat: scheme.BearerFormat,
		Description:  scheme.Description,
		Name:         scheme.Name,
		In:           scheme.In,
	}
}
