# Design

## Overview

本变更保留 domain 子结构准入规则，但当前 auth token/session 校验能力统一落在 `application/validators`：

```text
features/<feature>/domain
  -> entities, value objects, enums, domain errors
features/<feature>/domain/services
  -> optional pure domain services, only when real candidates exist
features/<feature>/domain/events
  -> optional pure domain event models, only when real candidates exist

features/auth/application/validators
  -> transport-neutral input validation
  -> token version revocation validator
  -> token version cache/database fallback
  -> refresh session and token version consistency helpers

features/<feature>/application/command|query
  -> use case orchestration
features/<feature>/infrastructure/*
  -> Ent, Redis, SQL, persistence and runtime adapters
features/<feature>/transport/http
  -> Gin, HTTP DTOs, binding, response mapping
```

`domain/services` 和 `domain/events` 是按需目录。实现时必须先找到真实代码迁移目标，确认它属于领域层且依赖纯净，再创建目录。没有真实迁移目标时，只更新文档规则，不创建空包。

## Target Package Rules

### domain root

`domain/` 根部继续承载：

- 领域实体，例如 `User`、`UserCredential`、`AuthSession`。
- 值对象、枚举和状态类型，例如 `UserStatus`。
- 领域错误，例如 `ErrInvalidCredentials`、`ErrMissingSession`。
- 与单个实体或值对象紧密绑定的短方法，例如 `CanLogin`、`RequiresPasswordChange`。

不要把所有已有 domain 文件机械移动到子目录。根部仍然是简单领域模型的默认位置。

### application/validators

Auth `application/validators` 承载当前认证输入和 token/session validation helper：

- `auth_validator.go`：login、refresh token、change-password command 的 transport-neutral 输入校验。
- `token_version_validator.go`：access token 中间件使用的 token version 撤销校验器，以及 Redis cache miss 后回源数据库并 backfill cache 的策略。
- `session_policy.go`：refresh session metadata 与 token claims/token version 的一致性 helper。

该包可以依赖：

- `auth/application` ports。
- `auth/domain` 错误和模型。
- `common/security/auth` token version validator interface。
- `common/runtime/logger`，用于记录 cache/database fallback 错误。

该包不得导入 Gin、HTTP request/response DTO、HTTP response writer、Ent、Redis client、SQL 或 infrastructure adapter。

### domain/services

`domain/services` 只承载跨实体、跨值对象或不适合挂在单个实体方法上的纯领域规则。它可以包含函数或小型无状态 service 类型，但不应该成为 application service 或 application validators 的替代品。

允许：

- 标准库。
- 同 feature 的 `domain` 包。
- 其他 feature 中已经被当前 domain 根部稳定消费的纯领域值对象，例如 auth 当前使用 user `UserStatus`。
- 纯算法和纯规则组合。

禁止：

- Gin、HTTP binder、HTTP response、Swagger DTO。
- Ent、SQL、Redis client、Redis key 使用、外部 SDK。
- `common/runtime/config`、logger、Fx。
- JWT service、password hash helper、Bearer token 解析。
- Application ports、command/query DTO、use case、validators。
- Infrastructure adapter 和 `internal/providers`。

当前没有把 auth token/session helper 放入 `domain/services`，因为 token version validator 依赖 application ports 和 cache/database fallback，且用户明确要求该能力归入 `application/validators`。

### domain/events

`domain/events` 承载领域事件模型。领域事件表示业务领域中已经发生的事实，命名应使用过去式或事实式，例如 `PasswordChanged`、`UserCreated`、`UserDisabled`。

允许：

- 事件名常量。
- 事件 payload struct。
- 事件构造函数或字段规范化，只要保持纯净。
- 标准库时间、UUID 等纯值类型。

禁止：

- 事件总线、publisher、subscriber、handler。
- Kafka、NATS、RabbitMQ、Redis Stream 或任何外部 broker SDK。
- Outbox persistence、transaction hook、Ent mutation hook。
- 外部系统 DTO 或 integration event schema。
- HTTP response 或 transport DTO。

