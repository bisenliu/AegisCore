## Context

`/api/v1/auth/change-password` 当前处理的是强制改密/首次登录改密：调用方先通过 `/api/v1/auth/login` 获得 subject 为 `password_change` 的受限 token，再调用该入口消费一次性 password-change session 并完成凭据更新。该流程不经过普通 access token middleware，和未来“已登录用户提供旧密码修改自己的密码”的保护语义不同。

本 change 调整 HTTP 契约和强制改密相关 symbol 命名边界，不改变 token subject、请求字段、响应字段、Redis session、token version、撤销和错误处理语义。受影响范围集中在 user-service auth feature、router/provider 测试、OpenAPI 生成物、架构文档和 auth-session-management 规格；common、数据库 schema、migration、部署清单和观测资产不需要变化。

## Goals / Non-Goals

**Goals:**

- 将强制改密入口命名为 `POST /api/v1/auth/force-change-password`，明确它是受限 token 流程。
- 保持该入口在 auth public routes 下注册，但由 controller/use case 校验 `password_change` token。
- 移除旧 `POST /api/v1/auth/change-password` 路由，不提供别名。
- 将当前强制改密流程的内部命名同步改为 `ForceChangePassword`，避免未来普通 `ChangePassword` 接口落地时冲突。
- 更新 OpenAPI 注解、生成物、测试、文档和规格，保证可发现契约与实际路由一致。
- 为未来普通已登录改密预留独立命名和 protected route 边界。

**Non-Goals:**

- 不新增普通已登录改密接口。
- 不改变强制改密的业务编排、请求字段、响应字段、错误码、token subject 或 session 存储语义。
- 不调整密码策略、bcrypt 行为、JWT 配置、Redis key schema、token version cache 或 refresh session 撤销实现。
- 不修改 Ent schema、Atlas migration、RBAC 策略、部署清单或观测指标。

## Decisions

- 路径使用 `/auth/force-change-password`，而不是继续使用 `/auth/change-password`。

  选择原因：`force-change-password` 直接对应 `UserStatusMustChangePassword` 和 `password_change` token 流程，避免和未来普通改密冲突。备选 `/auth/password-change/complete` 更偏流程化但更长，且和当前 `/auth/login`、`/auth/refresh`、`/auth/logout` 的短动作风格不一致。

- 不保留旧路径别名。

  选择原因：主规格已经要求系统不得暴露旧认证路径别名或认证绕过路径；双挂会让客户端继续依赖歧义命名，也增加 OpenAPI 与路由保护测试负担。备选方案是短期双挂并 deprecated，但这需要额外规格豁免，并可能延长迁移窗口。

- 同步重命名强制改密相关内部 symbol。

  选择原因：未来普通已登录改密需要保留自然的 `ChangePassword` 命名；当前流程已经由 `password_change` 受限 token 和 `UserStatusMustChangePassword` 限定，应使用 `ForceChangePassword` 表达强制改密。备选方案是只改路由、不改内部 symbol，但会在新增普通改密时产生 use case、DTO、测试 helper 命名冲突。

- 不改变 public/protected route 分组。

  选择原因：该入口不应由普通 access token middleware 保护，否则 `password_change` subject 会被共享认证 middleware 拒绝；它必须继续在 public auth group 中进入 controller，然后由业务层按受限 token 和一次性 session fail-closed 校验。

## Risks / Trade-offs

- [客户端破坏性变更] 已接入旧路径的客户端会收到 404。→ 在 release note 和 OpenAPI diff 中明确路径替换，客户端同步切换到 `/api/v1/auth/force-change-password`。
- [漏改测试或文档] 路由注册、middleware 测试、OpenAPI 注解和架构文档可能仍引用旧路径。→ 使用文本搜索覆盖 `change-password` 引用，保留非路径语义的 `password-change token/session` 表述，只替换 HTTP path 语义。
- [OpenAPI 生成物未同步] 注解变更后生成物可能滞后。→ 实现时运行 `make user-service-openapi-generate` 并检查 `user-service/docs/` diff。
- [命名范围扩大] 全量重命名会影响 mock、OpenAPI schema 名称和测试 helper。→ 使用 `go generate` 重新生成 mocks，运行 OpenAPI 生成和相关 Go 测试捕获遗漏引用。

## Migration Plan

1. 修改 auth public route，将 `POST /change-password` 改为 `POST /force-change-password`。
2. 将强制改密相关 handler、use case、command/result、request/response、preparer、mapper、validator、Fx wiring、mock 和测试 helper 重命名为 `ForceChangePassword` 语义。
3. 更新 controller OpenAPI 注解和面向用户的摘要/说明，使其明确为强制改密。
4. 更新路由注册和 auth middleware 相关测试中的期望路径。
5. 更新 `docs/ARCHITECTURE.md` 和 auth-session-management 规格路径说明。
6. 运行 mock 生成、相关 Go 测试、`make user-service-openapi-generate`、`make user-service-architecture-lint`，最终按仓库要求运行 `make lint` 和 `make verify`。

回滚方式：将路由和 OpenAPI 注解恢复为 `/auth/change-password`，重新生成 OpenAPI，并回滚对应测试、文档和规格变更。由于不涉及数据库、Redis key、token 或响应结构，回滚不需要数据迁移。

## Open Questions

无。
