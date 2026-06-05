## Why

用户服务停止时会依次关闭 `user_db` 与 `common_db` 两个 Ent client；当前逻辑在第一个 close 返回错误时直接返回，可能丢失第二个 close 错误，降低停机故障排查的可观察性。

## What Changes

- 调整用户服务 Ent client 停止 lifecycle 的错误处理，确保两个具名 Ent client 都会尝试关闭。
- 当多个 Ent client close 同时失败时，返回的停止错误必须保留每个失败 client 的具名上下文与底层错误。
- 不改变 Ent client 创建数量、Fx named injection、PostgreSQL 配置路径或 repository 注入行为。

## Capabilities

### New Capabilities

- 无。

### Modified Capabilities

- `shared-infrastructure`: 细化服务侧具名 Ent client 停止 lifecycle 的错误保留要求。

## Impact

- 受影响代码：`user-services/internal/bootstrap/ent.go`。
- 受影响 capability：`shared-infrastructure`。
- 外部兼容性：不改变 HTTP API、错误码、配置、数据模型、Ent schema 或 migration。
- 运行时影响：仅改变 Fx app 停止阶段 Ent client close 错误的聚合或具名记录方式，使多个 close 错误不会互相覆盖。
