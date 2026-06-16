package openapi

// DefaultOpenAPIVersion 是共享转换器在调用方未指定版本时使用的 OpenAPI 版本。
const DefaultOpenAPIVersion = "3.0.3"

// ConvertOptions 控制 Swagger 2 到 OpenAPI 3 转换后的通用规范化行为。
type ConvertOptions struct {
	OpenAPIVersion  string
	Servers         []Server
	PathServers     map[string][]Server
	SecuritySchemes map[string]SecurityScheme
}

// Server 描述 OpenAPI server 配置。
type Server struct {
	URL         string
	Description string
}

// SecurityScheme 描述 OpenAPI security scheme 配置。
type SecurityScheme struct {
	Type         string
	Scheme       string
	BearerFormat string
	Description  string
	Name         string
	In           string
}

// Document 是共享转换器返回的 OpenAPI 3 文档与序列化产物。
type Document struct {
	OpenAPI   string
	PathCount int
	JSON      []byte
	YAML      []byte
}

// GoDocumentOptions 控制 OpenAPI JSON 文档的 Go 源码渲染。
type GoDocumentOptions struct {
	PackageName  string
	FunctionName string
	ConstName    string
	GeneratedBy  string
}

func (opts ConvertOptions) openAPIVersion() string {
	if opts.OpenAPIVersion == "" {
		return DefaultOpenAPIVersion
	}
	return opts.OpenAPIVersion
}
