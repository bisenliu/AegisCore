package openapi

// DefaultOpenAPIVersion 是共享转换器在调用方未指定版本时使用的 OpenAPI 版本。
const DefaultOpenAPIVersion = "3.0.3"

// ConvertOptions 控制 Swagger 2 到 OpenAPI 3 转换后的通用规范化行为。
// PathServers 的 key 必须与转换后的 OpenAPI path 完全一致；不存在的 path 会被静默忽略，便于服务脚本复用同一参数集合。
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
// PathCount 来自转换后 path 数量，不代表 operation 数量；JSON 和 YAML 是已经完成规范化与验证后的最终输出。
type Document struct {
	OpenAPI   string
	PathCount int
	JSON      []byte
	YAML      []byte
}

// GoDocumentOptions 控制 OpenAPI JSON 文档的 Go 源码渲染。
// PackageName 必填；FunctionName、ConstName 和 GeneratedBy 为空时使用稳定默认值。
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
