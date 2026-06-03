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

### Requirement: Return paginated success responses

系统 MUST 在 `common/response` 中提供可复用分页响应数据结构。分页列表成功响应 MUST 保持统一 `Envelope` 顶层字段，并将列表数据包装到 `data.items`，分页元信息包装到 `data.pagination`。

#### Scenario: Return paginated list payload
- **Given** controller 成功处理分页列表请求并获得列表数据和分页元信息
- **When** controller 使用 `response.OK` 返回分页数据
- **Then** 系统返回 HTTP 200
- **Then** 响应 JSON 包含 `success: true`、`code: 0`、`message: ok`
- **Then** `data.items` MUST 包含当前页业务数据数组
- **Then** `data.pagination` MUST 包含 `page`、`page_size`、`total`、`total_pages`

#### Scenario: Return empty paginated list
- **Given** 分页列表请求没有匹配到任何记录
- **When** controller 返回分页数据
- **Then** 系统返回 HTTP 200
- **Then** `data.items` MUST 为空数组
- **Then** `data.pagination.total` MUST 为 `0`
- **Then** `data.pagination.total_pages` MUST 为 `0`

### Requirement: Normalize pagination query parameters

系统 MUST 在 common 中提供可复用分页参数规范化方法，用于为 list 类接口统一处理 `page`、`page_size` 默认值并计算数据库分页参数。

#### Scenario: Default missing pagination parameters
- **Given** list 请求未提供 `page` 或 `page_size`
- **When** 系统规范化分页参数
- **Then** `page` MUST 使用默认值 `1`
- **Then** `page_size` MUST 使用默认值 `10`
- **Then** 数据库查询 offset MUST 为 `0`
- **Then** 数据库查询 limit MUST 为 `10`

#### Scenario: Default invalid pagination parameters
- **Given** list 请求提供的 `page` 小于 `1` 或 `page_size` 小于 `1`
- **When** 系统规范化分页参数
- **Then** 小于 `1` 的 `page` MUST 使用默认值 `1`
- **Then** 小于 `1` 的 `page_size` MUST 使用默认值 `10`

#### Scenario: Calculate total pages
- **Given** `total` 为 `128` 且 `page_size` 为 `20`
- **When** 系统构造分页元信息
- **Then** `total_pages` MUST 为 `7`

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

### Requirement: Return token-specific authentication errors

系统 MUST 对 JWT 认证失败返回统一失败信封，并在能够可靠分类时使用 token 专用业务码。所有 token 专用认证失败响应 MUST 使用 HTTP 401，且 MUST NOT 在对外 `message` 中暴露签名、issuer、audience 或 claims 校验细节。

#### Scenario: Return unauthenticated for missing credentials
- **Given** 请求需要认证
- **When** 请求缺少 `Authorization` header
- **Then** 系统 MUST 返回 HTTP 401
- **Then** 响应 JSON MUST 包含 `success: false`、`code: 20000` 和统一未认证消息
- **Then** 响应 MUST NOT 包含业务 `data`

#### Scenario: Return token invalid for malformed bearer token
- **Given** 请求需要认证
- **When** 请求的 `Authorization` header 不是 `Bearer ` 前缀、bearer token 为空、token 格式错误、签名非法、签名算法不允许、issuer 或 audience 不匹配，或 token 缺少必须的用户标识
- **Then** 系统 MUST 返回 HTTP 401
- **Then** 响应 JSON MUST 包含 `success: false`、`code: 20001` 和统一未认证消息
- **Then** 响应 MUST NOT 包含业务 `data`

#### Scenario: Return token expired for expired token
- **Given** 请求需要认证
- **When** 请求携带的 JWT 已过期
- **Then** 系统 MUST 返回 HTTP 401
- **Then** 响应 JSON MUST 包含 `success: false`、`code: 20002` 和统一未认证消息
- **Then** 响应 MUST NOT 包含业务 `data`

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
- **Then** Token 格式错误、非法或签名解析失败使用业务码 `20001`
- **Then** Token 已过期使用业务码 `20002`
- **Then** 无权限使用业务码 `30000`
- **Then** 业务冲突使用业务码 `40000`
- **Then** 资源不存在使用业务码 `50000`
- **Then** 服务内部错误使用业务码 `90000`

