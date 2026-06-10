## ADDED Requirements

### Requirement: Map token version infrastructure failures to server errors

API 响应契约 SHALL 保证认证中间件的 token version 基础设施故障使用统一失败信封返回服务端故障响应。Redis、PostgreSQL 或缓存回填等非预期依赖错误 MUST NOT 使用 token invalid 业务码；系统 MUST 返回 HTTP 500、业务码 `90000` 和对外安全消息 `internal server error`，并且 MUST NOT 执行受保护业务 handler。明确的 token version mismatch MUST 继续使用 HTTP 401 和 token invalid 业务码。

#### Scenario: Token version mismatch keeps token invalid response
- **Given** 请求携带签名有效且未过期的 Access Token
- **Given** token version validator 返回 token version mismatch 语义
- **When** 认证中间件处理该请求
- **Then** 系统 MUST 返回 HTTP 401
- **Then** 响应 JSON MUST 包含 `success: false` 和 token invalid 业务码
- **Then** 响应 MUST NOT 包含业务 `data`

#### Scenario: Token version infrastructure error returns internal error envelope
- **Given** 请求携带签名有效且未过期的 Access Token
- **Given** token version validator 返回 Redis、PostgreSQL 或缓存回填相关基础设施错误
- **When** 认证中间件处理该请求
- **Then** 系统 MUST 返回 HTTP 500
- **Then** 响应 JSON MUST 包含 `success: false`、`code: 90000` 和 `message: internal server error`
- **Then** 响应 MUST NOT 包含业务 `data`
- **Then** 受保护业务 handler MUST NOT 执行

#### Scenario: Infrastructure failure response hides dependency details
- **Given** token version validator 返回包含 Redis 地址、SQL 错误或底层依赖信息的错误
- **When** 认证中间件返回失败响应
- **Then** 响应 message MUST 使用对外安全消息
- **Then** 响应 MUST NOT 暴露 Redis key、数据库 DSN、SQL 语句、JWT 内容或底层依赖错误文本
