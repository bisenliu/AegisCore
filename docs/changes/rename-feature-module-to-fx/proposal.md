# Rename feature module to fx

## What

将 user-service 内每个 feature 的 Fx 入口文件从 `module.go` 重命名为 `fx.go`，对齐已经稳定的服务级 Fx 文件命名和最终目录结构。

目标结构：

```text
user-service/internal/features/
  user/
    api/
    application/
    domain/
    infrastructure/postgres/
    transport/http/
    fx.go
  auth/
    api/
    application/
    domain/
    infrastructure/postgres/
    infrastructure/redis/
    transport/http/
    fx.go
```

本变更迁移：

- `user-service/internal/features/user/module.go` -> `user-service/internal/features/user/fx.go`。
- `user-service/internal/features/auth/module.go` -> `user-service/internal/features/auth/fx.go`。
- 当前长期文档中关于 feature Fx 入口的路径、目录结构说明和依赖规则。

迁移后每个 feature 仍通过导出的 `Module` 变量暴露 Fx module，调用方仍使用 `user.Module` 和 `auth.Module`。文件名变为 `fx.go`，Go package name、Fx module 名称、provider 列表和 provider 顺序保持不变。

## Why

仓库已经将服务级 Fx provider 统一放在 `internal/providers/fx.go`，共享 runtime 也使用 `config/fx.go`、`logger/fx.go`、`datastore/*_fx.go` 这类文件名表达 Fx 组装入口。Feature 内继续使用 `module.go` 会让同一种职责出现两套命名。

将 feature Fx 入口统一为 `fx.go` 后：

- 文件名直接表达这是 Fx wiring，而不是业务 module 或 Go module。
- feature 目录结构和服务级 provider 命名更一致。
- 后续新增 feature 时可以按 `api/application/domain/transport/http/infrastructure/*/fx.go` 的稳定形态落位。

这次变更只调整文件名和文档，不改变 HTTP API、业务流程、持久化模型、Redis/JWT 语义或 Fx 依赖图。

## Scope

包括：

- 移动 `user-service/internal/features/user/module.go` 到 `user-service/internal/features/user/fx.go`。
- 移动 `user-service/internal/features/auth/module.go` 到 `user-service/internal/features/auth/fx.go`。
- 保留 `package user`、`package auth`。
- 保留导出变量名 `Module`。
- 保留 Fx module 名称 `feature-user` 和 `feature-auth`。
- 保留 `fx.Provide` 列表、`fx.Annotate`、`fx.As`、provider 顺序和依赖关系。
- 更新 `AGENTS.md` 中 feature 目录结构、关键入口和 repository rules。
- 更新 `docs/ARCHITECTURE.md` 中 Feature-First Organization、Dependency Rules 和 runtime flow 中的 feature Fx 入口说明。
- 如 `docs/DEVELOPMENT.md` 存在 feature 入口或新增 feature 指引，更新为 `fx.go`。
- 运行 `gofmt` 格式化移动后的 Go 文件。

不包括：

- 不重命名导出的 `Module` 变量。
- 不改变 `bootstrap.AppModule` 中 feature module 的引用方式。
- 不改变 provider 顺序、provider 集合或 Fx annotation。
- 不拆分 feature module。
- 不新增 feature。
- 不移动 application、domain、transport 或 infrastructure 代码。
- 不改变 HTTP route、request/response DTO、response envelope、状态码或错误码。
- 不改变 Ent schema、Ent generated code、Atlas migration 或数据库结构。
- 不改变 Redis key、PostgreSQL query、JWT、session 或 token version 语义。
- 不新增 `openspec/` 或 `docs/opsx/` 工件。

## Acceptance Criteria

- `user-service/internal/features/user/fx.go` 存在，并承载原 user feature Fx wiring。
- `user-service/internal/features/auth/fx.go` 存在，并承载原 auth feature Fx wiring。
- `user-service/internal/features/user/module.go` 不再存在。
- `user-service/internal/features/auth/module.go` 不再存在。
- Feature packages 仍导出 `Module`，`bootstrap.AppModule` 仍通过现有 feature package 引用完成组装。
- `feature-user` 与 `feature-auth` 的 Fx provider 集合、provider 顺序和 `fx.As` annotation 与变更前一致。
- 当前长期文档不再把 feature Fx 入口描述为 `module.go`。
- 当前长期文档将 feature Fx 入口描述为 `fx.go`。
- HTTP API、配置 key、Ent schema、migration 和 generated code 无变更。
- 从仓库根目录运行 `rg -n 'features/.+/module[.]go|module[.]go' AGENTS.md docs/ARCHITECTURE.md docs/DEVELOPMENT.md user-service/internal/features` 不发现当前 feature Fx 入口引用。
- 在 `user-service/` 下运行 `go test ./...` 通过。
