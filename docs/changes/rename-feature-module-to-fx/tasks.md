# Tasks

## Implementation

- [x] 阅读 `docs/ARCHITECTURE.md`、`docs/DEVELOPMENT.md` 和本 change 的 `proposal.md`、`design.md`，确认本变更只做 feature Fx 入口文件命名迁移。
- [x] 将 `user-service/internal/features/user/module.go` 移动为 `user-service/internal/features/user/fx.go`。
- [x] 将 `user-service/internal/features/auth/module.go` 移动为 `user-service/internal/features/auth/fx.go`。
- [x] 确认 `user-service/internal/features/user/fx.go` 仍使用 `package user`。
- [x] 确认 `user-service/internal/features/auth/fx.go` 仍使用 `package auth`。
- [x] 确认 user feature 仍导出 `Module`，且 Fx module 名称仍为 `feature-user`。
- [x] 确认 auth feature 仍导出 `Module`，且 Fx module 名称仍为 `feature-auth`。
- [x] 确认 user feature provider 列表、provider 顺序、`fx.Annotate` 和 `fx.As` annotation 与迁移前一致。
- [x] 确认 auth feature provider 列表、provider 顺序、`fx.Annotate` 和 `fx.As` annotation 与迁移前一致。
- [x] 确认 `bootstrap.AppModule` 仍通过 `authfeature.Module` 和 `userfeature.Module` 组装 feature modules。
- [x] 运行 `gofmt -w user-service/internal/features/user/fx.go user-service/internal/features/auth/fx.go`。

## Documentation

- [x] 更新 `AGENTS.md` Repository Shape，使 user/auth feature 分层使用 `fx.go`。
- [x] 更新 `AGENTS.md` Key Entry Points，使 user/auth feature module 路径指向 `fx.go`。
- [x] 更新 `AGENTS.md` Repository Rules，使每个 feature 自己提供 Fx module 的说明指向 `features/<feature>/fx.go`。
- [x] 更新 `AGENTS.md` 依赖表，将 feature Fx 入口行从 `module.go` 改为 `fx.go`。
- [x] 更新 `docs/ARCHITECTURE.md` Feature-First Organization 表格，将 `module.go` 改为 `fx.go`。
- [x] 更新 `docs/ARCHITECTURE.md` Dependency Rules，将 feature Fx 入口行从 `module.go` 改为 `fx.go`。
- [x] 更新 `docs/ARCHITECTURE.md` 中任何 file-specific feature Fx 入口引用，使其指向 `fx.go`。
- [x] 更新 `docs/DEVELOPMENT.md` 中新增 feature 或目录结构说明，使 feature Fx 入口使用 `fx.go`。
- [x] 确认文档仍声明不新增 `openspec/` 或 `docs/opsx/`。

## Verification

- [x] 运行 `test ! -f user-service/internal/features/user/module.go`。
- [x] 运行 `test ! -f user-service/internal/features/auth/module.go`。
- [x] 运行 `test -f user-service/internal/features/user/fx.go`。
- [x] 运行 `test -f user-service/internal/features/auth/fx.go`。
- [x] 运行 `rg -n 'features/.+/module[.]go|module[.]go' AGENTS.md docs/ARCHITECTURE.md docs/DEVELOPMENT.md user-service/internal/features`，确认没有当前 feature Fx 入口引用。
- [x] 运行 `rg -n 'var Module = fx[.]Module[(]"feature-(user|auth)"' user-service/internal/features/user/fx.go user-service/internal/features/auth/fx.go`。
- [x] 在 `user-service/` 运行 `go test ./...`。
- [x] 检查 `git diff -- user-service/internal/features AGENTS.md docs/ARCHITECTURE.md docs/DEVELOPMENT.md`，确认除文件名和文档命名外没有业务逻辑变更。

## Review Notes

- [x] 确认没有重命名导出的 `Module` 变量。
- [x] 确认没有改变 provider 顺序和依赖关系。
- [x] 确认没有拆分 feature Fx module。
- [x] 确认没有新增 feature。
- [x] 确认没有移动 application、domain、transport 或 infrastructure 代码。
- [x] 确认 HTTP API、响应 envelope、错误码和状态码无变化。
- [x] 确认 Ent schema、generated code、migration 无变化。
- [x] 确认 Redis key、PostgreSQL query、JWT、session 和 token version 语义无变化。
- [x] 确认没有新增 `openspec/` 或 `docs/opsx/`。
