# Restructure auth application components

## What

调整 auth feature 的 application 层组织，将当前 `application/command` 包中继续增长的凭证、token 和 session 支撑逻辑提取为稳定 application component 包，同时保持 command/query 作为 use case 入口的扁平结构。

目标结构：

```text
user-service/internal/features/auth/
  application/
    ports.go
    authctx/
      session.go
    command/
      login.go
      refresh_token.go
      change_password.go
      logout_current_session.go
      logout_all_sessions.go
    credentials/
      verifier.go
    sessions/
      lifecycle.go
      revocation.go
    tokens/
      issuer.go
      result.go
    validators/
    query/
      README.md
  domain/
    credential.go
    session.go
    errors.go
  infrastructure/
    postgres/
    redis/
```

本变更迁移和拆分：

- 保持 `application/command` 扁平，只承载登录、刷新、强制改密、退出当前设备和退出全部设备 use case 入口。
- 将凭据校验和强制改密凭据更新逻辑迁入 `application/credentials`。
- 将 JWT 签发、解析、TTL fallback 和 token result DTO 迁入 `application/tokens`。
- 将 refresh session 创建、校验、轮换、删除、撤销和 token version 投影刷新逻辑迁入 `application/sessions`。
- 将认证上下文读取 helper 迁入 `application/authctx`，避免使用泛化 `common` 包名。
- 保持 `application/ports.go` 作为 auth application 层消费侧 ports 根定义。
- 保持 `domain` 只承载纯领域模型、领域错误和值对象方法，不承载 application 组件、日志、密码 hash、JWT 或 Redis session 生命周期。
- 不为对称而创建空业务代码；`application/query` 仅在已有真实查询用例时放实现，否则最多保留 README。

## Why

当前 `auth/application/command` 已同时包含 use case 入口和多个支撑组件：

- 登录、刷新、强制改密、退出当前设备、退出全部设备等 command use case。
- 凭据校验、密码 hash、用户状态映射。
- token 签发/解析、TTL fallback、token result DTO。
- refresh session 生命周期、token version cache/database fallback、会话撤销。
- 认证上下文读取和客户端审计上下文。

这些能力都属于 auth application 层，但不是同一种抽象。继续把所有内容放在 `command` 包会让目录持续膨胀，也会让测试文件和私有 helper 越来越难按职责定位。

按稳定 component 拆分可以带来几个收益：

- `command` 只保留“做什么”的 use case 编排，文件体积和依赖更清晰。
- `credentials`、`tokens`、`sessions` 分别承载“怎么做”的应用组件，可以被 command 和未来 query 复用。
- `authctx` 避免出现 `application/common` 这类泛化杂物包。
- 保持 Go package 边界按职责稳定拆分，不采用“一 use case 一子包”的过早拆分。
- 降低未来新增 auth query、设备管理、MFA 或 OAuth 能力时的迁移成本。

本变更只调整 auth feature 内部 application 层组织，不改变认证 HTTP API、JWT token 格式、Redis key 语义、响应 envelope、错误码、数据库模型或迁移。

## Scope

包括：

- 新增 `application/authctx`、`application/credentials`、`application/sessions` 和 `application/tokens` 包。
- 将 `command/authenticated_session.go` 迁入 `application/authctx/session.go`。
- 将 `command/credentials.go` 迁入 `application/credentials/verifier.go`。
- 将 `command/tokens.go` 中 token 签发解析逻辑迁入 `application/tokens/issuer.go`，将 `TokenResult` 迁入 `application/tokens/result.go`。
- 将 `command/sessions.go` 中 session lifecycle 和 revocation 逻辑拆入 `application/sessions/lifecycle.go`、`application/sessions/revocation.go`。
- 更新 command use case，使其通过 component 包完成凭据、token 和 session 协作。
- 更新 Fx provider，使 component constructor 和 command use case constructor 的依赖关系清晰可注入。
- 更新 HTTP mapper/controller 引用，保持 transport DTO 到 command DTO 的映射不变。
- 更新 auth application 和 transport 测试，按新 component package 拆分测试。
- 更新 `AGENTS.md` 和 `docs/ARCHITECTURE.md` 中 auth application 层说明。
- 运行 `gofmt` 格式化受影响 Go 文件。

不包括：

- 不新增 `openspec/` 或 `docs/opsx/` 工件。
- 不改变 `POST /api/v1/auth/login`、`POST /api/v1/auth/refresh`、`POST /api/v1/auth/change-password`、`POST /api/v1/auth/logout`、`POST /api/v1/auth/logout-all` 的 HTTP API。
- 不改变 request/response JSON 字段、状态码、错误码或 response envelope。
- 不改变 JWT subject、claims、TTL fallback、签名方式、token type 或 Bearer 兼容规则。
- 不改变 Redis key builder、Redis key 前缀、session 存储语义或 token version cache 语义。
- 不改变 Ent schema、Ent generated code、Atlas migration 或数据库结构。
- 不新增团队、角色、权限、MFA、OAuth、设备列表、会话查询 API 或后台 worker。
- 不把 auth feature 专属逻辑移动到 `common`、`internal/shared`、`internal/providers` 或 `integration`。
- 不把 command 拆成 `command/login`、`command/refresh` 这类一用例一 package。

## Acceptance Criteria

- `application/command` 仍为扁平 use case 入口，包含登录、刷新、强制改密、退出当前设备和退出全部设备 command 文件。
- `application/credentials` 存在，并承载凭据校验、密码验证和强制改密凭据更新应用组件。
- `application/tokens` 存在，并承载 JWT 签发、解析、TTL fallback 和 token result DTO。
- `application/sessions` 存在，并承载 refresh session 生命周期、轮换、删除、全部撤销和 token version 投影刷新。
- `application/authctx` 存在，并承载认证上下文读取 helper。
- `application/ports.go` 继续拥有凭据、token version 和 session store ports。
- `domain` 只包含纯领域模型、领域错误和值对象方法，不导入 application、Gin、Ent、Redis、config、logger、JWT 或 password hash helper。
- Command use case 不导入 Gin、HTTP binder、HTTP response、Ent、Redis client 或 SQL。
- Component packages 不导入 Gin、HTTP binder、HTTP response、Ent、Redis client 或 SQL。
- Infrastructure PostgreSQL 和 Redis adapter 仍只实现 application 层拥有的 ports。
- Auth controller 继续把 HTTP DTO 映射为 command DTO 后调用 use case。
- 认证 HTTP API、响应契约、错误映射、JWT token 格式、Redis key 语义保持不变。
- Auth application component tests、command use case tests 和 controller tests 通过。
- 在 `user-service/` 下运行相关测试通过，至少覆盖 `./internal/features/auth/...`。
