## Why

当前受保护路由的 token version 校验会把 Redis 或 PostgreSQL 抖动等基础设施错误映射为 token invalid，导致真实依赖故障被伪装成认证失败。该行为可能触发大面积用户重新登录，也会削弱故障定位和告警能力，因此需要在认证边界区分版本不一致与基础设施不可用。

## What Changes

- 调整 `user-session-control` 的 token version 校验契约：只有明确的 token version mismatch 才返回认证失败语义。
- Redis token version cache 读取、PostgreSQL 回源或缓存回填等基础设施错误必须 fail-closed，但对外映射为服务端故障响应，而不是 token invalid。
- 为该路径补充可观察性要求：基础设施错误需要保留底层错误上下文并记录告警级别日志，日志不得泄露 token 或敏感 claims。
- 保持现有认证成功、token version mismatch、token 缺失/非法/过期的 HTTP 路由、响应信封和业务码语义不变。

## Capabilities

### New Capabilities

- 无。

### Modified Capabilities

- `user-session-control`: 修改受保护路由和认证会话流程中的 token version 校验错误分类要求。
- `api-response-contract`: 明确 token version 基础设施错误不得使用 token invalid 业务码，必须按服务端故障响应输出统一失败信封。

## Impact

- 主要影响 `common/http/middleware/auth.go` 中认证中间件的 token version validator 错误映射，以及 `user-services/internal/features/auth/app` 内 token version validator/session resolver 返回的错误分类。
- 可能涉及 `user-services/internal/features/auth/infra/redis` 与 `user-services/internal/features/auth/infra/postgres` 中基础设施错误透传或包装方式，但不改变 Redis key、Ent schema、Atlas migration 或运行时配置。
- 外部 API 路由、请求体和成功响应不变；token version mismatch 继续返回 HTTP 401/token 专用认证失败，Redis/DB 抖动等非预期依赖错误改为 HTTP 503 或 HTTP 500 的统一失败信封。
- 需要补充单元测试覆盖 token version mismatch 与 Redis/DB 基础设施错误的不同映射，避免回归为统一 401。
