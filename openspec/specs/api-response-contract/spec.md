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
- **Then** 响应 JSON 包含 `success: true`、`code: OK`、`message: ok`
- **Then** `data` 包含业务响应数据

#### Scenario: Return created response
- **Given** controller 成功创建资源并获得响应数据
- **When** controller 调用 `response.Created`
- **Then** 系统返回 HTTP 201
- **Then** 响应 JSON 包含 `success: true`、`code: OK`、`message: created`
- **Then** `data` 包含创建结果

### Requirement: Return failure responses with application errors

系统必须将应用错误转换为统一失败信封，并使用错误对象中的 HTTP 状态码、错误码和消息。

#### Scenario: Return bad request
- **Given** controller 检测到请求参数无效
- **When** controller 调用 `response.BadRequest`
- **Then** 系统返回 HTTP 400
- **Then** 响应 JSON 包含 `success: false`、`code: BAD_REQUEST` 和调用方可读消息
- **Then** 响应不包含 `data`

#### Scenario: Return not found
- **Given** repository 或 service 返回 not found 应用错误
- **When** controller 调用 `response.Fail`
- **Then** 系统返回 HTTP 404
- **Then** 响应 JSON 包含 `success: false`、`code: NOT_FOUND` 和对应消息

#### Scenario: Wrap unexpected error
- **Given** service 或 repository 返回普通 Go error
- **When** 系统调用 `response.FromError`
- **Then** 错误被包装为 `INTERNAL_ERROR`
- **Then** HTTP 状态码为 500
- **Then** 对外消息为 `internal server error`

### Requirement: Recover panics with failure envelope

系统必须通过 recovery 中间件捕获 panic，记录上下文并返回统一内部错误响应。

#### Scenario: Handler panics
- **Given** HTTP handler 或下游逻辑发生 panic
- **When** recovery 中间件捕获 panic
- **Then** 系统记录 panic、request id 和 stack 信息
- **Then** 系统返回失败信封，错误码为 `INTERNAL_ERROR`
- **Then** 请求被 abort，避免继续执行后续 handler

### Requirement: Preserve request id for observability

系统必须为 HTTP 请求保留或生成 request id，并在响应 header 和日志中使用。

#### Scenario: Request provides request id
- **Given** HTTP 请求包含 `X-Request-ID` header
- **When** request id 中间件处理请求
- **Then** 系统将该值写入 Gin context
- **Then** 响应包含相同的 `X-Request-ID` header

#### Scenario: Request omits request id
- **Given** HTTP 请求没有 `X-Request-ID` header
- **When** request id 中间件处理请求
- **Then** 系统生成新的 UUID 字符串
- **Then** 响应包含生成的 `X-Request-ID` header
