## ADDED Requirements

### Requirement: Log authentication decisions with caller-provided logger

系统 MUST 要求 Gin 认证中间件由调用方显式传入 Zap logger。认证中间件 MUST 使用该 logger 记录白名单放行、认证头缺失、认证头格式错误、空 bearer token 和 token 校验失败等认证决策日志，并 MUST 继续通过请求 `context.Context` 保留 `trace-id` 日志字段。认证失败日志 MUST NOT 记录 token 原文。

#### Scenario: Log whitelisted path with provided logger
- **Given** 调用方使用显式 Zap logger 构造认证中间件
- **Given** auth 配置白名单包含 `/healthz`
- **When** 请求路径为 `/healthz`
- **Then** 认证中间件 MUST 使用调用方传入的 logger 记录白名单放行日志
- **Then** 该请求 MUST 继续执行后续 handler

#### Scenario: Log authentication failure with provided logger
- **Given** 调用方使用显式 Zap logger 构造认证中间件
- **Given** 请求路径不在 auth 白名单中
- **When** 调用方未携带有效 `Authorization: Bearer <token>` 请求头
- **Then** 认证中间件 MUST 使用调用方传入的 logger 记录认证失败日志
- **Then** 认证失败日志 MUST NOT 包含 token 原文
- **Then** 系统 MUST 返回现有 HTTP 401 失败响应信封

#### Scenario: Preserve trace id in authentication logs
- **Given** 请求 context 中存在 trace id
- **Given** 调用方使用显式 Zap logger 构造认证中间件
- **When** 认证中间件输出认证相关日志
- **Then** 日志 MUST 包含请求 context 对应的 `trace-id` 字段
