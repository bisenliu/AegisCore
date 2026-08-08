## Why

认证会话全量撤销后的后台清理任务会在 `Task.Fields` 中携带完整 Redis purge key 和 refresh session key prefix；当 workerpool 记录 error 或 panic 日志时，这些字段会原样进入集中日志，暴露 Redis namespace、用户 UUID hash tag 和可拼装会话 key 的材料。

这违反了 `common/runtime/workerpool` 对后台任务日志字段的安全契约，增加日志访问面中的内部拓扑和用户标识泄露风险，需要在实施前明确认证会话与共享日志边界的约束。

## What Changes

- 收敛 auth refresh session 后台 purge 任务的 workerpool 日志字段，不再记录完整 Redis key、Redis key prefix、Redis namespace、用户 UUID hash tag 或可拼装会话 key 的字段。
- 保留后台任务执行所需的 Redis key material 仅在闭包内部使用，不作为 `Task.Fields` 或其他错误日志字段输出。
- 日志定位字段只保留稳定操作名、批量大小、cut time，以及确需关联时的不可逆 opaque 标识。
- 增加失败和 panic 路径的日志安全测试，验证日志中不包含 `purge_key`、`session_prefix`、Redis namespace、`{user_uuid}` 或可拼装 session key material。
- 不保留旧日志字段，不提供兼容字段名或兼容分支。

## Capabilities

### New Capabilities

- 无。

### Modified Capabilities

- `auth-session-management`: 认证会话撤销后的后台 Redis purge 失败和 panic 日志必须避免暴露完整 Redis key、key prefix、用户 UUID hash tag 或可拼装会话 key 的材料。
- `shared-platform-primitives`: workerpool 的消费端任务字段必须遵守共享日志安全契约，后台任务失败和 panic 日志不得通过 `Task.Fields` 泄露完整 Redis key 或其他敏感值。

## Impact

- 影响代码：`user-service/internal/features/auth/infrastructure/redis/refresh_session_store.go` 的 purge workerpool task 字段构造，以及相关 auth Redis session 测试。
- 影响共享契约：对 `common/runtime/workerpool` 既有 `Task.Fields` 安全约束进行消费端落实，必要时补充规格说明；不改变 workerpool API。
- 影响观测：相关 error/panic 日志字段会移除 `purge_key`、`session_prefix` 和明文 Redis key material；排障时依赖稳定任务名、批量大小、cut time 和不可逆关联标识。
- 不影响外部 HTTP API、OpenAPI、数据库 schema、Atlas migration、Redis key 实际存储格式或部署资产。
