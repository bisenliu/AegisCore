## Why

当前 `common` 共享层同时承载 user-service 私有认证配置、用户数据库资源名、JWT 签发能力以及 refresh/password-change 业务语义，违背跨服务共享模块只提供无业务语义 primitive 的边界。随着后续服务复用 `common`，这些能力会把用户服务签发权限和会话模型扩散到更多服务，扩大安全面并推动架构走向分布式单体。

## What Changes

- **BREAKING**：从 `common/runtime/config.Config` 移除 `Auth` 和 `Ent`，共享配置只保留跨服务通用 runtime 配置块。
- **BREAKING**：将 user-service 的 `AuthConfig`、`JWTConfig`、`PasswordKDFConfig`、`EntConfig` 和相关校验下沉到 user-service 私有配置包。
- **BREAKING**：从 `common/runtime/resources` 移除 `NameUserDB`，用户数据库和服务级 Redis 资源名由 user-service 私有包声明。
- **BREAKING**：将 access、refresh、password-change token 签发能力、subject 常量、用户会话 claims 和 TTL fallback 下沉到 user-service auth feature。
- **BREAKING**：将 `common/security/auth` 收敛为通用 JWT verifier/loader primitive，不再暴露服务级签发 API 或 user-service 专属 claims。
- 调整共享 HTTP auth middleware，使其依赖最小 verifier 接口，而不是具备签发能力的 concrete JWT service。
- 调整 user-service Fx provider、auth application、RBAC CLI、datastore/Ent provider 和测试，使用服务私有配置与资源名。

## Capabilities

### New Capabilities

- 无。

### Modified Capabilities

- `shared-platform-primitives`：共享层安全、配置和资源 primitive 的边界收敛，移除 user-service 私有认证、Ent 与用户数据库命名语义。
- `auth-session-management`：认证会话的 issuer、token subject、用户会话 claims、TTL 与强制改密/refresh 策略由 user-service 私有边界拥有。

## Impact

- 影响 `common/runtime/config`、`common/runtime/resources`、`common/security/auth`、`common/http/middleware` 的公开 Go API，属于破坏性变更。
- 影响 user-service 配置加载、Fx provider、auth token 签发与解析、token version 校验、password KDF 构造、Ent SQL debug 配置、RBAC CLI 数据库连接。
- 影响 `user-service/configs/config.yaml`、Kubernetes/Helm configmap 中配置结构的加载归属，但不改变现有配置字段名称和部署键名。
- 不改变 HTTP API 路径、请求/响应 DTO、OpenAPI 契约和数据库 schema。
- 不引入兼容 alias、旧 constructor 或旧资源名转发；所有误用通过编译错误暴露。
