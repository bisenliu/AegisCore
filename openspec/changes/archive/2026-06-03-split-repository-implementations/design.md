## Context

`user-services` 当前通过 controller/service/repository 分层实现用户资料、认证会话和 HTTP runtime 装配。根 `repository` 包既包含 `UserRepository` 接口，也包含 Ent/PostgreSQL 具体实现；service 层的 `session_store.go` 既定义认证会话抽象，也包含 Redis 存储实现。这会让业务编排层暴露存储细节，也让 Fx provider 的包归属和测试位置不够清晰。

本变更只调整内部包边界和依赖方向。HTTP 路由、响应信封、错误码、Redis/PostgreSQL 命名实例、Ent schema、Atlas migration 和 token 语义保持不变。

## Goals / Non-Goals

**Goals:**

- 让根 `repository` 包只保留用户仓储和认证会话仓储的接口、输入结构和 repository 级错误。
- 将 PostgreSQL/Ent 用户仓储实现迁移到 `repository/postgres` 包。
- 将 Redis 认证会话仓储实现迁移到 `repository/redis` 包。
- 让 service 层只依赖根 `repository` 抽象，不依赖 Redis session store 具体类型。
- 更新 Fx 装配，使 runtime 通过具体实现包提供 `repository.UserRepository` 和 `repository.AuthSessionRepository`。
- 迁移并更新测试，验证重构后行为与现有用户查询、用户创建、列表查询和认证会话控制一致。

**Non-Goals:**

- 不修改 HTTP API、请求/响应 JSON、业务错误码或 Swagger 语义。
- 不修改 Ent schema、生成代码、Atlas migration 或数据库结构。
- 不将 `UserRepository` 的返回类型从 `*ent.User` 替换为 domain DTO。
- 不新增 Redis/PostgreSQL 配置项或改变 `cache_redis`、`user_db` 具名依赖。
- 不引入新的外部依赖或跨模块 common 抽象。

## Decisions

### Decision: 根 repository 包保留抽象，具体实现按存储类型分包

根 `user-services/internal/repository` 包保留 `UserRepository`、`CreateUserInput`、`UpdateCredentialsInput`、`ListUsersInput`、`AuthSessionRepository`、`AuthSession`、`ErrAuthSessionNotFound` 和 `ErrTokenVersionMismatch`。PostgreSQL/Ent 代码迁移到 `repository/postgres`，Redis 会话代码迁移到 `repository/redis`。

理由：这保持 `service -> repository` 的稳定依赖方向，同时允许具体存储实现依赖根抽象。相比在 service 层保留 Redis 实现，这能避免业务编排层承担存储层职责；相比创建跨模块 common 仓储接口，当前能力仍属于用户服务内部，不需要提升到 shared infrastructure。

### Decision: 保持 UserRepository 的 Ent 返回类型不变

本次继续让 `UserRepository` 返回 `*ent.User`。

理由：这次目标是包边界迁移，不是领域模型解耦。立即改为 domain DTO 会扩大影响范围，涉及 user service、auth service、测试和潜在 Swagger 映射。后续如要支持非 Ent 实现，可单独提出更大的数据访问契约变更。

### Decision: AuthSessionRepository 仍提供 token version 校验能力

`AuthSessionRepository` 保留 `ValidateTokenVersion(ctx, userID, tokenVersion)`，使 bootstrap 中的认证中间件 token version validator 仍能依赖同一个抽象。

理由：认证中间件只需要 token version 校验，不需要知道 Redis 实现细节。相比为 middleware 新增独立 adapter，本次保留同一仓储接口可以减少装配复杂度，并保持现有行为。

### Decision: Fx provider 位于具体实现包

`postgres.NewUserRepository` 返回 `repository.UserRepository`，`redis.NewAuthSessionRepository` 返回 `repository.AuthSessionRepository`。bootstrap 显式引用两个具体实现包完成 runtime 装配。

理由：bootstrap 是运行时组合根，可以依赖具体 provider；service 和根 repository 不应反向依赖具体实现包。这符合 Fx composition root 模式，并保持 `user_db`、`cache_redis` 具名依赖由 runtime 组装管理。

### Decision: 测试随实现包迁移

PostgreSQL 用户仓储测试迁移到 `repository/postgres`，Redis 认证会话仓储测试迁移到 `repository/redis`。auth service 和 bootstrap 测试使用根 repository 接口与 stub。

理由：实现包测试应与被测具体类型同包，便于覆盖私有 helper；service 测试只验证业务编排，不应依赖 Redis 实现类型。

## Risks / Trade-offs

- [Risk] 包迁移可能造成 import 循环。→ Mitigation：根 `repository` 包不得 import `repository/postgres` 或 `repository/redis`；service 只 import 根 repository；bootstrap 作为组合根 import 具体实现包。
- [Risk] 错误重命名可能导致 `errors.Is` 匹配失败。→ Mitigation：统一将 `ErrSessionNotFound` 引用替换为 `repository.ErrAuthSessionNotFound`，将 token version mismatch 引用替换为 `repository.ErrTokenVersionMismatch`，并通过 auth service 测试验证。
- [Risk] Redis session store 迁移后 key helper 或 TTL 默认值行为改变。→ Mitigation：整体迁移 Redis 方法和 helper，保留现有 key 格式、ZSet 清理、TTL fallback 和测试断言。
- [Risk] Fx provider 替换遗漏导致服务启动缺少依赖。→ Mitigation：更新 bootstrap provider 列表和 `BootstrapParams` 类型，运行 `go test ./...` 验证 Fx 相关单元测试。

## Migration Plan

1. 新增根 repository 认证会话抽象文件。
2. 迁移 Redis session store 实现到 `repository/redis`，保留方法语义、key helper 和 TTL 默认值。
3. 更新 auth service、bootstrap 和测试对 session 类型与错误的引用。
4. 迁移 Ent/PostgreSQL 用户仓储实现到 `repository/postgres`，精简根 `user_repository.go`。
5. 更新 bootstrap provider 和用户仓储测试位置。
6. 执行 `gofmt` 和 `go test ./...`。

Rollback 策略：由于不涉及数据结构或外部接口变更，可通过恢复包迁移前的文件布局和 provider 引用回滚；无需数据库 migration 回滚。

## Open Questions

- 无待决问题。本变更范围限定为内部包边界和依赖方向调整。
