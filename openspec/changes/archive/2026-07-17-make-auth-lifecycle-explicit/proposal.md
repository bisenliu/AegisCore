## Why

当前 auth Redis infrastructure 中的 session purge pool 与 token-version localcache 仍通过 Fx lifecycle 和为关闭顺序注入的 Redis 伪依赖表达资源所有权，导致组件边界混入装配框架语义，也让关闭顺序依赖隐式且难以在非 Fx 场景中验证。`remove-fx-from-auth-adapters` 已完成后，auth adapter 可以改为普通 Go 组件，由服务装配层显式持有、显式关闭并测试关闭顺序。

## What Changes

- **BREAKING**：`NewSessionPurgePool` 删除旧 Params API、`fx.Lifecycle` 依赖和仅用于 ordering 的 named Redis dependency，只接收真实运行依赖并返回可显式停止的 pool。
- **BREAKING**：token-version cache 构造结果改为显式暴露 validator/cache、stats 和幂等 `Close`，enabled、disabled 和 direct 模式都提供一致关闭契约。
- auth Fx module 只负责登记新构造 API 和生命周期 hook，不再把 lifecycle ordering 下沉到 Redis adapter 构造器内部。
- auth 测试覆盖 `Stop`/`Close` 幂等、超时、drain、关闭后不泄漏 goroutine，以及 session purge pool 必须先于 Redis client 关闭。
- `auth-session-management` 规格改为以资源所有权和关闭顺序约束替代构造器内部 Fx lifecycle ordering。

## Capabilities

### New Capabilities

- 无。

### Modified Capabilities

- `auth-session-management`：认证会话相关 Redis 组件的资源生命周期改为显式构造、显式关闭，并要求 auth 自有资源先于共享 Redis client 关闭。

## Impact

- 影响代码：`user-service/internal/features/auth/infrastructure/redis` 中 session purge pool、token-version localcache、相关构造器和测试；auth provider/Fx module 中对应装配代码。
- 影响规格：更新 `openspec/specs/auth-session-management/spec.md` 的 change delta，明确 auth 资源所有权和关闭顺序。
- 影响依赖：从 auth Redis infrastructure 正式代码中移除 `go.uber.org/fx`、`go.uber.org/dig`、`fx.Lifecycle` 和 `name:"cache_redis"` ordering-only dependency。
- 不影响 HTTP API、数据库 schema、OpenAPI、token/session 删除语义、token version 查询与失效语义、cache TTL/容量或指标语义。
- 不改变共享 Redis client 所有权；auth 组件只关闭自身 goroutine、queue、localcache 等自有资源。
