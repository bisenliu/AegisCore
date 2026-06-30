## Why

当前登录路径会把 Argon2 KDF 资源池忙碌错误统一映射为无效凭据，导致服务端过载被伪装成用户密码错误，客户端无法正确重试，监控也无法区分真实凭据失败与资源耗尽。随着登录流量和部署规模提升，需要把 KDF busy 作为临时服务不可用语义暴露出来，避免高峰期产生误导性的 401 登录失败。

## What Changes

- **BREAKING**：当密码 KDF 队列达到实例资源上限时，登录接口不再返回无效凭据，而是返回 `503 Service Unavailable`。
- 在认证应用层保留 `password.ErrPasswordKDFBusy` 原因，使 HTTP 边界能够区分 KDF 资源繁忙与真实凭据错误。
- 为 KDF busy 登录失败增加独立 metrics reason，避免污染无效凭据统计。
- 更新认证 HTTP 错误映射、OpenAPI 注解和相关测试，明确登录接口可能因认证服务繁忙返回 503。
- 保留服务内 Argon2 并发和队列门控；本 change 不取消资源保护，也不把 KDF busy 改为网关限流语义。

## Capabilities

### New Capabilities

无。

### Modified Capabilities

- `auth-session-management`：登录凭据校验过程中遇到密码 KDF 资源池繁忙时，认证接口必须返回临时服务不可用，而不是无效凭据。
- `shared-platform-primitives`：共享错误契约需要支持服务不可用响应，以便功能边界表达临时资源耗尽。

## Impact

- API 行为：`POST /api/v1/auth/login` 在 KDF busy 时由 `401 invalid credentials` 变为 `503 Service Unavailable`，属于不兼容响应语义变更。
- 安全边界：真实密码错误仍返回无效凭据；仅 KDF 资源耗尽暴露为服务繁忙，不泄露用户存在性或密码匹配状态。
- 代码路径：影响 `common/contract/errors`、`user-service/internal/features/auth/application/credentials`、`user-service/internal/features/auth/application/command`、`user-service/internal/features/auth/transport/http` 以及相关测试。
- OpenAPI：登录接口需要补充 503 响应说明并重新生成 `user-service/docs/openapi.*`。
- 观测：登录失败 metrics 需要新增 KDF busy reason，用于容量告警和与无效凭据失败分离。
- 部署：不改变数据库 schema、Ent migration、Redis 数据结构、Casbin policy、Kubernetes/Helm 配置和网关限流策略。
