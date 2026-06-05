## Context

`user-session-control` 当前由 `user-services/internal/service/auth_service.go` 承担大部分实现：登录凭证认证、密码 hash 校验、受限改密 token 校验、Access/Refresh token 签发、Refresh Token rotation、Redis 会话创建和删除、token_version 校验与失效、认证上下文解析都集中在一个服务中。该文件已经成为认证流程的用例入口和底层策略实现的混合体，导致测试需要覆盖大量交叉场景，也让后续新增认证策略时难以独立演进。

本设计只调整 `user-services/internal/service` 内部职责边界。controller 仍负责 HTTP 解析和响应输出，repository 仍负责 PostgreSQL/Redis 数据访问，common 仍提供 JWT、密码、响应、日志等共享基础能力。HTTP API、DTO、Redis key、token claims、错误响应和配置字段保持兼容。

## Goals / Non-Goals

**Goals:**

- 保留 `AuthService` 作为认证相关用例统一入口，负责登录、改密、刷新、退出当前设备和退出全部设备的高层编排。
- 将登录凭证校验拆为 `CredentialVerifier`，隔离用户资料读取、密码验证和无效凭证错误映射。
- 将 token 签发和 token claims 验证拆为 `AuthTokenIssuer` 或等价组件，隔离 TTL 兜底、JWT 签发、Refresh Token 解析和 Password-Change Token 解析。
- 将 Redis 会话生命周期与 token_version 会话校验拆为 `AuthSessionManager` 或等价组件，隔离会话创建、刷新轮转、删除、全部失效和当前 token_version 读取。
- 使 `AuthService` 的单元测试可以聚焦用例编排，组件测试可以分别覆盖 token、session、credential 策略。

**Non-Goals:**

- 不新增认证接口、授权模型、角色权限、MFA、设备管理或账号锁定策略。
- 不修改 HTTP 路由、DTO、响应信封、错误码、Swagger 契约或配置字段。
- 不修改 Ent schema、数据库结构、Redis key 格式、Refresh Token 会话 TTL 或 token_version 持久化语义。
- 不将服务特定会话业务移动到 `common`；`common/security/auth` 继续只提供共享 JWT/token 原语和认证上下文 helper。

## Decisions

### Decision: AuthService 只保留用例编排

`authService` 保留 `AuthService` 接口并继续由 Fx 注入给 controller。其字段调整为依赖 `UserRepository`、`CredentialVerifier`、`AuthTokenIssuer`、`AuthSessionManager` 和认证上下文解析组件。登录流程编排为：凭证校验 -> 根据用户状态选择签发受限改密 token 或普通 token pair；改密流程编排为：验证改密 token -> 读取用户状态 -> hash 新密码并更新凭证 -> 失效 token version 缓存；刷新流程编排为：验证 Refresh Token 和会话 -> 按配置轮转会话 -> 签发新 token pair。

替代方案是只把私有方法移动到同一文件底部。该方案虽然减少函数长度，但无法形成可替换依赖，也无法让测试独立覆盖策略组件，因此不采用。

### Decision: 凭证校验组件负责认证资料和密码策略

新增 `CredentialVerifier`，依赖 `repository.UserRepository`，方法建议为 `VerifyPassword(ctx, username, plainPassword string) (*domain.User, error)`。该组件负责 `GetByUsername`、`password.Verify`、用户不存在/密码错误/hash 错误/禁用状态的统一无效凭证映射，并保持 `status=300` 在密码校验通过后视为认证成功，由 `AuthService` 决定后续签发受限改密凭据。

替代方案是让 repository 返回“已认证用户”。该方案会把密码校验和响应错误映射推入数据访问层，破坏 service/repository 分层，因此不采用。

### Decision: token 组件负责 JWT 策略，不负责 Redis 会话持久化

新增 `AuthTokenIssuer`，依赖 `*auth.JWTService` 和 `*config.Config`。它负责 Access Token、Refresh Token、Password-Change Token 的 TTL 兜底与签发，并负责解析 Refresh Token 和 Password-Change Token 的 claims 基础校验，包括 `auth.StripBearerPrefix`、subject 校验和 `user_id` UUID 解析。普通 token pair 签发返回 token 内容和使用的 Refresh Token TTL，让会话管理组件按相同 TTL 创建 Redis 会话。

替代方案是 token 组件直接创建 Redis 会话。该方案会把 JWT 签发和会话存储耦合在一起，不利于后续替换会话策略或独立测试 token 签发，因此不采用。

### Decision: session 组件负责会话生命周期与 token_version 语义

新增 `AuthSessionManager`，依赖 `repository.AuthSessionRepository`。它负责创建 Refresh Token 会话、校验 Refresh Token claims 对应的 Redis 会话、读取当前 token_version、执行 Refresh Token rotation 的旧会话删除、退出当前设备、退出全部设备后的 Redis 清理以及改密后的 token version 缓存失效。用户级 `token_version` 的 PostgreSQL 递增仍由 `AuthService` 调用 `UserRepository`，因为这是用户安全状态持久化更新，不属于 Redis 会话组件独立决定的行为。

替代方案是把 logout-all 全部封装进 session 组件。该方案会迫使 session 组件依赖 `UserRepository` 并更新 PostgreSQL token_version，扩大组件边界，因此不采用。

### Decision: 认证上下文解析保持 service 内部小组件

当前 `authenticatedSession(ctx)` 可保留为 service 包内函数，或拆为 `AuthenticatedSessionResolver`。无论采用哪种形式，它只从 `common/security/auth` 上下文读取并校验 `user_id` 和 `session_id`，不访问 repository 或执行业务校验。为最小化改动，优先保留函数；如测试需要可再抽接口。

替代方案是把上下文解析下沉到 controller。该方案会让 controller 承担认证会话语义，违反 controller/service 分层，因此不采用。

## Risks / Trade-offs

- [Risk] 拆分后组件数量增加，初期文件和类型更多。→ Mitigation: 仅在 `service` 包内新增少量窄接口和实现，不引入新包层级，避免过度抽象。
- [Risk] 错误映射分散后可能改变现有响应语义。→ Mitigation: 组件方法继续返回 `response.*` 应用错误，保持现有日志和错误消息，重构时优先搬移原逻辑而不是重写逻辑。
- [Risk] Refresh Token rotation 同时涉及 token 签发和 Redis 会话，拆分可能造成调用顺序错误。→ Mitigation: `AuthService.Refresh` 明确编排顺序：先 token claims，再 session/token_version 校验，再按配置删除旧会话并生成新 session_id，最后签发并创建新会话。
- [Risk] Fx 注入变复杂。→ Mitigation: 新组件在 `NewAuthService` 内由现有 `AuthServiceParams` 组装，初期不要求 controller 或 bootstrap 注入额外类型；后续测试需要时再公开 providers。
