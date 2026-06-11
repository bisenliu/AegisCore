# Rename feature app to application

## What

将 user-service 内 feature 的 `app` 应用层目录重命名为 `application`，贴合最终目录结构和分层命名。

目标结构：

```text
user-service/internal/features/
  user/
    api/
    application/
    domain/
    infra/postgres/
    transport/http/
    module.go
  auth/
    api/
    application/
    domain/
    infra/postgres/
    infra/redis/
    transport/http/
    module.go
```

本变更迁移：

- `user-service/internal/features/user/app/` -> `user-service/internal/features/user/application/`。
- `user-service/internal/features/auth/app/` -> `user-service/internal/features/auth/application/`。
- Go package name 从 `app` 改为 `application`。
- 所有 import path、import alias、Fx provider annotation、测试和当前长期文档引用。

迁移后仍保留现有 service、commands、queries、ports、result、credential/session/token component 的语义和文件职责。

## Why

`app` 当前承载的是应用层用例编排、command/query、端口接口和 transport-neutral 结果模型。目录名改为 `application` 后，分层名称更完整，也能减少它与 `bootstrap/app.go`、Fx `AppModule` 或局部变量 `app` 的命名混淆。

这次变更只修正 feature 内应用层命名，不改变 HTTP API、业务流程、持久化模型或运行时依赖。完成后，feature-first 结构会更接近最终文档形态：

- `transport/http` 负责 HTTP 边界。
- `application` 负责用例编排和消费侧 ports。
- `domain` 负责领域实体和值对象。
- `infra/*` 负责数据库或 Redis adapter。

## Scope

包括：

- 移动 user feature 应用层目录到 `internal/features/user/application`。
- 移动 auth feature 应用层目录到 `internal/features/auth/application`。
- 将移动后的 Go 文件 package declaration 从 `package app` 改为 `package application`。
- 更新 user/auth feature module 中对应用层 service 和 ports 的引用。
- 更新 user/auth transport HTTP controller、mapper、controller test 中对 command/query/result/service 的引用。
- 更新 user/auth infra PostgreSQL 和 Redis adapter、adapter test 中对 ports/result/component 的引用。
- 更新 `internal/providers/routes_test.go` 等跨 feature 测试中的应用层 import。
- 更新 `AGENTS.md`、`docs/ARCHITECTURE.md`、`docs/DEVELOPMENT.md` 中当前结构、关键入口、依赖规则和开发约定。
- 运行 `gofmt` 格式化受影响 Go 文件。

不包括：

- 不拆分 `commands.go`、`queries` 或 ports 到子目录。
- 不改变 service、commands、queries、ports、result 的字段、方法、错误语义或业务流程。
- 不改变 HTTP route、request/response DTO、response envelope、状态码或错误码。
- 不改变 Ent schema、Ent generated code、Atlas migration 或数据库结构。
- 不改变 Redis key、PostgreSQL query、JWT、session 或 token version 语义。
- 不将 feature 应用层代码移动到 `common`、`internal/shared` 或服务级 `providers`。
- 不新增 `openspec/` 或 `docs/opsx/` 工件。

## Acceptance Criteria

- `user-service/internal/features/user/application/` 存在并承载原 user 应用层代码。
- `user-service/internal/features/auth/application/` 存在并承载原 auth 应用层代码。
- `user-service/internal/features/user/app/` 和 `user-service/internal/features/auth/app/` 不再存在。
- 移动后的应用层 Go 文件统一使用 `package application`。
- 当前 Go 代码中不再导入 `github.com/aegiscore/user-service/internal/features/user/app` 或 `github.com/aegiscore/user-service/internal/features/auth/app`。
- 当前长期文档不再把 user/auth feature 应用层描述为 `app/`。
- 旧 `userapp`、`authapp` import alias 不再出现在当前 Go 代码中，调用方使用能表达新目录的 alias，例如 `userapplication`、`authapplication`。
- Service、commands、queries、ports 和 result 语义保持不变。
- HTTP API、配置 key、Ent schema、migration 和 generated code 无变更。
- 从仓库根目录运行 `rg -n '/app"|features/(user|auth)/app(/|$)|^package app$|\buserapp\b|\bauthapp\b' user-service AGENTS.md docs/ARCHITECTURE.md docs/DEVELOPMENT.md docs/TESTING.md` 不发现当前业务引用。
- 在 `user-service/` 下运行 `go test ./...` 通过。
