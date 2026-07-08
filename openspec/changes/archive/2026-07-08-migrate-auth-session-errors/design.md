## Context

认证登录、refresh、改密、退出和会话撤销路径当前会返回 `authdomain.ErrInvalidCredentials`、`ErrMissingSession`、`ErrTokenInvalid`、`ErrAuthSessionNotFound`、`ErrAuthSessionMismatch`、`ErrSessionRevocationIncomplete`，以及来自 `common/security/password` 的 `ErrPasswordKDFBusy`。这些错误多数仍是普通 sentinel error，auth HTTP controller 需要通过 `toAuthHTTPError` 重复判断 auth domain、identity 和 password 错误后再交给 `response.Fail` 渲染。

共享 response helper 已经通过 `contracterrors.FromError` 识别应用错误，且 `contracterrors.Error` 支持按 `Kind` 和 `Reason` 进行 `errors.Is` 匹配。user、permission、role 的错误迁移也已经采用 feature-local domain 直接定义应用错误、HTTP controller 只调用 `response.Fail(c, err)` 的模式。本变更把 auth 迁移到同一模式，但不改变 token、session、Redis key 或强制改密成功响应结构。

## Goals / Non-Goals

**Goals:**

- 将认证、会话、token、撤销和用户状态拒绝错误定义为可由共享 response helper 直接渲染的应用错误。
- 为每类认证错误固定稳定 `Reason`、`Kind`、`Code` 和中文公开消息。
- 调整 `common/security/password.ErrPasswordKDFBusy` 的应用错误表达，使 KDF 繁忙可直接渲染为 `503 Service Unavailable`，且继续支持 `errors.Is` 匹配。
- 保留登录、refresh、logout metrics 通过 `errors.Is` 或应用错误 `Reason` 分类的能力。
- 删除 auth HTTP transport 中仅用于错误翻译的 mapper，controller 的 use case 失败统一使用 `response.Fail(c, err)`。
- 更新 `auth-session-management` 和 `shared-platform-primitives` delta，固化迁移后的稳定契约。

**Non-Goals:**

- 不调整登录强制改密成功分支响应结构。
- 不迁移 user、role、permission 错误。
- 不改变 JWT claims、refresh session 存储结构、token version 语义、Redis key schema、API 路由、请求 DTO 或成功响应 data。
- 不保留 `toAuthHTTPError` 或任何等价兼容函数。
- 不新增跨模块认证错误映射注册表。

## Decisions

1. 在 `auth/domain` 直接定义认证应用错误。
   - 选择：无效凭据和用户状态拒绝使用 `KindUnauthenticated`、`CodeUnauthenticated` 和无效凭据公开消息；缺失会话使用 `KindUnauthenticated`、`CodeUnauthenticated` 和登录状态失效公开消息；token 无效、refresh session 未找到、session mismatch、password-change session 缺失或 mismatch 使用 `KindUnauthenticated`、`CodeTokenInvalid` 和登录状态失效公开消息；撤销不完整使用 `KindServiceUnavailable`、`CodeServiceUnavailable` 和退出登录尚未完全生效公开消息。每个错误使用独立 `Reason`。
   - 理由：错误的业务归属仍在 auth domain，HTTP 渲染由共享契约完成；独立 `Reason` 可以让 metrics 和测试区分不同认证失败，不需要在 HTTP 层重复维护映射。
   - 备选：保留普通 sentinel error 并继续在 `toAuthHTTPError` 映射。该方案保留重复边界逻辑，不满足本次收敛目标。

2. `ErrPasswordKDFBusy` 在 password primitive 中表达为应用错误。
   - 选择：`common/security/password` 将 `ErrPasswordKDFBusy` 定义为业务中立的服务不可用应用错误，使用 `password_kdf_busy` reason、`KindServiceUnavailable`、`CodeServiceUnavailable` 和不泄露资源预算的公开消息。
   - 理由：KDF busy 是 password primitive 自身的资源预算错误，不应由 user-service auth HTTP mapper 才能变成 503；保持该错误在 common 中业务中立，可以被其他服务复用。
   - 备选：auth credentials 捕获 busy 后包装成 auth domain 错误。该方案仍要求认证层知道 password primitive 的响应契约，且容易丢失 `errors.Is(err, password.ErrPasswordKDFBusy)` 语义。

