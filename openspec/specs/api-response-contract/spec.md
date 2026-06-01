# api-response-contract

## Purpose

API 响应契约能力定义所有 HTTP API 的成功与失败 JSON 信封、错误码和应用错误映射，确保调用方能以一致方式解析结果。

## Requirements

### Requirement: Return success responses with envelope

系统必须将成功响应包装为统一 `Envelope`。

#### Scenario: Return OK response
- **Given** controller 成功处理请求并获得响应数据
- **When** controller 调用 `response.OK`
- **Then** 系统返回 HTTP 200
- **Then** 响应 JSON 包含 `success: true`、`code: 0`、`message: ok`
- **Then** `data` 包含业务响应数据

#### Scenario: Return created response
- **Given** controller 成功创建资源并获得响应数据
- **When** controller 调用 `response.Created`
- **Then** 系统返回 HTTP 201
- **Then** 响应 JSON 包含 `success: true`、`code: 0`、`message: created`
- **Then** `data` 包含创建结果

### Requirement: Return failure responses with application errors

系统必须将应用错误转换为统一失败信封，并使用错误对象中的 HTTP 状态码、数字业务码和消息。参数校验失败响应必须在统一失败信封中携带字段级 `errors` 明细。

#### Scenario: Return bad request
- **Given** controller 检测到请求格式错误、请求体格式错误或参数无法解析
- **When** controller 调用 `response.BadRequest` 或返回 `response.BadRequestError`
- **Then** 系统返回 HTTP 400
- **Then** 响应 JSON 包含 `success: false`、`code: 10000` 和调用方可读消息
- **Then** 响应不包含 `data`

#### Scenario: Return validation failed
- **Given** 请求参数绑定成功但 required、min、max、len、email、enum、gt、gte、lt 或 lte 等校验规则不通过
- **When** validation helper 生成失败响应
- **Then** 系统返回 HTTP 400
- **Then** 响应 JSON 包含 `success: false`、`code: 10001` 和顶层消息 `请求参数验证失败`
- **Then** 响应 JSON 包含 `errors` 数组
- **Then** 每个 `errors` 元素必须包含 `field`、`label`、`rule`、`message`
- **Then** 响应不包含业务 `data`

#### Scenario: Return unauthenticated
- **Given** 请求缺少有效认证信息
- **When** controller 或中间件调用 `response.Unauthenticated` 或返回 `response.UnauthenticatedError`
- **Then** 系统返回 HTTP 401
- **Then** 响应 JSON 包含 `success: false`、`code: 20000` 和调用方可读消息

#### Scenario: Return forbidden
- **Given** 已认证调用方无权访问资源或执行操作
- **When** controller 或 service 返回 `response.ForbiddenError`
- **Then** 系统返回 HTTP 403
- **Then** 响应 JSON 包含 `success: false`、`code: 30000` 和调用方可读消息

#### Scenario: Return conflict
- **Given** service 检测到业务冲突或资源状态不允许当前操作
- **When** service 返回 `response.ConflictError`
- **Then** 系统返回 HTTP 409
- **Then** 响应 JSON 包含 `success: false`、`code: 40000` 和调用方可读消息

#### Scenario: Return not found
- **Given** repository 或 service 返回 not found 应用错误
- **When** controller 调用 `response.Fail`
- **Then** 系统返回 HTTP 404
- **Then** 响应 JSON 包含 `success: false`、`code: 50000` 和对应消息

#### Scenario: Wrap unexpected error
- **Given** service 或 repository 返回普通 Go error
- **When** 系统调用 `response.FromError`
- **Then** 错误被包装为内部错误
- **Then** HTTP 状态码为 500
- **Then** 响应 JSON 包含 `success: false`、`code: 90000` 和对外安全消息 `internal server error`

### Requirement: Recover panics with failure envelope

系统必须通过 recovery 中间件捕获 panic，记录上下文并返回统一内部错误响应。