如果当前没有任何真实领域事件会被 application 或后续变更消费，实施时不创建 `domain/events`。文档中只记录准入规则。

## Package Migration

Move token version validator:

- From `user-service/internal/features/auth/application/tokenversion/validator.go`
- To `user-service/internal/features/auth/application/validators/token_version_validator.go`

Move session consistency helpers:

- From `user-service/internal/features/auth/domain/services/session_policy.go`
- To `user-service/internal/features/auth/application/validators/session_policy.go`

Remove empty directories after moving files:

- `user-service/internal/features/auth/application/tokenversion/`
- `user-service/internal/features/auth/domain/services/`

Update import paths in:

- `user-service/internal/features/auth/fx.go`
- `user-service/internal/features/auth/application/command/sessions.go`
- `user-service/internal/features/auth/application/command/components_test.go`
- `user-service/internal/features/auth/infrastructure/redis/session_store_test.go`

## Behavior Preservation

The implementation must not change:

- Public HTTP paths or HTTP methods.
- Request/response DTO fields.
- Response envelope shape.
- Command/use case result fields.
- Port interface method sets or ownership.
- Domain errors and value objects.
- Ent predicates, query filters, mutation fields or storage error mapping.
- Redis key construction, session serialization, token version lookup or expiration semantics.
- JWT subject, claims, TTL fallback or Bearer compatibility behavior.
- Fx graph composition beyond package path references.
- Config keys, runtime resource names, Ent schema, generated code or migration files.

## Documentation Updates

Update `AGENTS.md`:

- Repository Shape for auth should state `application/validators` owns auth input validation, token version validator, cache/database fallback and session consistency helper.
- Key Entry Points should point to `application/validators/token_version_validator.go` and `application/validators/session_policy.go`.
- Repository Rules should keep `domain/services` and `domain/events` optional and forbid empty placeholder packages.
- Dependency table should clarify domain subpackages follow pure domain constraints, while auth validators may depend on application ports and common runtime/security primitives.

Update `docs/ARCHITECTURE.md`:

- Feature-First Organization table should describe optional domain subdirectories and current auth validators responsibilities.
- Dependency Rules should include pure domain subpackage constraints.
- Current Constraints or Feature-First sections should state no event bus exists and event publishing requires a future change.

Update `docs/DEVELOPMENT.md`:

- Mention auth token version and session consistency validation live in `application/validators`.
- Mention domain subdirectories are created only when real domain services or domain event models exist.

Historical `docs/changes/*` records can remain unchanged unless directly relevant.

## Verification Strategy

Run structural checks:

```bash
test ! -d user-service/internal/features/auth/application/tokenversion
test ! -d user-service/internal/features/auth/domain/services
test -f user-service/internal/features/auth/application/validators/token_version_validator.go
test -f user-service/internal/features/auth/application/validators/session_policy.go
rg -n 'application/tokenversion|domain/services|authtokenversion|authservices' user-service/internal/features/auth user-service/internal/providers AGENTS.md docs/ARCHITECTURE.md docs/DEVELOPMENT.md
```

Run Go tests for affected feature packages:

```bash
cd user-service
go test ./internal/features/auth/...
go test ./internal/features/user/... ./internal/features/auth/...
```

## Risks And Mitigations

### Validators package becomes too broad

Risk: `application/validators` now holds both input validation and token/session validation strategy.

Mitigation: keep filenames explicit (`auth_validator.go`, `token_version_validator.go`, `session_policy.go`) and keep orchestration in command use cases.

### Behavior drift during package move

Risk: moving token version validator changes cache/database fallback or error wrapping.

Mitigation: preserve function names and implementation body, update import paths only, and run existing auth command and Redis adapter tests.

### Empty package drift

Risk: keeping empty `domain/services` or `domain/events` directories encourages placeholder code.

Mitigation: remove empty directories and document that domain subpackages are optional.

### Event bus scope creep

Risk: adding `domain/events` rules invites broker or outbox implementation during the same change.

Mitigation: events are only pure models in future changes. Publishing and delivery require a separate design because they cross application, infrastructure and integration boundaries.
