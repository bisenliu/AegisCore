# Tasks

## Implementation

- [x] 阅读 `docs/ARCHITECTURE.md`、`docs/DEVELOPMENT.md` 和本 change 的 `proposal.md`、`design.md`，确认本变更只做 feature infrastructure 层命名迁移。
- [x] 创建目标目录 `user-service/internal/features/user/infrastructure/postgres/`。
- [x] 创建目标目录 `user-service/internal/features/auth/infrastructure/postgres/`。
- [x] 创建目标目录 `user-service/internal/features/auth/infrastructure/redis/`。
- [x] 将 `user-service/internal/features/user/infra/postgres/*` 移动到 `user-service/internal/features/user/infrastructure/postgres/`。
- [x] 将 `user-service/internal/features/auth/infra/postgres/*` 移动到 `user-service/internal/features/auth/infrastructure/postgres/`。
- [x] 将 `user-service/internal/features/auth/infra/redis/*` 移动到 `user-service/internal/features/auth/infrastructure/redis/`。
- [x] 保持移动后的 Go package name 为 `postgres` 和 `redis`，不引入新的 package 名称。
- [x] 更新 `user-service/internal/features/user/module.go` import path 和 Fx provider 引用，使用 user infrastructure PostgreSQL package。
- [x] 更新 `user-service/internal/features/auth/module.go` import path 和 Fx provider 引用，使用 auth infrastructure PostgreSQL 和 Redis package。
- [x] 更新所有测试、helpers 或其他 Go 文件中对 feature infrastructure adapter 的 import path。
- [x] 将旧 import alias `userinfra`、`authinfra` 或同类旧层命名替换为 `userpostgres`、`authpostgres`、`authredis` 或同等清晰的新命名。
- [x] 删除迁移后的 `user-service/internal/features/user/infra/` 和 `user-service/internal/features/auth/infra/` 空目录。
- [x] 运行 `gofmt -w` 格式化所有受影响 Go 文件。

## Documentation

- [x] 更新 `AGENTS.md` Repository Shape，使 user/auth feature 分层使用 `infrastructure/postgres` 和 `infrastructure/redis`。
- [x] 更新 `AGENTS.md` Key Entry Points，使 user/auth adapter 路径指向 `infrastructure/...`。
- [x] 更新 `AGENTS.md` Repository Rules，使分层、adapter、目录命名、依赖表和 Ent predicate 规则使用 `infrastructure`。
- [x] 确认 `AGENTS.md` Ports 规则仍指向 `application/ports.go`，并且不要求把 ports 移入 infrastructure。
- [x] 更新 `docs/ARCHITECTURE.md` HTTP Request Flow，使数据访问位置指向 `features/*/infrastructure/postgres/` 和 `features/*/infrastructure/redis/`。
- [x] 更新 `docs/ARCHITECTURE.md` Feature-First Organization 表格，将 `infra/postgres` 和 `infra/redis` 改为 `infrastructure/postgres` 和 `infrastructure/redis`。
- [x] 更新 `docs/ARCHITECTURE.md` Dependency Rules、ports、adapter 和 integration 说明中的 infrastructure 层命名。
- [x] 更新 `docs/DEVELOPMENT.md` Coding Conventions 和 Adding Features，使服务内 feature 分层使用 `api/application/domain/transport/http/infrastructure/*`。
- [x] 确认文档仍声明不新增 `openspec/` 或 `docs/opsx/`。

## Verification

- [x] 运行 `test ! -d user-service/internal/features/user/infra`。
- [x] 运行 `test ! -d user-service/internal/features/auth/infra`。
- [x] 运行 `test -d user-service/internal/features/user/infrastructure/postgres`。
- [x] 运行 `test -d user-service/internal/features/auth/infrastructure/postgres`。
- [x] 运行 `test -d user-service/internal/features/auth/infrastructure/redis`。
- [x] 运行 `rg -n 'features/.*/infra(/|$)|/infra"|/infra/|infra/(postgres|redis)|internal/features/.*/infra(/|$)|\buserinfra\b|\bauthinfra\b' user-service AGENTS.md docs/ARCHITECTURE.md docs/DEVELOPMENT.md docs/TESTING.md`，确认没有当前业务引用。
- [x] 在 `user-service/` 运行 `go test ./...`。
- [x] 检查 `git diff -- user-service/internal/features AGENTS.md docs/ARCHITECTURE.md docs/DEVELOPMENT.md docs/changes/rename-feature-infra-to-infrastructure`，确认除目录、import、alias 和文档命名外没有业务逻辑变更。

## Review Notes

- [x] 确认没有把 application-owned ports、repository 接口、command/query 或 result 移入 infrastructure。
- [x] 确认没有改变 service、commands、queries、ports、result 字段或方法语义。
- [x] 确认 HTTP API、响应 envelope、错误码和状态码无变化。
- [x] 确认 Ent schema、generated code、migration 无变化。
- [x] 确认 Redis key、PostgreSQL query、JWT、session 和 token version 语义无变化。
- [x] 确认没有新增横向 `internal/repository`、`internal/store`、`internal/infrastructure` 或 `internal/shared` 包。
- [x] 确认没有将 feature infrastructure adapter 移动到 `common`、`internal/providers` 或 `internal/integration`。
- [x] 确认没有新增 `openspec/` 或 `docs/opsx/`。
