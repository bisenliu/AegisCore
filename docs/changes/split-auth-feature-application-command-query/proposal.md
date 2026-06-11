# Split auth feature application command query

## What

将 auth feature 的 application 层从当前单一 `AuthService` 拆成按认证命令/use case 组织的结构，使登录、刷新、强制改密、退出当前设备和退出全部设备各自拥有清晰的输入、结果和业务编排边界。

目标结构：

```text
user-service/internal/features/auth/
  application/
    command/
      login.go
      refresh.go
      change_password.go
      logout_current_session.go
      logout_all_sessions.go
      credentials.go
      sessions.go
      tokens.go
    validators/
      auth_validator.go
    tokenversion/
      validator.go
    ports.go
  domain/
  infrastructure/postgres/
  infrastructure/redis/
  transport/http/
  fx.go
```

本变更迁移和拆分：

- 创建 `user-service/internal/features/auth/application/command/`，承载登录、刷新、强制改密、退出当前设备和退出全部设备命令/use case。
- 创建 `user-service/internal/features/auth/application/validators/`，承载 transport-neutral 的认证 application 输入校验辅助。
- 将 `LoginCommand`、`RefreshTokenCommand`、`ChangePasswordCommand` 和登出相关 command 类型移动到所属 use case 文件。
- 将 `TokenResult`、`ChangePasswordResult`、`LogoutResult` 移出 application 根部 `result.go`，跟随所属 command/use case。
- 保持 `application/ports.go` 作为 auth feature application 层消费侧 ports 根定义。
- 保持 credential、token、session 三类 application 组件边界清晰，不把凭证校验、token 签发、session 生命周期混入 controller 或 infrastructure adapter。
- 创建 `application/tokenversion/`，承载 access token 中间件使用的 token version 撤销校验和 cache/database fallback 策略，避免把该能力误归入输入 validators 或某个 auth command。
- 更新 HTTP controller 和 mapper，使 controller 继续把 HTTP DTO 映射为 application command 后调用 use case。

Auth 当前没有只读 application 查询用例，因此本变更不为占位而新增空的 `application/query/` 包；后续如果出现真实读侧能力，例如查询当前会话列表，再放入 `application/query/`。

## Why

当前 auth application 根包中的 `AuthService` 同时承载登录、刷新、改密、登出和登出全部设备五个流程，并通过根部 `commands.go`、`result.go` 聚合所有输入输出。随着认证能力扩展，单一 service 会持续混合凭证、token、session、改密状态和 controller 测试替身，导致修改一个认证流程时更容易影响其他流程。

按 command/use case 拆分可以带来几个收益：

- 每个认证流程的输入、结果、依赖和测试更聚焦，降低登录、刷新、改密和登出之间的隐式耦合。
- Controller 依赖明确 use case，而不是一个包含全部认证行为的宽接口。
- `ports.go` 仍由 auth application 层拥有，PostgreSQL 和 Redis adapter 不需要定义接口或承担业务编排。
- Credential、token、session 组件可以作为 command 层内部协作对象保持边界，而不是散落在 controller 或 adapter。
- 移除根部 `result.go` 后，auth feature 与 user feature 的 command/query 组织方式一致。

本变更只调整 auth feature 内部 application 层组织，不改变认证 HTTP API、JWT token 格式、Redis key 语义、响应 envelope、错误码、数据库模型或迁移。

## Scope

包括：

- 新增 `application/command` 和 `application/validators` 目录。
- 将登录用例迁入 command 层，保留普通 token pair 和强制改密受限 token 的签发语义。
- 将刷新用例迁入 command 层，保留 refresh token rotation 开关、非轮换复用 session id、轮换时先签发再原子替换 session 的语义。
- 将强制改密用例迁入 command 层，保留受限 token 校验、凭证更新、token version 投影刷新和会话撤销语义。
- 将退出当前设备和退出全部设备用例迁入 command 层，保留认证上下文读取、当前 session 删除、全部 session 撤销和 token version 递增语义。
- 将根部 `result.go` 的 result 类型移动到对应 command/use case 文件。
- 保持 token version validator 可由服务级 provider 继续使用。
- 更新 auth HTTP controller、mapper、controller tests、Fx module 和 application tests 的引用。
- 更新架构文档中 auth application 层入口与分层说明。
- 运行 `gofmt` 格式化受影响 Go 文件。

不包括：

- 不改变 `POST /api/v1/auth/login`、`POST /api/v1/auth/refresh`、`POST /api/v1/auth/change-password`、`POST /api/v1/auth/logout`、`POST /api/v1/auth/logout-all` 的 HTTP API。
- 不改变 request/response JSON 字段、状态码、错误码或 response envelope。
- 不改变 JWT subject、claims、TTL fallback、签名方式、token type 或 Bearer 兼容规则。
- 不改变 Redis key builder、Redis key 前缀、session 存储语义或 token version cache 语义。
- 不改变 Ent schema、Ent generated code、Atlas migration 或数据库结构。
- 不新增团队、角色、权限、设备管理列表或会话查询 API。
- 不把 auth feature 专属逻辑移动到 `common`、`internal/shared`、`internal/providers` 或 `integration`。
- 不重新新增 `openspec/` 或 `docs/opsx/` 工件。

## Acceptance Criteria

- `user-service/internal/features/auth/application/command/` 存在，并承载登录、刷新、强制改密、退出当前设备和退出全部设备 use case。
- `user-service/internal/features/auth/application/validators/` 存在，并承载 transport-neutral 的 auth application 输入校验辅助。
- `user-service/internal/features/auth/application/tokenversion/` 存在，并承载 token version 撤销校验和 cache/database fallback 策略。
- `user-service/internal/features/auth/application/result.go` 被移除，result 类型跟随具体 command/use case。
- Auth controller 不再依赖一个包含所有认证行为的根部 `AuthService` 宽接口，而是依赖明确的 command/use case 接口或服务。
- `application/ports.go` 继续拥有凭据、token version 和 session store ports。
- Credential、token、session 组件不导入 Gin、HTTP binder、HTTP response、Ent、Redis client 或 SQL。
- Command/use case 包不导入 Gin、HTTP binder、HTTP response、Ent、Redis client 或 SQL。
- Infrastructure PostgreSQL 和 Redis adapter 仍只实现 application 层拥有的 ports。
- 认证 HTTP API、响应契约、错误映射、JWT token 格式、Redis key 语义保持不变。
- Auth application service/use case 测试通过。
- Auth session/credential/token 组件测试通过。
- Auth controller 测试通过。
- 在 `user-service/` 下运行相关测试通过，至少覆盖 `./internal/features/auth/...`。
