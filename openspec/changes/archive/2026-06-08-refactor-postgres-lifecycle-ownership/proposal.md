## Why

用户服务同时创建 `user_db` 与 `common_db` PostgreSQL 连接池，并基于这些连接池创建 Ent clients。当前 PostgreSQL pool 的创建、Fx lifecycle 注册、失败回滚和 Ent client 关闭分散在 `common/runtime/datastorefx` 与 `user-services/internal/bootstrap` 两层，导致连接池所有权不清晰，并可能在组合运行时中依赖 `database/sql.DB.Close` 的幂等性来掩盖重复关闭风险。

现在需要在共享基础设施能力中明确 PostgreSQL 连接池的单一 lifecycle owner，保留启动 ping、停止释放和失败回滚语义，同时避免用户服务 bootstrap 与 Ent client lifecycle 对同一底层资源形成重复 close 责任。

## What Changes

- 明确 `shared-infrastructure` 中命名 PostgreSQL `*sql.DB` 连接池的 lifecycle 所有权边界：创建、启动 ping、停止 close 和失败回滚必须由同一层统一处理。
- 调整用户服务 `user_db` 与 `common_db` 的批量创建/回滚路径，确保第二个 pool 创建失败时已创建资源被释放且不会留下模糊的后续 lifecycle close 责任。
- 调整 Ent client lifecycle，使 Ent client 清理与底层共享 `*sql.DB` pool 关闭职责不重复；停止错误仍需保留具名 Ent client 或具名 pool 的上下文。
- 补充测试覆盖组合运行时中 PostgreSQL pool 与 Ent clients 的启动、停止和失败回滚行为。
- 非目标：不修改 HTTP API、响应信封、配置路径、环境变量、Redis lifecycle、Ent schema 或 Atlas migration。

## Capabilities

### New Capabilities

- 无。

### Modified Capabilities

- `shared-infrastructure`: 明确具名 PostgreSQL pool 与基于该 pool 的 Ent client 在 Fx lifecycle 中的所有权、关闭和失败回滚要求。

## Impact

- 影响代码：`common/runtime/datastorefx/postgres.go`、`user-services/internal/bootstrap/postgres.go`、`user-services/internal/bootstrap/ent.go` 以及相关测试。
- 影响能力：主要修改 `shared-infrastructure`；`http-service-runtime` 作为消费者继续声明 `cache_redis`、`user_db` 和 `common_db`，但外部启动命令、路由和 graceful shutdown 契约不变。
- 兼容性：不改变 YAML key、`AEGISCORE_` 环境变量、`postgres.<name>` 命名实例、Fx named injection 名称、HTTP API、错误码或数据模型。
- 依赖与系统：不新增外部依赖，不触发数据库 schema 变更，不需要 migration。
