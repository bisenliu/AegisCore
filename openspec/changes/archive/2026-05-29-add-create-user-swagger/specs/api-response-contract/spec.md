## ADDED Requirements

### Requirement: Document response envelope in Swagger
系统必须在 Swagger/OpenAPI 文档中复用运行时 `common/response.Envelope` 语义描述业务 API 成功和失败响应，确保文档中的状态码、业务码、消息和 data 包装方式与真实响应一致。

#### Scenario: Document created response envelope
- **Given** 创建用户接口成功创建资源
- **When** Swagger 文档描述 HTTP 201 响应
- **Then** 文档必须显示响应包含 `success: true`、`code: 0`、`message: created`
- **Then** 文档必须显示 `data` 为创建后的用户资料结构

#### Scenario: Document validation failure envelope
- **Given** 业务 API 请求参数校验失败
- **When** Swagger 文档描述 HTTP 400 参数错误响应
- **Then** 文档必须显示失败响应使用统一信封
- **Then** 文档必须说明业务码可能为 `10000` 或 `10001`，取决于请求体解析错误或字段校验错误

#### Scenario: Document conflict response envelope
- **Given** 创建用户时发生用户已存在冲突
- **When** Swagger 文档描述 HTTP 409 响应
- **Then** 文档必须显示失败响应使用统一信封
- **Then** 文档必须说明业务码为 `40000`

#### Scenario: Document not found and internal error envelopes
- **Given** 查询用户不存在或下游出现非预期错误
- **When** Swagger 文档描述 HTTP 404 或 HTTP 500 响应
- **Then** 文档必须显示失败响应使用统一信封
- **Then** 文档必须说明对应业务码为 `50000` 或 `90000`
