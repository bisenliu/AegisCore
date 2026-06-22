## Why

`common/runtime/config` 当前把 `local_cache` 写死为 `AuthTokenVersion` 和 `RBACUserRoles` 两个具名字段，使跨服务配置层携带了 user-service 的业务缓存语义。需要把共享配置模型改为通用缓存实例 map，同时让 user-service 在自身 provider/feature 边界声明并校验必需缓存实例。

## What Changes

- 将 `LocalCacheConfig` 从固定字段结构改为 `map[string]LocalCacheInstanceConfig` 风格的通用实例集合。
- 从 `common/runtime/config` 移除 `auth_token_version` 和 `rbac_user_roles` 的业务字段、业务名校验和专用读取假设。
- 将 `local_cache` 中所有 entry 的容量、TTL、load timeout、num counters 和 buffer items 校验改为通用遍历。
- 保持配置文件结构和默认值不变，`user-service/configs/config.yaml` 仍使用 `local_cache.auth_token_version` 与 `local_cache.rbac_user_roles`。
- 由 user-service 在 feature/provider 层使用本服务常量读取 `auth_token_version`、`rbac_user_roles`，并在缺少必需缓存实例时明确报错。
- 更新 loader、env override、validation、bootstrap/e2e 和相关 feature/provider 测试。
- 不改变 `localcache.Cache` 运行时行为，不新增动态缓存发现或自动注册机制，不改变外部 HTTP API、OpenAPI、Ent schema 或 migration。

## Capabilities

### New Capabilities

无。

### Modified Capabilities

- `shared-platform-primitives`: 共享 runtime config 的 `local_cache` 需要表达通用缓存实例集合，且不得在 common 中固定 user-service 业务缓存名。
- `auth-session-management`: 认证 token version 本地缓存仍为必需运行依赖，但实例名和存在性校验由 user-service 认证/provider 边界负责。
- `rbac-access-control`: RBAC 用户角色本地缓存仍为必需运行依赖，但实例名和存在性校验由 user-service 权限/provider 边界负责。

## Impact

- 受影响代码：`common/runtime/config/config.go`、`common/runtime/config/validation.go`、配置 loader/env override 测试、user-service provider/feature 中读取 `LocalCache.AuthTokenVersion` 和 `LocalCache.RBACUserRoles` 的调用点、`user-service/configs/config.yaml`、bootstrap/e2e 测试配置断言。
- 受影响规格：`shared-platform-primitives`、`auth-session-management`、`rbac-access-control`。
- 外部契约：不改变 HTTP API、OpenAPI、数据库 schema、migration、部署拓扑或 `localcache.Cache` 的运行时缓存语义。
- 验证重点：common 配置测试、user-service 相关配置/bootstrap/auth/permission 测试，以及 `make user-service-architecture-lint`。