#### Scenario: Handler panics
- **Given** HTTP handler 或下游逻辑发生 panic
- **When** recovery 中间件捕获 panic
- **Then** 系统记录 panic、trace-id 和 stack 信息
- **Then** 系统返回失败信封，错误码为 `90000`
- **Then** 请求被 abort，避免继续执行后续 handler

### Requirement: Preserve trace id for observability

系统必须为 HTTP 请求保留或生成 trace id，并在响应 header、context 和日志中使用。

#### Scenario: Request provides trace id
- **Given** HTTP 请求包含 `X-Trace-ID` header
- **When** trace id 中间件处理请求
- **Then** 系统将该值写入 Gin context
- **Then** 系统将该值写入 Go `context.Context`
- **Then** 响应包含相同的 `X-Trace-ID` header

#### Scenario: Request omits trace id
- **Given** HTTP 请求没有 `X-Trace-ID` header
- **When** trace id 中间件处理请求
- **Then** 系统生成新的 UUID 字符串
- **Then** 系统将生成值写入 Gin context 和 Go `context.Context`
- **Then** 响应包含生成的 `X-Trace-ID` header

#### Scenario: Health endpoint uses minimal status payload
- **Given** HTTP server 已启动
- **When** 调用方请求 `GET /healthz`
- **Then** 系统可以返回最小健康状态 JSON，而不要求使用业务 API 响应信封
- **Then** 业务 API 仍必须使用 `common/response.Envelope`

### Requirement: Provide standard numeric response codes

系统必须在 `common/response` 中定义标准数字业务码，业务码必须独立于 HTTP status code。

#### Scenario: Standard code values are stable
- **Given** 服务构造成功或失败响应
- **When** 响应被序列化为 JSON
- **Then** 成功响应使用业务码 `0`
- **Then** 通用请求错误使用业务码 `10000`
- **Then** 参数校验失败使用业务码 `10001`
- **Then** 未认证使用业务码 `20000`
- **Then** 无权限使用业务码 `30000`
- **Then** 业务冲突使用业务码 `40000`
- **Then** 资源不存在使用业务码 `50000`
- **Then** 服务内部错误使用业务码 `90000`

### Requirement: Provide reusable error constructors

系统必须提供通用错误构造函数和 Gin helper，供服务侧以固定文案或格式化模板创建标准失败响应。

#### Scenario: Create fixed-message application error
- **Given** 调用方传入固定错误文案且不传格式化参数
- **When** 调用 `BadRequestError`、`UnauthenticatedError`、`ForbiddenError`、`ConflictError` 或 `NotFoundError`
- **Then** 系统直接使用传入文案作为 `message`
- **Then** 即使文案包含 `%` 字符也不得按格式化模板解析

#### Scenario: Create formatted application error
- **Given** 调用方传入格式化模板和参数
- **When** 调用 `BadRequestError`、`UnauthenticatedError`、`ForbiddenError`、`ConflictError` 或 `NotFoundError`
- **Then** 系统使用模板和参数生成 `message`
- **Then** 错误对象包含对应业务码和 HTTP status code

#### Scenario: Wrap internal error with public message
- **Given** 下游返回内部错误原因且 service 提供对外安全文案
- **When** 调用 `response.WrapInternal(err, publicMessage)`
- **Then** 错误对象保留内部错误原因用于 `Unwrap`
- **Then** 响应 JSON 使用业务码 `90000` 和 `publicMessage`

### Requirement: Maintain service business messages separately

`user-services` 必须在 `internal/apperror` 集中维护非参数校验类业务错误文案常量和模板，并复用 `common/response` 通用错误构造函数。

#### Scenario: Return user business error from shared message
- **Given** repository 或 service 需要返回用户服务业务错误
- **When** 业务错误不属于参数校验错误
- **Then** 代码使用 `user-services/internal/apperror` 中的文案常量或模板
- **Then** 代码使用 `common/response` 的通用错误构造函数创建应用错误
- **Then** `user-services` 不为每条业务错误封装专用响应 helper

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
