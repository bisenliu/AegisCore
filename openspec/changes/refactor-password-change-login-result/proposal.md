## Why

强制改密登录当前需要向前端表达“凭据有效，但必须先完成密码修改”。继续在登录响应 data 中返回 `status=password_change_required` 会把流程控制放进业务载荷，前端需要同时解析 token 数据和登录状态。使用稳定 envelope 业务码 `CodePasswordChangeRequired=20006` 可以让调用方按统一响应信封分支处理，同时仍在 data 中交付受限 `password_change` token。

## What Changes

- **BREAKING**：登录响应 data 不再返回 `status` 枚举字段，也不保留旧 `password_change_required` 兼容字段。
- 强制改密登录返回 HTTP `200 OK`、`success=false`、`code=20006`、`message=PasswordChangeRequired`，并在 data 中返回受限改密 token。
- 普通登录继续返回 HTTP `200 OK`、`success=true`、`CodeOK`，并在 data 中返回 access token、refresh token、token type 和 expires_in。
- `common/contract/errors` 保留并规范化 `CodePasswordChangeRequired=20006`，作为认证类稳定业务码；不新增 `ReasonPasswordChangeRequired` 或错误构造函数。
- auth application 返回值使用登录 use case 专属 `LoginResult.PasswordChangeRequired` 表达是否需要强制改密；`TokenResult` 只表达 token 载荷。
- auth HTTP transport 使用专用 `toPasswordChangeRequiredEnvelope` mapper 渲染强制改密响应，普通登录、强制改密和 refresh 共用 `TokenResponse` token DTO。
- 更新 auth transport DTO、OpenAPI 注解、controller 测试、login use case 测试和 E2E flow 断言。
- 更新 `auth-session-management`、`shared-platform-primitives` 和 `runtime-observability` delta，使规格与最终 HTTP 契约一致。

## Capabilities

### New Capabilities

无。

### Modified Capabilities

- `auth-session-management`: 登录 use case 结果模型、强制改密登录 HTTP envelope、auth transport DTO/OpenAPI 注解和相关测试约定发生变化。
- `shared-platform-primitives`: 共享错误码契约新增或保留认证类业务码 `CodePasswordChangeRequired=20006`，用于稳定表达强制改密流程状态。
- `runtime-observability`: user-service OpenAPI 输出需要同步表达登录接口普通登录和强制改密登录的 envelope 差异。

## Impact

- 影响代码：`common/contract/errors`、`user-service/internal/features/auth/application/command` 登录 use case、`user-service/internal/features/auth/transport/http` controller、DTO、mapper、OpenAPI 注解和 controller 测试，以及 E2E HTTP flow 断言。
- API 行为：普通登录继续返回 access token、refresh token 和会话过期信息；强制改密登录继续只签发 subject 为 `password_change` 的受限 token，不创建 refresh session、不返回 refresh token，但登录响应 data 不再包含状态枚举，分支由 envelope `code=20006` 表达。
- OpenAPI：登录接口文档需要声明普通登录 `success=true/code=0` 与强制改密登录 `success=false/code=20006` 两种响应语义，并且登录接口复用 `TokenResponse`，不再声明单独的 `LoginResponse`。
- 不影响范围：不改变强制改密 token 的签发、校验、TTL、一次性 session 存储、改密流程、普通 refresh/logout 业务逻辑、Ent schema、Atlas migration、部署资产或 Redis key schema。
