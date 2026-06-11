# Rename feature infra to infrastructure

## What

将 user-service 内 feature 的 `infra` 目录重命名为 `infrastructure`，贴合最终目录结构和更完整的分层命名。

目标结构：

```text
user-service/internal/features/
  user/
    api/
    application/
    domain/
    infrastructure/postgres/
    transport/http/
    module.go
  auth/
    api/
    application/
    domain/
    infrastructure/postgres/
    infrastructure/redis/
    transport/http/
    module.go
```

本变更迁移：

- `user-service/internal/features/user/infra/postgres/` -> `user-service/internal/features/user/infrastructure/postgres/`。
- `user-service/internal/features/auth/infra/postgres/` -> `user-service/internal/features/auth/infrastructure/postgres/`。
- `user-service/internal/features/auth/infra/redis/` -> `user-service/internal/features/auth/infrastructure/redis/`。
- 所有 import path、import alias、feature module Fx provider 引用、测试和当前长期文档引用。

迁移后 adapter 的 Go package name 可继续保持 `postgres` 和 `redis`。本变更只调整目录路径和分层称谓，不改变 adapter 对 application ports 的实现方式。

## Why

`infra` 当前承载的是 feature 内部的基础设施 adapter，包括 Ent/PostgreSQL adapter、Redis session adapter 和 predicate 构造。目录名改为 `infrastructure` 后，分层名称与 `application`、`transport/http`、`domain` 一样完整，也减少缩写造成的长期文档不一致。

这次变更只修正 feature 内基础设施层命名，不改变业务流程、HTTP API、持久化模型、Redis key、Ent predicate 或 Fx 依赖语义。完成后，feature-first 结构会更接近最终文档形态：

- `transport/http` 负责 HTTP 边界。
- `application` 负责用例编排和消费侧 ports。
- `domain` 负责领域实体和值对象。
- `infrastructure/*` 负责数据库、Redis 等服务拥有资源的 adapter。

## Scope

包括：

- 移动 user feature PostgreSQL adapter 到 `internal/features/user/infrastructure/postgres`。
- 移动 auth feature PostgreSQL adapter 到 `internal/features/auth/infrastructure/postgres`。
- 移动 auth feature Redis adapter 到 `internal/features/auth/infrastructure/redis`。
- 更新 user/auth feature module 中的 infrastructure provider imports。
- 更新所有 Go import path、import alias、测试引用和 package qualifier。
- 更新 `AGENTS.md`、`docs/ARCHITECTURE.md`、`docs/DEVELOPMENT.md` 中当前结构、关键入口、依赖规则和开发约定。
- 运行 `gofmt` 格式化受影响 Go 文件。

不包括：

- 不把 application 层 ports 或 repository 接口移入 `infrastructure`。
- 不改变 Ent/PostgreSQL query、predicate 构造、Redis key、session serialization 或 token version 访问逻辑。
- 不改变 service、commands、queries、ports、result、domain entity 或 domain error 语义。
- 不改变 HTTP route、request/response DTO、response envelope、状态码或错误码。
- 不改变 Ent schema、Ent generated code、Atlas migration 或数据库结构。
- 不改变 JWT、session、token version 或认证状态机语义。
- 不新增横向 `internal/repository`、`internal/store`、`internal/infrastructure` 或 `internal/shared` 包。
- 不把 feature-owned infrastructure adapter 移入 `common`、`internal/providers` 或 `internal/integration`。
- 不新增 `openspec/` 或 `docs/opsx/` 工件。

## Acceptance Criteria

- `user-service/internal/features/user/infrastructure/postgres/` 存在并承载原 user PostgreSQL adapter。
- `user-service/internal/features/auth/infrastructure/postgres/` 存在并承载原 auth PostgreSQL adapter。
- `user-service/internal/features/auth/infrastructure/redis/` 存在并承载原 auth Redis adapter。
- `user-service/internal/features/user/infra/` 和 `user-service/internal/features/auth/infra/` 不再存在。
- 当前 Go 代码中不再导入 `github.com/aegiscore/user-service/internal/features/user/infra/postgres`、`github.com/aegiscore/user-service/internal/features/auth/infra/postgres` 或 `github.com/aegiscore/user-service/internal/features/auth/infra/redis`。
- 当前业务代码和长期规则文档中不再存在 `features/.*/infra` 业务引用。
- 旧 `userinfra`、`authinfra` 或含义不清的 `infra` import alias 不再用于 feature adapter imports；调用方使用能表达新目录的 alias，例如 `userpostgres`、`authpostgres`、`authredis` 或同等清晰命名。
- Adapter 行为保持不变：仍实现 application-owned ports，仍使用 Ent 或 Redis 访问服务拥有资源，仍在 adapter 内完成存储模型转换和存储错误转换。
- HTTP API、配置 key、Ent schema、migration 和 generated code 无变更。
- 从仓库根目录运行 `rg -n 'features/.*/infra(/|$)|/infra"|/infra/|infra/(postgres|redis)|internal/features/.*/infra(/|$)|\\buserinfra\\b|\\bauthinfra\\b' user-service AGENTS.md docs/ARCHITECTURE.md docs/DEVELOPMENT.md docs/TESTING.md` 不发现当前业务引用。
- 在 `user-service/` 下运行 `go test ./...` 通过。
