# Tasks

## Implementation

- [x] 阅读 `docs/ARCHITECTURE.md`、`docs/DEVELOPMENT.md` 和本 change 的 `proposal.md`、`design.md`，确认本变更只做 feature 应用层命名迁移。
- [x] 创建目标目录 `user-service/internal/features/user/application/` 和 `user-service/internal/features/auth/application/`。
- [x] 将 `user-service/internal/features/user/app/*` 移动到 `user-service/internal/features/user/application/`。
- [x] 将 `user-service/internal/features/auth/app/*` 移动到 `user-service/internal/features/auth/application/`。
- [x] 将移动后的 user 应用层文件从 `package app` 改为 `package application`。
- [x] 将移动后的 auth 应用层文件从 `package app` 改为 `package application`。
- [x] 更新 `user-service/internal/features/user/module.go` import path 和 Fx `fx.As` annotation，使用 user application package。
- [x] 更新 `user-service/internal/features/auth/module.go` import path 和 Fx `fx.As` annotation，使用 auth application package。
- [x] 更新 user feature HTTP controller、mapper 和 controller tests 中对 application command/query/result/service 的引用。
- [x] 更新 auth feature HTTP controller、mapper 和 controller tests 中对 application command/query/result/service 的引用。
- [x] 更新 user PostgreSQL adapter 和 adapter tests 中对 application ports/results 的引用。
- [x] 更新 auth PostgreSQL adapter 和 adapter tests 中对 application ports、credentials 和 token version store 的引用。
- [x] 更新 auth Redis adapter 和 adapter tests 中对 application session ports/results 的引用。
- [x] 更新 `user-service/internal/providers/routes_test.go` 中对 user/auth application service 或 port 类型的引用。
- [x] 将旧 import alias `userapp`、`authapp` 替换为 `userapplication`、`authapplication` 或同等清晰的新命名。
- [x] 删除迁移后的 `user-service/internal/features/user/app/` 和 `user-service/internal/features/auth/app/` 空目录。
- [x] 运行 `gofmt -w` 格式化所有受影响 Go 文件。

## Documentation

- [x] 更新 `AGENTS.md` Repository Shape，使 user/auth feature 分层使用 `application/`。
- [x] 更新 `AGENTS.md` Key Entry Points，使 user/auth service 路径指向 `application/service.go`。
- [x] 更新 `AGENTS.md` Repository Rules，使分层、ports、controller mapping、依赖表和 Ent predicate 规则使用 `application`。
- [x] 更新 `docs/ARCHITECTURE.md` HTTP Request Flow，使业务调用位置指向 `features/*/application/`。
- [x] 更新 `docs/ARCHITECTURE.md` Feature-First Organization 表格，将 `app/` 改为 `application/`。
- [x] 更新 `docs/ARCHITECTURE.md` Dependency Rules、ports、controller 和 integration 说明中的应用层命名。
- [x] 更新 `docs/DEVELOPMENT.md` Coding Conventions 和 Adding Features，使服务内 feature 分层使用 `api/application/domain/transport/http/infra/*`。
- [x] 确认文档仍声明不新增 `openspec/` 或 `docs/opsx/`。

## Verification

- [x] 运行 `test ! -d user-service/internal/features/user/app`。
- [x] 运行 `test ! -d user-service/internal/features/auth/app`。
- [x] 运行 `test -d user-service/internal/features/user/application`。
- [x] 运行 `test -d user-service/internal/features/auth/application`。
- [x] 运行 `rg -n '/app"|features/(user|auth)/app(/|$)|^package app$|\buserapp\b|\bauthapp\b' user-service AGENTS.md docs/ARCHITECTURE.md docs/DEVELOPMENT.md docs/TESTING.md`，确认没有当前业务引用。
- [x] 在 `user-service/` 运行 `go test ./...`。
- [x] 检查 `git diff -- user-service/internal/features user-service/internal/providers AGENTS.md docs/ARCHITECTURE.md docs/DEVELOPMENT.md`，确认除目录、package、import、alias 和文档命名外没有业务逻辑变更。

## Review Notes

- [x] 确认没有拆分 command/query 子目录。
- [x] 确认没有改变 service、commands、queries、ports、result 字段或方法语义。
- [x] 确认 HTTP API、响应 envelope、错误码和状态码无变化。
- [x] 确认 Ent schema、generated code、migration 无变化。
- [x] 确认 Redis key、PostgreSQL query、JWT、session 和 token version 语义无变化。
- [x] 确认没有将 feature 应用层代码移动到 `common`、`internal/shared`、`internal/providers` 或 integration 边界。
- [x] 确认没有新增 `openspec/` 或 `docs/opsx/`。
