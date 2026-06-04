## Context

AegisCore 当前是 Go workspace，包含 `common` 和 `user-services` 两个 Go module。`common` module path 为 `github.com/aegiscore/common`，内部已有多个共享能力包：配置加载、日志、基础设施 provider、HTTP 中间件、响应契约、认证凭证、密码、时区和请求校验。`user-services` 通过这些包完成 HTTP runtime、Fx 依赖装配、API 响应、认证、校验和日志输出。

现有 `common` 子包基本按职责拆分，但顶层目录仍是兜底式命名。随着共享能力增加，单层 `common/<package>` 结构无法表达能力类别，容易让新增共享代码绕过 capability 判断。本设计通过能力分类目录重组代码位置，让目录结构直接反映长期边界：契约、运行时、HTTP 适配、安全和通用校验。

本变更是跨模块包路径重构，会影响 `common` 内部 import、`user-services` import、测试、文档和 OpenSpec 路径引用。变更不得修改 HTTP API、配置 key、环境变量、Redis/PostgreSQL 命名实例、Fx named injection、日志字段、数据库 schema 或 Ent 生成代码。

## Goals / Non-Goals

**Goals:**

- 将 `common` 目录重组为 `contract/`、`runtime/`、`http/`、`security/` 和 `validation/` 五类边界。
- 保持 `github.com/aegiscore/common` Go module path 不变，仅修改 module 内子包路径。
- 让响应契约、运行时基础能力、HTTP 适配、安全凭证和通用校验在目录层面可区分。
- 同步更新仓库内 Go imports、测试、文档、capability map 和相关 OpenSpec 规格引用。
- 保持现有外部可观察行为不变，包括响应信封、错误码、配置加载、trace-id、认证、密码 hash、请求校验和 Fx runtime wiring。

**Non-Goals:**

- 不拆分 `common` Go module，不改 module path。
- 不新增服务、HTTP API、认证业务流程、数据库表或 Atlas migration。
- 不重写现有 logger、validator、middleware、config loader 或 datastore provider 行为。
- 不为旧 import path 提供兼容 wrapper 包；本仓库内引用在同一变更中同步迁移。
- 不改变 `user-services` 的 controller/service/repository 分层。

## Decisions

### Decision 1: 保留 `common` module path，仅重组子目录

本变更保留 `github.com/aegiscore/common`，避免扩大为 Go module 级 breaking change。迁移后的 import path 示例：

- `github.com/aegiscore/common/response` -> `github.com/aegiscore/common/contract/response`
- `github.com/aegiscore/common/config` -> `github.com/aegiscore/common/runtime/config`
- `github.com/aegiscore/common/logger` -> `github.com/aegiscore/common/runtime/logger`
- `github.com/aegiscore/common/infrastructure` -> `github.com/aegiscore/common/runtime/infrastructure`
- `github.com/aegiscore/common/timezone` -> `github.com/aegiscore/common/runtime/timezone`
- `github.com/aegiscore/common/middleware` -> `github.com/aegiscore/common/http/middleware`
- `github.com/aegiscore/common/ginvalidation` -> `github.com/aegiscore/common/http/ginvalidation`
- `github.com/aegiscore/common/auth` -> `github.com/aegiscore/common/security/auth`
- `github.com/aegiscore/common/password` -> `github.com/aegiscore/common/security/password`
- `github.com/aegiscore/common/validation` remains `github.com/aegiscore/common/validation`

Alternative considered: rename `common` to `platform` or split into multiple modules. This was rejected for now because current repository has one consuming service, and module split would introduce coordination cost without proving stable multi-service boundaries.

### Decision 2: Keep Go package names short and mostly unchanged

Directory hierarchy carries the capability category; leaf package names stay short: `response`, `config`, `logger`, `infrastructure`, `timezone`, `middleware`, `ginvalidation`, `auth`, `password`, `validation`. This preserves idiomatic Go call sites such as `response.OK`, `config.Load`, `logger.Info`, and avoids stutter like `contractresponse.OK`.

Alternative considered: rename leaf packages to include category prefixes. This was rejected because Go package names should be short, lowercase and clear at call site.

### Decision 3: Separate HTTP adapter from validation core

`common/validation` remains the generic validation core without Gin dependency. `common/http/ginvalidation` becomes the Gin-specific binding, abort and failure-response adapter. This keeps non-HTTP or non-Gin consumers able to reuse core validation without importing Gin.

Alternative considered: move validation core under `runtime/` or `http/`. This was rejected because core validation is neither runtime dependency wiring nor HTTP-specific behavior.

### Decision 4: Move credentials under `security/` without changing behavior

JWT token primitives, Bearer constants and authentication context helpers move under `security/auth`; password hash and verification move under `security/password`. The move clarifies that these packages are security primitives, not user-service business authentication workflows. User session control remains owned by `user-services`.

Alternative considered: create a single `security/credentials` package. This was rejected because existing `common-credentials` spec explicitly keeps password and auth primitives focused and avoids an aggregate credentials package.

### Decision 5: Documentation and specs migrate with code

All references in architecture, development, capability map and relevant OpenSpec specs must be updated with the new package paths. The docs must also state that `common` is for stable shared contracts and infrastructure, not for service-specific convenience helpers.

## Risks / Trade-offs

- [Risk] Import path churn may miss references in tests, docs or Swagger-adjacent comments. → Mitigation: search the workspace for `github.com/aegiscore/common/` and old `common/<dir>` references after moving packages; run tests in both modules.
- [Risk] Directory movement may accidentally alter package names or exported API names. → Mitigation: keep leaf package names and exported identifiers unchanged unless required by imports.
- [Risk] Fx named injection or resource constants may change while moving infrastructure code. → Mitigation: preserve constant values, struct tags and provider behavior; verify with `common` and `user-services` tests.
- [Risk] Middleware, response or validation behavior could change due to import rewrites. → Mitigation: no functional rewrites during migration; run existing controller, middleware, validation and response tests.
- [Risk] Documentation could describe desired structure but not actual code. → Mitigation: update docs only after code paths are moved and verified.

## Migration Plan

1. Create new target directories under `common/contract/`, `common/runtime/`, `common/http/` and `common/security/`.
2. Move package files to their new directories, preserving package declarations where possible.
3. Update imports inside `common` and `user-services` to new paths.
4. Update documentation and OpenSpec references to the new paths and governance rule.
5. Run formatting where needed and execute `go test ./...` in `common/` and `user-services/`.
6. If migration causes unexpected behavior or broad failures, rollback by moving packages back to original directories and restoring imports; no database or external runtime rollback is required because no persistent schema or configuration contract changes are introduced.

## Open Questions

- None for proposal scope. Future multi-service adoption may justify splitting `common` into multiple Go modules, but that is intentionally outside this change.
