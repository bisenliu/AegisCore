# Tasks

## Preparation

- [x] 阅读 `AGENTS.md`、`docs/ARCHITECTURE.md` 和本 change 的 `proposal.md`、`design.md`，确认本变更当前目标是把 auth token/session validation helper 收敛到 `application/validators`。
- [x] 查看 `user-service/internal/features/auth/application/tokenversion/validator.go` 当前 token version validator 和 `Current` fallback 逻辑。
- [x] 查看 `user-service/internal/features/auth/domain/services/session_policy.go` 当前 session/token version 一致性 helper。
- [x] 查看 `user-service/internal/features/auth/application/validators/` 当前输入校验 helper 和测试。
- [x] 查看所有 `application/tokenversion`、`domain/services`、`authtokenversion`、`authservices` 引用。

## Package Moves

- [x] 将 token version validator 移入 `user-service/internal/features/auth/application/validators/token_version_validator.go`。
- [x] 将 session/token version 一致性 helper 移入 `user-service/internal/features/auth/application/validators/session_policy.go`。
- [x] 将 session policy 测试移入 `user-service/internal/features/auth/application/validators/session_policy_test.go`。
- [x] 删除 `user-service/internal/features/auth/application/tokenversion/validator.go`。
- [x] 删除 `user-service/internal/features/auth/domain/services/session_policy.go` 和对应测试。
- [x] 删除空的 `user-service/internal/features/auth/application/tokenversion/` 目录。
- [x] 删除空的 `user-service/internal/features/auth/domain/services/` 目录。
- [x] 不创建 `domain/events`，因为当前没有真实领域事件模型。

## Import Updates

- [x] 更新 `user-service/internal/features/auth/fx.go`，使用 `application/validators.NewValidator`。
- [x] 更新 `user-service/internal/features/auth/application/command/sessions.go`，使用 `application/validators.Current` 和 session policy helper。
- [x] 更新 `user-service/internal/features/auth/application/command/components_test.go` 的 token version validator import。
- [x] 更新 `user-service/internal/features/auth/infrastructure/redis/session_store_test.go` 的 token version validator import。
- [x] 确认没有剩余 `application/tokenversion`、`domain/services`、`authtokenversion` 或 `authservices` 业务引用。

## Boundary Rules

- [x] 保持 application command/query 继续负责用例编排。
- [x] 保持 auth validators 不导入 Gin、HTTP request/response DTO、HTTP response writer、Ent、Redis client、SQL 或 infrastructure adapter。
- [x] 保持 auth validators 可以依赖 application ports、auth domain、common runtime logger 和 common security auth primitive。
- [x] 保持 infrastructure/postgres 继续负责 Ent、SQL、predicate 构造和存储错误转换。
- [x] 保持 infrastructure/redis 继续负责 Redis client、Redis key 使用、session serialization 和缓存操作。
- [x] 保持 transport/http 不直接绕过 application command/query 访问 token/session validation helper。

## Documentation

- [x] 更新 `AGENTS.md` Repository Shape，使 auth validators 承载输入校验、token version 撤销校验、cache/database fallback 和 session 一致性校验。
- [x] 更新 `AGENTS.md` Key Entry Points，将 token version validator 和 session policy 指向 `application/validators`。
- [x] 更新 `AGENTS.md` Repository Rules，说明 `domain/services` 和 `domain/events` 仍是按需目录，不得添加空业务包。
- [x] 更新 `docs/ARCHITECTURE.md` Feature-First Organization，说明 auth validators 的当前职责和 domain 子目录准入条件。
- [x] 更新 `docs/ARCHITECTURE.md` Dependency Rules，保留 pure domain 子包约束，并说明 auth token/session helper 当前保留在 validators。
- [x] 更新 `docs/ARCHITECTURE.md` Current Constraints 或相关章节，说明当前不引入事件总线实现。
- [x] 更新 `docs/DEVELOPMENT.md` Adding Features 或 Coding Conventions，说明 auth token version/session validation 归入 `application/validators`。
- [x] 更新本 change 的 `proposal.md` 和 `design.md`，使 artifacts 与当前实现一致。
- [x] 确认没有新增 `openspec/` 或 `docs/opsx/`。

## Formatting

- [x] 对受影响 Go 文件运行 `gofmt -w`。
- [x] 检查 Go import alias，使用清晰 alias，例如 `authvalidators`。

## Verification

- [x] 运行目录检查：

```bash
test ! -d user-service/internal/features/auth/application/tokenversion
test ! -d user-service/internal/features/auth/domain/services
test -f user-service/internal/features/auth/application/validators/token_version_validator.go
test -f user-service/internal/features/auth/application/validators/session_policy.go
```

- [x] 运行引用扫描，确认没有旧 auth 包引用：

```bash
rg -n 'application/tokenversion|authtokenversion|authservices|internal/features/auth/domain/services' user-service/internal/features/auth user-service/internal/providers AGENTS.md docs/ARCHITECTURE.md docs/DEVELOPMENT.md
```

- [x] 运行 validators 越层依赖扫描：

```bash
rg -n 'gin-gonic|common/http/binding|common/http/response|/ent/|redis\.|database/sql|infrastructure/' user-service/internal/features/auth/application/validators
```

- [x] 在 `user-service/` 下运行：

```bash
go test ./internal/features/auth/...
go test ./internal/features/user/... ./internal/features/auth/...
```

- [x] 检查 `git diff -- AGENTS.md docs/ARCHITECTURE.md docs/DEVELOPMENT.md docs/changes/add-feature-domain-events-services-layout user-service/internal/features/auth user-service/internal/providers`，确认没有 HTTP API、JWT、Redis key、Ent schema、migration 或无关重构变更。

## Review Notes

- [x] 确认 `application/tokenversion` 已移除。
- [x] 确认 `domain/services` 已移除，且没有新增空 `domain/events`。
- [x] 确认 token version cache hit、cache miss、database fallback 和 cache backfill 逻辑保持不变。
- [x] 确认 refresh session metadata 与 token claims/token version 的一致性检查保持不变。
- [x] 确认 application command/query 仍是用例编排入口。
- [x] 确认 infrastructure adapter 仍是 Ent/Redis 访问入口。
- [x] 确认 HTTP request/response DTO 没有移动到 application validators 或 domain。
- [x] 确认 response envelope、错误码和状态码无变化。
- [x] 确认 JWT subject、claims、TTL fallback 和 Bearer 兼容行为无变化。
- [x] 确认 Redis key builder、session key 和 token version cache key 语义无变化。
- [x] 确认 Ent generated code、Ent schema、Atlas migration 和部署资产无变化。
- [x] 确认没有新增事件总线、broker、outbox、publisher、subscriber 或后台 worker。
- [x] 确认没有新增横向 `internal/domain`、`internal/shared`、`internal/events` 或 `internal/services` 包。
