## Context

`common/middleware.Auth` 是共享 Gin 认证中间件，当前签名只接收 JWT service 和认证配置，内部通过 `common/logger` 的默认上下文 API 写认证日志。用户服务启动时已经通过 `common/infrastructure.Module` 在 Fx 图中注入 `*zap.Logger`，其他共享 HTTP 中间件如 recovery 和 request logger 也已经显式接收该 logger。

本变更涉及 `common/middleware` 与 `user-services/internal/bootstrap` 两个包，不改变 controller/service/repository 分层，不涉及 Ent schema、数据库迁移、Redis/PostgreSQL 连接或 HTTP 响应契约。

## Goals / Non-Goals

**Goals:**

- 让 `common/middleware.Auth` 由调用方显式传入 `*zap.Logger`。
- 让认证日志使用传入 logger 与当前请求 context 组合输出，继续包含 `trace-id`。
- 更新用户服务 Fx 组装和认证中间件测试调用处，保持编译与现有行为测试通过。
- 保持认证失败响应、JWT 校验、白名单放行和用户 ID 上下文传播语义不变。

**Non-Goals:**

- 不新增认证 API、登录流程、刷新 token 或权限模型。
- 不修改 JWT claims、错误码、响应 message 或 HTTP status。
- 不引入新的日志配置项、全局 logger 初始化策略或第三方依赖。
- 不修改数据库 schema、migration 或 Ent 生成代码。

## Decisions

- 将 `Auth` 签名调整为 `Auth(log *zap.Logger, jwtService *commonjwt.Service, cfg config.AuthConfig) gin.HandlerFunc`。这样与 `Recovery(params.Log)`、`RequestLogger(params.Log)` 的调用风格一致，调用处可清晰看到认证中间件依赖 logger。
- 在认证中间件内部通过 `logger.WithContext(log, ctx)` 输出日志，而不是直接调用包级 `logger.Debug/Error`。这样可以避免依赖隐式默认 logger，同时继续复用项目 logger context API 注入 `trace-id` 字段。
- 用户服务 `NewGinEngine` 使用现有 `GinParams.Log` 传入认证中间件，不新增 Fx provider 或参数结构。该方案保持依赖图最小变更，只更新中间件链调用表达式。
- 单元测试使用 Zap 测试 logger 或 no-op logger 调用 `Auth`，避免测试依赖全局默认 logger 状态。测试重点继续覆盖认证行为，不需要断言日志内容。

## Risks / Trade-offs

- `Auth` 是 Go 代码层面的破坏性签名变更。Mitigation: 当前仓库内调用处有限，实施时全量搜索 `Auth(` 并更新用户服务和测试调用处。
- 如果传入 nil logger，认证中间件可能在运行时 panic 或丢日志。Mitigation: 调用处使用 Fx 注入的非 nil logger，测试中显式提供 logger；无需为未声明需求添加兼容分支。
- 日志输出调用方式变化可能影响测试环境输出。Mitigation: 使用测试 logger/no-op logger，不改变 HTTP 行为断言。

## Migration Plan

1. 修改 `common/middleware/auth.go` 的函数签名和内部日志调用。
2. 修改 `user-services/internal/bootstrap/bootstrap.go` 的中间件注册调用，传入 `params.Log`。
3. 修改 `common/middleware/auth_test.go` 的测试调用，传入测试 logger。
4. 运行 `go test ./...` 验证 `common` 与 `user-services` 模块。

Rollback 时恢复 `Auth` 原签名并同步恢复调用处即可；该变更不涉及持久化数据或外部 HTTP API 迁移。

## Open Questions

- 无。
