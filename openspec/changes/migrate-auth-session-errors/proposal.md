## Why

认证、会话、token、撤销和密码 KDF 繁忙错误当前仍依赖 auth HTTP transport 的 `toAuthHTTPError` 做 sentinel-to-HTTP 翻译，导致 domain、identity、password 等边界的错误语义重复散落在 HTTP 层。将这些错误迁移为携带稳定 `Kind`、`Reason`、`Code` 和中文公开消息的应用错误，可以让认证 controller 统一通过 `response.Fail(c, err)` 渲染失败响应，并保留 command metrics 所需的 `errors.Is` 或 `Reason` 分类能力。

## What Changes

- **BREAKING**：删除认证 HTTP transport 中仅用于认证错误翻译的 `toAuthHTTPError` 或等价兼容函数，不保留旧 sentinel-to-HTTP 映射兼容方案。
- 将 `user-service/internal/features/auth/domain/errors.go` 中的无效凭据、缺失会话、token 无效、refresh session 未找到、session mismatch、撤销不完整、用户状态拒绝等认证错误定义为应用错误，并固定稳定 `Reason`、`Kind`、`Code` 和中文公开消息。
- 评估并调整 `common/security/password.ErrPasswordKDFBusy` 的应用错误表达，使认证层遇到 KDF 资源繁忙时可直接透传为 `503 Service Unavailable`，同时继续支持 `errors.Is(err, password.ErrPasswordKDFBusy)`。
- 调整 auth command、credentials、sessions、validators、transport 和相关测试，使 use case 失败统一返回可由 `response.Fail(c, err)` 直接渲染的错误。
- 更新 `auth-session-management` delta，明确无效凭据、缺失或无效 session/token、撤销不完整、KDF 繁忙和 controller 错误出口的稳定行为。
- 更新 `shared-platform-primitives` delta，明确共享 password KDF busy 错误可作为应用错误直接被共享 response helper 渲染，且 common 不引入 user-service 专用认证错误 mapper。

## Capabilities

### New Capabilities

无。

### Modified Capabilities

- `auth-session-management`: 认证、会话、token、撤销错误的应用错误表达、HTTP 渲染行为、metrics 分类语义和 auth HTTP controller 统一错误出口发生变化。
- `shared-platform-primitives`: password KDF busy 作为共享安全 primitive 的应用错误表达发生变化，并要求共享 response helper 可直接渲染该错误。

## Impact

- 影响代码：`user-service/internal/features/auth/domain/errors.go`、auth application command、credentials、sessions、validators、transport HTTP controller/mapper 和相关测试，以及 `common/security/password` 的错误定义或包装 helper。
- API 行为：无效凭据继续返回 `401 Unauthorized` 和 unauthenticated 语义；缺失或无效 session/token 返回 `401 Unauthorized`，并按场景使用 token invalid 或 unauthenticated 语义；撤销不完整和 KDF 繁忙返回 `503 Service Unavailable`。不改变登录强制改密成功分支响应结构、JWT claims、refresh session 存储结构、token version 语义或 Redis key schema。
- 共享契约：复用现有 `common/contract/errors` 与 `common/http/response` 应用错误渲染能力，不新增跨模块认证错误映射注册表，不迁移 user、role、permission 错误。
- 数据库、OpenAPI 与部署：不修改 Ent schema、Atlas migration、HTTP 路由、请求 DTO、成功响应 data 结构、OpenAPI 注解或部署资产。