### Requirement: Provide reusable error constructors

系统必须提供通用错误构造函数和 Gin helper，供服务侧以固定文案或格式化模板创建标准失败响应。

#### Scenario: Create fixed-message application error
- **Given** 调用方传入固定错误文案且不传格式化参数
- **When** 调用 `BadRequestError`、`UnauthenticatedError`、`TokenInvalidError`、`TokenExpiredError`、`ForbiddenError`、`ConflictError` 或 `NotFoundError`
- **Then** 系统直接使用传入文案作为 `message`
- **Then** 即使文案包含 `%` 字符也不得按格式化模板解析

#### Scenario: Create formatted application error
- **Given** 调用方传入格式化模板和参数
- **When** 调用 `BadRequestError`、`UnauthenticatedError`、`TokenInvalidError`、`TokenExpiredError`、`ForbiddenError`、`ConflictError` 或 `NotFoundError`
- **Then** 系统使用模板和参数生成 `message`
- **Then** 错误对象包含对应业务码和 HTTP status code

#### Scenario: Wrap internal error with public message
- **Given** 下游返回内部错误原因且 service 提供对外安全文案
- **When** 调用 `response.WrapInternal(err, publicMessage)`
- **Then** 错误对象保留内部错误原因用于 `Unwrap`
- **Then** 响应 JSON 使用业务码 `90000` 和 `publicMessage`

### Requirement: User service owns error message constants separately from application errors

用户服务 MUST 将服务内复用的错误消息文本常量放在 `user-services/internal/errmsg` 包中，并 MUST NOT 使用 `user-services/internal/apperror` 承载仅包含消息文本的常量。迁移 MUST 保持既有响应信封、HTTP 状态码、业务错误码和对外错误消息文本不变。

#### Scenario: Reuse service error message text
- **Given** user-services controller、service 或相关业务代码需要复用用户错误消息文本
- **When** 代码引用消息常量
- **Then** 系统 MUST 从 `user-services/internal/errmsg` 引用 `Msg*` 常量
- **Then** 常量值 MUST 与迁移前对应消息文本一致

#### Scenario: Preserve external error response contract
- **Given** 请求触发用户服务中使用迁移后消息常量的失败路径
- **When** controller 或中间件返回统一失败响应
- **Then** 响应 MUST 继续使用 `common/response.Envelope` 失败信封
- **Then** HTTP 状态码和业务错误码 MUST 与迁移前一致
- **Then** 对外 `message` 文本 MUST 与迁移前一致

#### Scenario: Remove misleading application error package
- **Given** 迁移完成后的 user-services 代码
- **When** 仓库中搜索 `user-services/internal/apperror` 或 `apperror.Msg`
- **Then** 系统 MUST 不再存在对旧包或旧限定名的引用

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

### Requirement: Handle nil errors safely in failure helpers

系统 MUST 保证失败响应 helper 在收到 nil error 时不会 panic，并返回统一失败信封。

#### Scenario: Fail helper receives nil error
- **Given** controller 或 middleware 调用失败响应 helper 时传入 nil error
- **When** 系统将错误转换为响应
- **Then** 系统不得 panic
- **Then** 系统必须返回 HTTP 500
- **Then** 响应 JSON 必须包含 `success: false`、内部错误业务码 `90000` 和对外安全消息 `internal server error`

### Requirement: Keep response contract constants centralized

系统 MUST 在 `common/response` 中集中维护标准成功消息、内部错误消息和业务码常量，避免在响应构造函数中重复硬编码同一契约值。

