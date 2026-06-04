## Why

当前 `user-services` 已使用 Zap 和 trace-id 记录部分业务错误，但关键异常分支的日志覆盖不足，部分日志缺少必要业务上下文，导致排查认证、用户查询、用户创建和会话异常时检索维度不稳定。

需要在继续复用 `common/logger` 的前提下补充缺失日志和关键上下文，提升问题定位、监控告警和审计排查效率，同时避免密码、令牌和哈希等敏感信息进入日志。

## What Changes

- 新增用户服务日志可观测能力，约束业务日志、错误日志、脱敏字段、错误级别和关键上下文。
- 补充 `user-services` 关键流程日志字段，包括 `trace-id`、用户标识、用户名、会话标识、请求参数摘要、错误原因和错误堆栈。
- 优化共享 HTTP 请求日志中间件 `RequestLogger`，按响应状态区分日志级别，并在请求日志字段中记录 `user_id`，无法获取认证用户时记录为 `anonymous`。
- 补充认证、用户创建、用户查询、用户列表、密码修改、token 刷新、登出和会话仓储异常分支的日志。
- 优化现有日志中过于笼统、字段不足或可能暴露敏感信息的记录方式。
- 保持现有 HTTP API、错误码、响应信封、数据库 schema 和运行时配置兼容。

## Capabilities

### New Capabilities
- `user-service-logging-observability`: 约束 `user-services` 的业务日志补充、错误日志、脱敏策略、关键上下文和告警检索友好性。

### Modified Capabilities
- `shared-infrastructure`: 明确用户服务应复用 `common/logger` context API、Zap 日志和 `trace-id` 日志关联能力，不新增独立日志实现。

## Impact

- 受影响代码：`common/middleware/logging.go`、`user-services/internal/service/`、`user-services/internal/controller/`、`user-services/internal/repository/redis/`、必要时 `common/logger/` 与 `common/middleware/`。
- 受影响能力：`shared-infrastructure`、`api-response-contract`、`request-validation`、`user-profile-query`、`user-profile-create`、`user-authentication`、`user-session-control`。
- API 兼容性：不改变 HTTP 路由、请求字段、响应信封、错误码或状态码。
- 数据兼容性：不修改 Ent schema、SQL migration 或 Redis key 格式。
- 安全影响：日志不得记录密码、明文 token、refresh token、password-change token、password hash、完整 Authorization header 或其他凭证。
