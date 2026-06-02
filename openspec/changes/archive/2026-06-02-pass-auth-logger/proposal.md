## Why

当前 `common/middleware.Auth` 在认证失败或白名单放行时直接使用 `common/logger` 默认上下文 API 输出日志，调用方无法显式传入用户服务启动阶段注入的 `*zap.Logger`。这会让认证中间件与全局 logger 状态耦合，不利于用户服务在 Fx 依赖图中保持日志依赖显式、可测试和一致。

## What Changes

- 调整共享认证中间件 `Auth` 的构造签名，使调用方显式传入 `*zap.Logger`。
- 认证中间件内部使用传入 logger 输出认证相关日志，并继续保留 `trace-id` 等请求上下文字段。
- 修改用户服务启动组装处，使用 Fx 注入的 logger 调用认证中间件。
- 修改认证中间件单元测试调用处，提供测试 logger，保持现有认证行为断言不变。
- 不改变 JWT 解析、认证失败响应、HTTP 状态码、错误码、白名单规则或用户 ID 上下文传播行为。

## Capabilities

### New Capabilities

- 无。

### Modified Capabilities

- `user-authentication`: 认证中间件必须由调用方显式传入 logger，并使用该 logger 记录认证日志，避免依赖隐式全局 logger。
- `http-service-runtime`: 用户服务运行时注册认证中间件时必须传入 Fx 注入的 Zap logger。

## Impact

- 受影响代码：`common/middleware/auth.go`、`common/middleware/auth_test.go`、`user-services/internal/bootstrap/bootstrap.go`。
- API 兼容性：HTTP API 响应契约不变；Go 代码层面的 `middleware.Auth` 函数签名发生变化，需要同步更新调用处。
- 配置与数据模型：不引入新配置，不修改数据库 schema 或 migration。
- 依赖：继续使用现有 Zap logger，不新增第三方依赖。
