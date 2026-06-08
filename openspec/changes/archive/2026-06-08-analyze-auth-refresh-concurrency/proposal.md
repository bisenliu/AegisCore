## Why

`AuthService.Refresh` 已经承担 Refresh Token 校验、轮转签发、新旧 Redis 会话写入和失败补偿等多项职责，当前实现虽然满足基本顺序约束，但流程集中在单个方法中，后续维护者难以快速判断失败路径、补偿路径和安全边界。

同时，启用 Refresh Token 轮转后，旧会话校验、新会话创建和旧会话删除目前跨多个 Redis 命令完成；在并发刷新、网络抖动或部分命令失败场景下，存在旧 Refresh Token 被重复使用、多个新 Refresh Token 同时可用或会话状态短暂不一致的风险，需要明确并补强并发安全策略。

## What Changes

- 重构 `AuthService.Refresh` 的内部编排结构，将请求规范化、Refresh Token claims 解析、会话校验、非轮转签发、轮转签发和轮转失败补偿拆分为职责清晰的内部辅助方法。
- 保持现有 `/api/v1/auth/refresh` HTTP 契约、响应信封、错误语义、JWT claims、Redis key 格式和配置项兼容。
- 为 Refresh Token 轮转引入明确的原子性保障方案，将旧会话仍存在校验、新会话创建和旧会话撤销收敛到 Redis 事务、Lua 脚本或等价的 repository 原子操作中。
- 增加并发刷新和部分失败路径测试，覆盖同一个旧 Refresh Token 并发刷新时只允许一次轮转成功，以及旧会话撤销失败时不得暴露无一致性保障的新 Refresh Token。
- 不引入服务进程内互斥锁作为主要安全边界，因为该服务可水平扩展，进程内锁无法覆盖多实例并发刷新。

## Capabilities

### New Capabilities

- 无。

### Modified Capabilities

- `user-session-control`: 补强 Refresh Token 轮转的实现边界、可维护性要求和严格重放防护下的原子轮转语义。

## Impact

- 影响代码主要位于 `user-services/internal/service/auth_service.go`、`user-services/internal/service/auth_sessions.go`、`user-services/internal/repository/auth_session_repository.go` 和 `user-services/internal/repository/redis/auth_session_repository.go`。
- 影响测试主要位于 `user-services/internal/service/auth_service_test.go`、`user-services/internal/service/auth_components_test.go` 和 `user-services/internal/repository/redis/auth_session_repository_test.go`。
- 不改变公开 HTTP 路由、请求体、响应体、错误码、配置字段、Ent schema、Atlas migration 或 Redis key 命名。
- Redis 会话仓储需要新增或调整原子轮转能力；实现应保持在 repository 抽象和 Redis 实现边界内，service 层不得直接依赖 Redis client 或 Lua 脚本细节。