3. metrics 分类继续使用 `errors.Is`，必要时补充 `Reason` 分类。
   - 选择：现有 login/refresh/logout 分类优先保留 `errors.Is`；如果某些路径需要区分被包装后的应用错误，可通过 `errors.As` 读取 `contracterrors.Error.Reason`。
   - 理由：应用错误变量本身支持 `errors.Is` 语义，迁移不应削弱现有 metrics 的低基数 reason。
   - 备选：仅按错误字符串或 HTTP code 分类。该方案不稳定，且会把不同认证失败合并到同一 code。

4. controller 只调用 `response.Fail(c, err)`。
   - 选择：登录、refresh、改密、退出当前会话、退出全部会话 controller 对 use case 返回错误不再调用认证专用 mapper；强制改密成功分支仍保留现有专用成功 envelope 映射。
   - 理由：业务失败由错误自身携带契约信息，未知错误仍由 `contracterrors.FromError` 归一化为内部错误；强制改密成功分支不是 error，仍需要保留 token data 的特殊 envelope。
   - 备选：保留一个薄的 `toAuthHTTPError` 包装 `contracterrors.FromError`。这仍是等价兼容函数，且没有额外稳定语义。

5. 不调整生成物、数据库和部署资产。
   - 选择：不运行 Ent migration 或 OpenAPI generate，除非实现过程中发现注解变化。
   - 理由：本变更不改变数据库结构、HTTP 路由、请求/响应 schema、OpenAPI 注解或部署配置。

## Risks / Trade-offs

- [Risk] 多个认证错误如果复用通用 `ReasonTokenInvalid`，metrics 和测试会失去细分语义。-> Mitigation：auth domain 为每个稳定认证错误定义独立 `Reason`，同时保留共享 `Kind` 和 `Code`。
- [Risk] `ErrPasswordKDFBusy` 从普通 sentinel 变为应用错误后，测试可能依赖旧英文 `Error()` 文本。-> Mitigation：测试改用 `errors.Is`、`errors.As`、`Kind`、`Reason`、`Code` 和公开 message 断言。
- [Risk] 用户状态拒绝直接作为应用错误返回后可能泄露状态信息。-> Mitigation：该错误继续使用无效凭据公开消息和 unauthenticated code；metrics 通过 `errors.Is` 或 `Reason` 内部区分。
- [Risk] 删除 mapper 后未知错误路径渲染方式变化。-> Mitigation：`response.Fail` 已统一调用 `contracterrors.FromError`，未知错误继续渲染为 `500 Internal Server Error`。

## Migration Plan

1. 更新 `common/security/password` 的 KDF busy 错误定义和测试，确认 Argon2id 参数、哈希编码、并发上限、队列上限和 busy 触发语义不变。
2. 更新 `auth/domain/errors.go` 的应用错误定义和必要测试，覆盖直接返回和包装后的 `errors.Is`。
3. 调整 auth credentials、sessions、validators 和 command 中错误返回或分类路径，确保业务失败返回可直接渲染的应用错误，metrics 分类仍稳定。
4. 调整 auth HTTP controller，删除认证错误 mapper，使 use case 失败统一 `response.Fail(c, err)`；保留强制改密成功 envelope。
5. 更新 auth HTTP、command、credentials、sessions、validators 和 password 测试，覆盖无效凭据 401、缺失或无效 session/token 401、撤销不完整 503、KDF busy 503，以及 mapper 不存在。
6. 运行 `gofmt`、`go test ./common/security/password/... ./user-service/internal/features/auth/...` 和 `make user-service-architecture-lint`。

回滚方式：恢复 auth domain 和 password busy 的旧 sentinel error、恢复 auth HTTP mapper 和对应测试。由于没有数据库、配置、OpenAPI schema 或部署资产变更，回滚不涉及数据迁移。

## Open Questions

无。