#### Scenario: Success messages use response constants
- **Given** controller 调用 `response.OK` 或 `response.Created`
- **When** 系统构造成功响应信封
- **Then** `OK` 响应必须使用统一的 `ok` 消息
- **Then** `Created` 响应必须使用统一的 `created` 消息

#### Scenario: Internal error message uses response constant
- **Given** 系统包装非预期错误或 nil error
- **When** 系统构造内部错误失败信封
- **Then** 响应必须使用统一的 `internal server error` 对外安全消息

### Requirement: Response naming documentation remains compatible
响应契约相关命名标准化 SHALL 统一规格和文档中的响应码命名表达，但不得改变响应信封字段、响应 `code` 数值、错误映射、validation error details 或 panic recovery 响应行为。

#### Scenario: Response code names are normalized in specs
- **WHEN** 实现修正规格或文档中的响应码名称表达
- **THEN** 规格 MUST 明确对外响应 `code` 仍使用当前数字枚举，且 Go 常量名或语义标签不得改变 JSON payload 结构

#### Scenario: Response envelope is preserved
- **WHEN** 命名标准化涉及 `common/response` 或 controller 响应引用
- **THEN** HTTP 响应 MUST 继续使用 `success`、`code`、`message`、`data` 和既有错误字段约定

### Requirement: Centralize shared authentication response message
系统 SHALL 在 `common` 的响应契约中集中维护通用认证失败公开文案，使认证中间件、服务调用方和测试能够复用同一 message 常量。

#### Scenario: Authentication failure message is reusable
- **WHEN** common 认证中间件构造未认证、token 非法或 token 过期失败响应
- **THEN** 响应 message MUST 来自 `common` 的共享认证失败 message 常量
- **THEN** message 值 MUST 保持为 `登录状态无效或已过期，请重新登录`

#### Scenario: Response codes remain stable
- **WHEN** 共享认证失败 message 常量被用于失败响应
- **THEN** 未认证响应业务码 MUST 保持为 `20000`
- **THEN** token invalid 响应业务码 MUST 保持为 `20001`
- **THEN** token expired 响应业务码 MUST 保持为 `20002`
- **THEN** 响应 envelope 结构 MUST 保持不变

#### Scenario: Internal error and success messages remain centralized
- **WHEN** 实现迁移或整合 response message 常量
- **THEN** `ok`、`created` 和 `internal server error` 的常量来源 MUST 继续位于 `common/response`
- **THEN** 成功响应和内部错误响应的 message 值 MUST 保持不变

### Requirement: Return external user identity fields in user profile data
系统 SHALL 保持用户相关 API 的 `common/response.Envelope` 外层响应契约不变，同时用户资料 `data` MUST 只公开外部用户身份字段和非敏感资料字段。用户资料响应 MUST 返回 `user_id` 和 `username`，MUST NOT 返回内部数据库 `id`、`email`、`password` 或密码哈希。

#### Scenario: Create user response exposes external identity
- **Given** 用户创建成功
- **When** controller 输出成功响应信封
- **Then** 响应信封 MUST 保持 `success`、`code`、`message`、`data` 结构
- **Then** `data` MUST 包含 `user_id` 和 `username`
- **Then** `data` MUST NOT 包含 `id`、`email`、`password` 或密码哈希

#### Scenario: Query user response exposes external identity
- **Given** 用户资料查询成功
- **When** controller 输出成功响应信封
- **Then** 响应信封 MUST 保持 `success`、`code`、`message`、`data` 结构
- **Then** `data` MUST 包含 `user_id` 和 `username`
- **Then** `data` MUST NOT 包含 `id`、`email`、`password` 或密码哈希

#### Scenario: User errors preserve envelope shape
- **Given** 用户创建、查询或登录请求失败
- **When** 系统返回错误响应
- **Then** 响应 MUST 继续使用统一失败响应信封
- **Then** 响应 MUST NOT 通过错误消息泄露内部数据库 `id`、密码明文、完整密码 hash 或底层数据库细节
