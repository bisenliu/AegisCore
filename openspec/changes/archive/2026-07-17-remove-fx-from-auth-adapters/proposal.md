## Why

auth 的 PostgreSQL/Redis adapter 和 HTTP controller 构造 API 当前暴露 Fx/Dig metadata，使 feature 内基础设施与 transport 构造方式绑定到服务级 DI 组合细节。移除这些 metadata 可以让认证调用链使用普通参数或无 DI tag 的 feature-local Options 装配，降低构造 API 的框架耦合并强化 auth capability 的分层边界。

## What Changes

- **BREAKING**：移除 `CredentialStoreParams`、`SessionStoreParams` 和 `AuthControllerParams` 中的 `fx.In`、named tag 或其他 Fx/Dig metadata。
- **BREAKING**：auth PostgreSQL credential store、Redis session store 和 HTTP controller constructor 改为显式参数或 feature-local Options；Options 不嵌入 `fx.In`，也不包含 DI tag。
- **BREAKING**：`SessionStore` constructor 不再接收完整 service config，而是只接收 Redis client、Redis key catalog、所需 auth 配置值、metrics 和 logger。
- 更新 user-service 生产 Fx composition，以普通 provider 函数适配新的 auth constructor 签名，不保留旧 constructor 或兼容 wrapper。
- 更新 auth infrastructure 与 HTTP transport 测试，覆盖新构造 API。
- 不迁移 `SessionPurgePool` 或 token-version localcache 的关闭责任。
- 不删除 auth feature 的 Fx module。
- 不改变 token、refresh rotation、session TTL、Redis key、错误码、HTTP API 或安全语义。

## Capabilities

### New Capabilities

- 无。

### Modified Capabilities

- `auth-session-management`：删除对 Fx 输入映射、named tag 和 composition-specific constructor 的稳定要求，新增 auth adapter/controller constructor 不依赖 Fx/Dig metadata 的要求。

## Impact

- 受影响代码：`user-service/internal/features/auth/infrastructure/postgres/credential_store.go`、`user-service/internal/features/auth/infrastructure/redis/session_store.go`、`user-service/internal/features/auth/transport/http/controller.go` 及其测试。
- 受影响装配：user-service auth 相关 Fx provider 需要在 composition 边界显式构造普通参数或 Options。
- 受影响规格：`openspec/specs/auth-session-management/spec.md` 通过 change delta 调整认证能力边界与私有配置要求。
- API 与安全语义：HTTP 路由、请求响应、错误码、token/refresh/session 行为、Redis key 和 TTL 语义不变。
- 数据库与部署：不涉及 Ent schema、Atlas migration、OpenAPI contract、部署资产或运行时 migration 流程。
