## Context

`bound-localcache-runtime` 已将 auth token version 和 RBAC user roles 热路径迁移到有界 `common/runtime/localcache`，并在 `common/runtime/config` 中引入 `LocalCacheConfig`。当前配置模型仍把 `auth_token_version` 和 `rbac_user_roles` 作为 Go 结构体固定字段暴露在 common 中，这让共享 runtime config 层承担了 user-service 的业务缓存名和必需实例假设。

本 change 要保持现有 YAML 结构、默认值和缓存运行时语义不变，只调整配置 ownership：common 负责通用 `local_cache` map 的解析和单个 entry 校验，user-service 负责声明自身需要的缓存名、读取实例配置并在 provider/feature 装配时校验缺失。

## Goals / Non-Goals

**Goals:**

- 将 `common/runtime/config.LocalCacheConfig` 调整为通用实例集合，支持按配置 key 表达任意本地缓存实例。
- 从 common 中移除 `AuthTokenVersion`、`RBACUserRoles` 两个 user-service 业务字段和专用校验路径。
- 让 validation 对 `local_cache` map 中所有 entry 做统一校验，并在错误路径中保留具体配置 key。
- 让 user-service 在 auth 和 permission/RBAC provider 边界通过本服务常量读取 `auth_token_version`、`rbac_user_roles`，缺失时返回明确装配错误。
- 更新 loader、env override、validation、bootstrap/e2e 和相关 feature/provider 测试，证明旧 YAML 结构仍可用。

**Non-Goals:**

- 不改变 `common/runtime/localcache.Cache` API、TTL、容量、loader、singleflight、clone、stats 或 close 行为。
- 不改变 `auth_token_version` 和 `rbac_user_roles` 的默认容量、TTL、load timeout、num counters 或 buffer items。
- 不新增动态缓存发现、自动注册、远程配置中心或缓存实例生命周期编排机制。
- 不改变外部 HTTP API、OpenAPI、Ent schema、Atlas migration、deployment manifest 或 observability 指标契约。

## Decisions

1. `LocalCacheConfig` 使用 map 语义，而不是继续在 common 中扩展固定字段。

   - common 的职责是跨服务 runtime primitive，不能携带 user-service 的 auth/RBAC 缓存名。
   - map 允许后续服务或 feature 增加本地缓存配置，而不需要修改 common 的 Go struct。
   - 备选方案：保留固定字段并增加注释说明它们来自 user-service。该方案不能移除业务语义泄漏，也会让每个新缓存继续修改 common。

2. user-service 通过常量读取必需缓存实例，并在 provider/feature 层 fail fast。

   - auth token version 缓存名保留在 auth feature/provider 边界，RBAC user roles 缓存名保留在 permission/casbin 边界。
   - 缺失 `local_cache.auth_token_version` 或 `local_cache.rbac_user_roles` 时，创建对应 cache 的 provider MUST 返回带缓存名的明确错误。
   - 备选方案：在 common validation 中继续检查这两个名字必需存在。该方案让 common 继续知道 user-service 业务缓存实例，违背本次目标。

3. env override 继续支持现有配置路径，同时实现对 map key 的通用覆盖。

   - `local_cache.auth_token_version.capacity` 这类 YAML 路径不变，测试继续覆盖该结构。
   - 环境变量覆盖需要能落到 map entry，而不是依赖 Go 字段名；具体实现遵循当前 loader/env override 的命名规则。
   - 备选方案：借此迁移配置 key 或 env 名。该方案会扩大兼容面风险，而本 change 不需要外部配置结构变更。

4. validation 遍历所有 local_cache entry，并对空 key 和 entry 字段做通用错误聚合。

   - 每个 entry 统一校验 `capacity > 0`、`ttl > 0`、`load_timeout > 0`、`num_counters >= 0`、`buffer_items >= 0`。
   - 错误路径使用 `local_cache.<name>.<field>`，空 key 使用 `local_cache` 级别错误。
   - 备选方案：只校验 user-service 已知两个 entry。该方案无法证明 common 已成为通用配置 primitive。

## Risks / Trade-offs

- [Risk] map 解码或 env override 对嵌套 key 的行为可能与结构体字段不同 → Mitigation: 用 loader 和 env override 测试覆盖 `auth_token_version`、`rbac_user_roles` 以及额外自定义 entry。
- [Risk] 必需缓存从 common validation 下沉到 provider 层后，错误暴露时间从配置校验变为服务装配 → Mitigation: auth/RBAC provider 单元测试和 bootstrap 配置测试覆盖缺失实例的明确错误。
- [Risk] map 遍历顺序不稳定导致 validation 错误顺序抖动 → Mitigation: validation 使用排序后的 cache names 生成错误。
- [Risk] 未来缓存 key 与 env 命名约定冲突 → Mitigation: 保持 kebab/snake 风格配置 key，并在 user-service 常量中集中定义必需缓存名。

## Migration Plan

1. 更新 OpenSpec delta，明确 common 通用 local_cache map 和 user-service 必需缓存 ownership。
2. 修改 `common/runtime/config/config.go`，将 `LocalCacheConfig` 改为 map 类型或等价通用实例集合，并保留 `LocalCacheInstanceConfig`。
3. 修改 `common/runtime/config/validation.go`，按排序 key 遍历所有 entry 做通用校验，移除 auth/RBAC 专用字段引用。
4. 更新 common loader、env override、validation 测试，覆盖原有两个 entry 和额外通用 entry。
5. 更新 user-service auth 与 permission/RBAC provider，使用本地常量读取 map entry；缺失时返回明确错误。
6. 更新 `user-service/configs/config.yaml`、bootstrap/e2e 测试配置断言和相关 provider/feature 测试。
7. 运行 common 配置测试、user-service 相关配置/bootstrap/auth/permission 测试和 `make user-service-architecture-lint`。

回滚策略：本 change 不涉及数据库、OpenAPI 或外部 API。若 map 解码或 provider 装配出现问题，可随代码回滚到固定字段模型；配置文件结构和默认值未改变，因此无需数据迁移或运维配置迁移。

## Open Questions

无。当前需求已明确不改变运行时缓存行为、不改变默认配置值，也不引入动态发现机制。
