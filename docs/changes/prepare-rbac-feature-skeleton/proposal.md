# Prepare RBAC feature skeleton

## What

为未来 RBAC 能力预留最小 feature 边界，在用户服务内新增 `role` 和 `permission` 两个 feature skeleton。

目标结构：

```text
user-service/internal/features/
  role/
    README.md or doc.go
  permission/
    README.md or doc.go
```

本变更只建立未来 feature 的所有权边界和准入说明。每个 feature 只添加最小 README 或 package doc，说明后续角色、权限模型、use case、HTTP transport、infrastructure adapter 和 Fx module 的放置规则。

## Why

用户服务当前稳定 feature 包括 `user` 和 `auth`。后续 RBAC 通常会同时影响用户身份、认证会话、权限校验、数据库模型和 HTTP API，如果没有提前标注 feature 边界，后续实现容易把角色权限代码放入 auth、user、`internal/shared` 或横向 service/repository 目录中。

先建立 `role` 与 `permission` 的最小 skeleton，可以让后续变更在正确 feature 下继续扩展，同时避免现在过早引入路由、Ent schema、授权逻辑或空 application/domain/infrastructure 包。

## Scope

包括：

- 新增 `user-service/internal/features/role`。
- 新增 `user-service/internal/features/permission`。
- 每个 feature 只添加 README 或最小 package doc，说明这是 future feature skeleton。
- 文档说明 role feature 未来负责角色聚合、角色生命周期和角色相关应用用例。
- 文档说明 permission feature 未来负责权限定义、权限查询和权限边界规则。
- 更新 `docs/ARCHITECTURE.md`，把 role 和 permission 标注为 future feature skeleton。
- 如 `AGENTS.md` 的 Repository Shape 或 Current Feature Areas 需要同步，补充相同 future skeleton 说明。

不包括：

- 不实现角色、权限、RBAC policy、授权中间件或认证授权逻辑。
- 不注册 HTTP route。
- 不新增 controller、request/response DTO、application service、ports、domain model、infrastructure adapter 或 Fx module。
- 不新增 Ent schema、Ent generated code、Atlas migration 或数据库表。
- 不修改 user/auth feature 行为、JWT claim、session lifecycle、token version 策略或响应契约。
- 不新增横向 `internal/rbac`、`internal/authorization`、`internal/shared`、`internal/service`、`internal/repository` 或 `common` 业务 helper。
- 不重新新增 `openspec/` 或 `docs/opsx/` 工件。

## Acceptance Criteria

- `user-service/internal/features/role` 存在，并只包含最小 README 或 package doc。
- `user-service/internal/features/permission` 存在，并只包含最小 README 或 package doc。
- 新增 skeleton 不产生未使用 Go 代码；如果使用 Go package doc，不定义常量、变量、接口、struct 或函数。
- 没有新增 HTTP route、Fx provider、Ent schema、migration、generated code、数据库表或授权逻辑。
- `docs/ARCHITECTURE.md` 将 role 和 permission 标注为 future feature skeleton，而不是当前稳定业务能力。
- 如更新 `AGENTS.md`，其规则与 `docs/ARCHITECTURE.md` 保持一致。
- 目录和文档说明后续实现仍需按 `application/`、`domain/`、`transport/http/`、`infrastructure/*` 和 `fx.go` 分层扩展。
- 本变更没有新增 `openspec/` 或 `docs/opsx/`。
