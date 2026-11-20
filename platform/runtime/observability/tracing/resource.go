package tracing

import (
	"strings"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/sdk/resource"
)

const (
	// attributeServiceName 是 OpenTelemetry 规范中标识微服务名称的标准资源属性键。
	attributeServiceName = "service.name"
	// attributeDeploymentEnvironment 是标识部署环境（如 dev/prod）的标准资源属性键。
	attributeDeploymentEnvironment = "deployment.environment"
	// attributeServiceVersion 是标识服务发布版本号的标准资源属性键。
	attributeServiceVersion = "service.version"
	// attributeServiceInstanceID 是标识服务运行时 Pod/容器唯一实例 ID 的标准资源属性键。
	attributeServiceInstanceID = "service.instance.id"
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
