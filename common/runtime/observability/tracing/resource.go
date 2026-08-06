package tracing

import (
	"strings"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/sdk/resource"
)

const (
	attributeServiceName           = "service.name"
	attributeDeploymentEnvironment = "deployment.environment"
	attributeServiceVersion        = "service.version"
	attributeServiceInstanceID     = "service.instance.id"
)

// trimSpace 统一裁剪 tracing identity 和 endpoint 配置中的空白字符。
func trimSpace(value string) string {
	return strings.TrimSpace(value)
}

// newResource 构造 OpenTelemetry resource identity。
//
// service.name 和 deployment.environment 是必填身份字段；版本和实例 ID 仅在调用方提供时加入，
// 避免为缺省值写入空字符串属性。
func newResource(serviceName string, environment string, version string, instanceID string) *resource.Resource {
	attrs := []attribute.KeyValue{
		attribute.String(attributeServiceName, serviceName),
		attribute.String(attributeDeploymentEnvironment, environment),
	}
	if version != "" {
		attrs = append(attrs, attribute.String(attributeServiceVersion, version))
	}
	if instanceID != "" {
		attrs = append(attrs, attribute.String(attributeServiceInstanceID, instanceID))
	}
	return resource.NewWithAttributes("", attrs...)
}
