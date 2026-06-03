## Context

`common` 当前已有三类与身份凭证直接相关的共享代码：`common/password` 提供 Argon2id 密码 hash 与校验，`common/jwt` 提供 JWT token 签发和解析，`common/contextutil/auth.go` 提供 Authorization/Bearer 常量以及认证 user/session 在 `context.Context` 中的绑定。`common/middleware/auth.go`、`user-services/internal/service/auth_service.go`、用户创建服务和相关测试都直接引用这些分散包。

这些代码长期看属于同一认证边界：凭证的产生、验证、传输表达和认证上下文传播。继续分散在 `jwt`、`password`、`contextutil` 会让调用方需要记忆多个包职责，也会让后续 MFA、OIDC、API Signature 等凭证类型缺少统一归属。

本变更是 common 模块内的包组织重构和共享凭证 capability 建立，不涉及数据库、Ent schema、Atlas migration、HTTP 路由、响应 envelope、错误码或配置结构变更。

## Goals / Non-Goals

**Goals:**

- 在 `common/credentials` 下建立统一 Go 包，采用 `password.go`、`jwt.go`、`context.go` 平铺文件组织。
- 将密码 hash/verify、JWT service/claims/sign input、Authorization/Bearer 常量和认证上下文 helper 迁移到 `credentials` 包。
- 更新 common 与 user-services 内部调用方，避免新代码继续依赖 `common/password`、`common/jwt`、`common/contextutil` 中的认证相关 API。
- 保留现有 Argon2id hash 格式、JWT claims/subject 校验、HTTP 401 响应语义、token type 值、Authorization header 名称和 context key 值。
- 用测试覆盖迁移后的公共 API 和认证中间件行为，确保重构不改变外部可观察行为。

**Non-Goals:**

- 不新增 MFA、OAuth/OIDC、API Signature、密码复杂度校验或新的认证协议。
- 不改变 `config.AuthConfig` 字段、YAML key、`AEGISCORE_` 环境变量覆盖或 JWT secret/issuer/audience 语义。
- 不改变用户登录、刷新、退出、修改密码等业务流程。
- 不修改 Ent schema、生成代码或数据库 migration。
- 不为 external Go module 消费者设计长期兼容 shim，除非实现阶段发现仓库内仍有必要的迁移过渡点。

## Decisions

1. 采用 `common/credentials` 单包平铺文件方案。

   原因：密码凭证、Bearer token 和认证上下文都属于凭证生命周期的基础原语，统一包可以让调用方通过一个导入路径发现和使用认证边界能力。平铺文件能保持维护粒度清晰，同时避免 `credentials/password`、`credentials/jwt` 这类二级包带来的重复命名。

   备选方案：保留 `common/password`、`common/jwt`、`common/contextutil`。该方案改动最小，但不能解决能力归属分散问题。另一个方案是 `common/credentials/password`、`common/credentials/jwt` 子包，但调用方仍需多个 import，且包名与目录名重复度高。

2. 使用更明确的凭证 API 名称。

   密码 API 在新包中使用 `HashPassword` 和 `VerifyPassword`，避免在 `credentials` 包中暴露过于泛化的 `Hash`、`Verify`。JWT 构造函数使用 `NewJWTService`，避免 `credentials.NewService` 无法表达服务类型。类型保留 `Claims`、`SignInput`、`Service` 或按实现需要命名为 `JWTService`，但调用方入口必须清晰表达 JWT 凭证语义。

   备选方案：完全保留 `Hash`、`Verify`、`NewService`。该方案迁移机械成本低，但会在聚合包中制造含义不清的公共 API。

3. 认证传输常量和认证上下文 helper 迁移到 `credentials`，trace-id 等非认证 context helper 保持在原职责包。

   原因：`Authorization`、`Bearer `、`user_id`、`session_id` 是认证凭证传输和认证主体传播的一部分，适合进入 `credentials`。但 trace-id 属于请求追踪和日志边界，不应因为同属 context helper 而被迁移到 credentials。

   备选方案：迁移整个 `contextutil` 包。该方案会把 trace-id 等非凭证概念混入 credentials，扩大重构范围并弱化包职责。

4. 以行为兼容为迁移约束，而不是保留旧导入路径为目标。

   本仓库是同一 Go workspace 内协作开发，当前变更可以直接更新仓库内调用方到新包名。若实现阶段发现旧包仍被大量内部测试或生成逻辑依赖，可以短期保留薄 wrapper，但 wrapper 不应成为长期规范来源，主规格以 `common/credentials` 为准。

   备选方案：永久保留旧包公共 API 并转发。该方案更保守，但会让新旧来源并存，削弱本次 consolidation 的收益。

## Risks / Trade-offs

- [Risk] 大量 import 和类型名变更可能引入机械性编译错误 -> Mitigation：先迁移 `common` 包测试，再迁移 `user-services` 调用方，最后分别运行 `go test ./...`。
- [Risk] JWT 或密码 API 改名可能遗漏测试中的 helper -> Mitigation：使用全仓搜索 `common/jwt`、`common/password`、`common/contextutil`、`commonjwt`、`commonpassword` 并逐一清理。
- [Risk] 认证上下文 key 类型迁移后旧值不可互通 -> Mitigation：所有写入和读取点必须同批迁移到 `credentials`，并通过 middleware/service 测试验证 `user_id` 与 `session_id` 传播。
- [Risk] `contextutil` 中仍有非认证工具时误删或误迁移 -> Mitigation：只迁移 `auth.go` 中认证相关常量和函数；trace-id/logging context 保持原职责边界。
- [Trade-off] 新 API 名称更长但语义更明确 -> Mitigation：接受少量调用代码冗长，换取聚合包中的可读性和未来扩展空间。
