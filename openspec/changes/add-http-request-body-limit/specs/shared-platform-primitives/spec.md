## ADDED Requirements

### Requirement: HTTP 入站请求体容量边界

系统 MUST 在共享 HTTP helper 中提供业务中立的入站请求体字节上限能力。该能力 MUST 在 JSON 解码前限制 `Request.Body` 可读取字节数，MUST 覆盖固定 `Content-Length`、chunked 请求体和首个 JSON 文档后的尾随数据，并 MUST 将超限错误渲染为 `413 Payload Too Large` 与统一错误 envelope。

#### Scenario: JSON 解码前拒绝超限请求体

- **WHEN** HTTP 请求体超过调用方配置的最大字节数，且请求使用固定 `Content-Length` 或 chunked 传输
- **THEN** 共享 body limit 能力 MUST 在业务 binder 完成解码前返回稳定超限错误
- **AND** 后续 feature use case MUST NOT 被调用

#### Scenario: 尾随 JSON 不能绕过容量边界

- **WHEN** 请求体包含一个合法 JSON 文档，随后追加导致总请求体超限的尾随 JSON 或其他数据
- **THEN** 系统 MUST 在读取尾随数据时继续受同一字节上限约束
- **AND** 响应 MUST 为 `413 Payload Too Large`，不得因第二次 JSON 解码分配与载荷大小线性增长的堆内存

#### Scenario: 超限错误使用统一响应契约

- **WHEN** 共享 HTTP helper、middleware 或 binder 产生请求体超限错误
- **THEN** `common/http/response` MUST 渲染 `413 Payload Too Large`
- **AND** 响应 MUST 使用统一错误 envelope、稳定应用错误码和非敏感公开消息
- **AND** 系统 MUST NOT 将请求体超限渲染为 `400 Bad Request`、`429 Too Many Requests` 或 `500 Internal Server Error`

#### Scenario: common 不拥有服务私有策略

- **WHEN** 消费服务需要默认请求体上限、端点覆盖或路径匹配策略
- **THEN** 这些策略 MUST 由消费服务拥有
- **AND** `common` MUST NOT 内置 user-service 路由、auth/user DTO、部署资源预算或服务私有配置字段
