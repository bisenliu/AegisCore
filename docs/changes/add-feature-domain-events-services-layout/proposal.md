# Add feature domain events services layout

## What

为用户服务内复杂领域规则预留更清晰的 domain 子结构，并明确当前 auth token/session 校验能力继续归入 `application/validators`，不再为本次变更创建 `domain/services` 业务包。

目标结构按 feature 需要出现，而不是所有 feature 强制补齐：

```text
user-service/internal/features/<feature>/
  domain/
    events/      # 仅在存在真实领域事件模型时新增
    services/    # 仅在存在真实纯领域服务规则时新增
```

本变更迁移和明确：

- `application/validators` 承载 auth transport-neutral 输入校验、token version 撤销校验、cache/database fallback 策略和 refresh session 一致性校验。
- 移除独立 `application/tokenversion` 包，将 token version validator 并入 `application/validators`。
- 不创建 auth `domain/services`，避免为了两个校验 helper 额外扩展 domain 子包。
- 不创建 `domain/events`，因为当前没有真实领域事件模型。
- 明确未来 `domain/services` 只能在存在真实纯领域规则时新增，并且不得依赖 Gin、Ent、Redis、config、logger、HTTP response、application ports 或 infrastructure adapter。
- 明确未来 `domain/events` 只是领域事实的数据模型和事件命名，不引入事件总线、broker、outbox、发布器或异步投递实现。
- 更新 `AGENTS.md`、`docs/ARCHITECTURE.md` 和 `docs/DEVELOPMENT.md`，使后续复杂领域规则有稳定准入规则，同时反映 auth validators 的当前落点。

## Why

当前 auth token version validator 和 refresh session 一致性判断服务于认证 token/session 的应用层校验流程。它们虽然不属于 HTTP DTO 字段校验，但仍与 application ports、Redis cache/database fallback、access token middleware 和 session lifecycle 紧密相关。放在 `application/validators` 可以减少包碎片，并避免将依赖 application ports 的策略误归入 domain。

引入 domain 子结构的规则仍然有价值：随着用户生命周期、认证会话和未来审计/事件能力变复杂，`domain/services` 和 `domain/events` 可以为真正纯净的领域服务与领域事件模型提供落点。但本次实现不为了目录完整而创建空包，也不把当前 token/session 校验拆到 domain。

本变更特别约束不引入事件总线实现。领域事件模型可以在未来作为纯数据结构存在；如果需要 outbox、broker 或集成事件发布，应另开变更并放在 application、infrastructure 或 `internal/integration/events` 的合适边界中设计。

## Scope

包括：

- 将 `user-service/internal/features/auth/application/tokenversion/validator.go` 移入 `application/validators`。
- 将当前 refresh session 与 token version 一致性 helper 放入 `application/validators`。
- 更新 auth command、Fx module、测试和 Redis adapter 测试的 import path。
- 保持 domain 根部继续承载实体、值对象、枚举和领域错误。
- 不创建 `domain/services` 或 `domain/events`，除非未来出现真实迁移目标。
- 保持 application command/query 继续承载用例编排、密码 hash、JWT 签发、session lifecycle 和跨 store 协作。
- 保持 infrastructure/postgres 和 infrastructure/redis 继续承载 Ent、SQL、Redis client、key 使用和存储模型转换。
- 更新架构文档，说明 `domain/services` 和 `domain/events` 的准入条件、禁止依赖和与 application validators/infrastructure/integration 的边界。
- 运行 `gofmt` 格式化受影响 Go 文件。

不包括：

- 不为了目录完整而新增空业务代码或空占位 package。
- 不引入事件总线、消息 broker、outbox、publisher、subscriber、异步投递或 integration event 实现。
- 不改变 HTTP API、request/response DTO、response envelope、状态码或错误码。
- 不改变 JWT claim、token TTL、Redis key、session 存储语义或 token version cache 策略。
- 不改变 Ent schema、Ent generated code、Atlas migration 或数据库结构。
- 不把 HTTP DTO、Redis key builder、JWT service 或密码 hash helper 移入 domain services。
- 不新增横向 `internal/domain`、`internal/shared`、`internal/events` 或 `internal/services` 包。
- 不把服务内领域事件误放入 `internal/integration/events`；后者只用于外部系统防腐层。
- 不重新新增 `openspec/` 或 `docs/opsx/` 工件。

## Acceptance Criteria

- `user-service/internal/features/auth/application/tokenversion/` 不再存在。
- `user-service/internal/features/auth/domain/services/` 不再存在。
- `user-service/internal/features/auth/application/validators/` 承载 auth 输入校验、token version validator 和 refresh session 一致性 helper。
- Auth Fx module 使用 validators 包提供 token version validator。
- Auth command 和 Redis adapter tests 不再导入 `application/tokenversion` 或 `domain/services`。
- 不存在只包含占位注释、空 struct、空 interface 或 package doc 的 `domain/services` 或 `domain/events` 业务包。
- Auth 登录、刷新、强制改密、登出和 token version 语义保持不变。
- User 创建、查询和列表语义保持不变。
- `AGENTS.md`、`docs/ARCHITECTURE.md` 和 `docs/DEVELOPMENT.md` 说明 `application/validators` 的当前 auth 职责，并说明 `domain/services`、`domain/events` 只在真实规则存在时创建。
- 在 `user-service/` 下运行相关测试通过，至少覆盖 `./internal/features/auth/...`。
