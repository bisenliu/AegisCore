## ADDED Requirements

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

## MODIFIED Requirements

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
