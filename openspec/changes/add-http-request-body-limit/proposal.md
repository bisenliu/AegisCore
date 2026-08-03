## Why

当前入站 JSON 请求体在进入 binder 解码前没有统一字节上限，公开认证和用户写接口可被超大固定长度、chunked 或尾随 JSON 载荷在业务校验前消耗大量内存与 CPU。user-service Pod 默认内存上限为 `512Mi`，该缺口会使少量并发恶意请求触发 GC 抖动、OOMKill 与级联故障，因此上线前必须补齐服务内 HTTP 入站容量边界。

## What Changes

- 在 HTTP 入站最外层引入可配置请求体字节上限，默认覆盖 JSON 写请求，避免每个 controller 单独记忆限制。
- 在 `common/http/binding` 或共享 HTTP middleware 中提供业务中立的 body limit 机制，并在超限时生成稳定可映射的应用错误语义。
- 将 `*http.MaxBytesError` 或等价超限错误映射为 `413 Payload Too Large`，保持统一 response envelope，不进入 feature use case。
- 为 user-service 增加服务私有默认配置和必要校验，允许少量端点按路由或分组覆盖上限。
- 更新 Nacos 本地环境配置中的默认请求体上限；Compose 与 Helm 继续只负责选择 Nacos 配置来源，不新增业务字段环境变量覆盖。
- 增加固定长度、chunked 与尾随 JSON 超限测试，覆盖公开 auth 和受保护 user 写接口。

## Capabilities

### New Capabilities

### Modified Capabilities

- `shared-platform-primitives`: 调整共享 HTTP helper 与错误映射契约，要求入站 JSON 绑定在解码前具备硬字节上限并将超限渲染为 `413`。
- `auth-session-management`: 调整认证 HTTP 契约，要求公开认证入口在 controller 和 use case 前拒绝超限请求体。
- `user-identity-management`: 调整用户写接口 HTTP 契约，要求创建用户等写请求在业务处理前拒绝超限请求体。
- `delivery-operations`: 调整运行与部署配置契约，要求 user-service 默认配置和 Nacos 环境资产声明请求体上限。

## Impact

- 影响代码：`common/http/binding`、可能新增或调整 `common/http/middleware`、`common/contract/errors`、`common/http/response`、`user-service/internal/config`、`user-service/internal/providers/transport`、`user-service/internal/bootstrap`、auth/user HTTP controller 测试。
- 影响配置与部署：user-service 本地配置与 Nacos 配置目录；Compose 和 Helm 保持仅通过环境变量定位 Nacos，不覆盖业务字段。
- 影响 HTTP 行为：超出配置上限的 JSON 请求返回 `413 Payload Too Large` 和稳定错误 envelope；合法小请求、空 body、字段校验、未知字段和尾随小 JSON 的既有语义保持不变。
- 不影响数据库 schema、Ent 生成物、OpenAPI 路径、认证 token 语义、RBAC 策略或外部系统协议。
