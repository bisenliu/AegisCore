## ADDED Requirements

### Requirement: Support service-local validation boundaries
系统 MUST 允许 Gin controller 在共享请求校验通过后调用服务内 Validation 层处理服务特定的请求清洗、基础校验和复杂跨字段规则。共享 `common/validation` MUST 保持跨服务通用能力定位，不得承载用户服务独有规则。

#### Scenario: Controller delegates service-specific validation
- **Given** controller 已使用共享校验器完成请求绑定和结构体 tag 校验
- **When** 请求还需要用户服务特定的字符串清洗、分页规范化、UUID 解析、请求体 token 规范化或跨字段校验
- **Then** controller MUST 调用服务内 Validation 层或等价请求校验边界
- **Then** controller MUST NOT 将大量服务特定规则内联在 HTTP handler 中

#### Scenario: Shared validation remains generic
- **Given** 某个校验规则只适用于用户服务的用户资料请求
- **When** 开发者实现该规则
- **Then** 规则 MUST NOT 放入 `common/validation`
- **Then** 规则 MUST 位于用户服务自己的请求校验边界，除非该规则已成为多个服务稳定复用的通用能力

#### Scenario: Validation errors preserve envelope contract
- **Given** 服务内 Validation 层判定请求级校验失败
- **When** controller 输出失败响应
- **Then** 响应 MUST 继续使用 `common/response.Envelope`
- **Then** 错误状态码、业务码和公开消息 MUST 与现有请求校验错误语义兼容
