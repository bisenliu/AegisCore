## Why

强制改密登录当前只签发携带 `session_id` 的无状态 `password_change` token，没有服务端一次性会话约束，且 token TTL 复用 `access_token_ttl`。改密成功后的 token version 投影刷新和 refresh session 删除失败会被记录但仍向调用方返回成功，导致安全敏感撤销存在不可见的部分成功窗口。

## What Changes

- **BREAKING** 强制改密 token 使用独立 `auth.jwt.password_change_token_ttl`，默认和推荐值为 5 分钟，不再复用 `access_token_ttl`。
- **BREAKING** 强制改密登录必须创建 Redis 一次性 password-change session，并将 `jti`、`session_id`、`user_id` 和 `token_version` 绑定到服务端状态。
- **BREAKING** `ChangePassword` 必须在更新密码前原子消费 password-change session；复用、过期、撤销、并发消费、claims 不一致均返回统一无效凭据。
- **BREAKING** 强制改密凭据更新必须按旧 `token_version` 和强制改密状态做条件更新，防止同一 token 并发改密双写。
- **BREAKING** 改密成功后若 token version 投影刷新、本地缓存失效或 refresh session 删除失败，系统不得返回普通 `Changed: true` 成功结果，必须返回可观察的安全撤销未完成错误，并记录可告警指标。
- 增加强制改密一次性会话、投影失败、撤销补偿入队或失败的测试与观测指标。

## Capabilities

### New Capabilities

- 无。

### Modified Capabilities

- `auth-session-management`: 收紧强制改密 token TTL、一次性服务端会话、原子消费、并发防护和改密后撤销失败语义。
- `runtime-observability`: 增加强制改密一次性会话消费失败、复用拒绝、撤销投影失败和补偿失败的指标与告警要求。

## Impact

- 影响 Go 代码：`common/runtime/config`、`common/security/auth`、`user-service/internal/features/auth/application/*`、`user-service/internal/features/auth/infrastructure/redis`、`user-service/internal/features/auth/infrastructure/postgres`、auth Fx provider 和相关测试。
- 影响配置：新增 `auth.jwt.password_change_token_ttl`，示例配置和配置校验需要同步更新。
- 影响 Redis：新增 password-change session key、原子创建/消费/撤销脚本和 TTL 策略；不复用 refresh session key schema。
- 影响 HTTP API 行为：`POST /api/v1/auth/change-password` 在撤销投影失败时不再返回普通成功，复用或过期 password-change token 统一返回无效凭据。
- 影响 OpenAPI：改密接口错误响应说明需要同步，登录响应的 `expires_in` 语义改为 password-change token 独立 TTL。
- 影响观测：新增 Prometheus 指标和告警规则，覆盖 password-change session 消费失败、复用拒绝、投影失败和补偿失败。
- 不影响数据库 schema；凭据条件更新使用现有 `users.status` 和 `users.token_version` 字段完成。
