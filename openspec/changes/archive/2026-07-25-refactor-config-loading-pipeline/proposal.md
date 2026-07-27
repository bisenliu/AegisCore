## Why

`common/runtime/config` 当前同时拥有核心 schema、默认值、校验、Nacos 环境解析与 client、YAML 合并、严格解码、effective settings 编码、脱敏和摘要计算。服务默认值又分散在 Viper core defaults、`ConfigDefaults()` 解码前 defaults 和 `ApplyDefaults()` 解码后 defaults 三条路径，并由 `DecodeStrict[T]` 通过未导出接口自动发现。这使配置来源、加载顺序和 user-service 扩展行为相互耦合，新增配置来源或服务配置段时难以从加载入口判断完整行为。

auth token-version cache 与 RBAC user-role cache 还重复完成 feature cache 到 `localcache.Config` 的转换，并直接依赖完整 `*serviceconfig.Config`。需要在不改变现有 YAML 字段、安全语义和部署来源顺序的前提下，用显式加载管线、集中默认值和窄 settings 收敛这些职责。

## What Changes

- **BREAKING** 将 feature cache 配置改为非指针字段，并以 user-service 完整默认配置表达“默认启用”；禁用时容量、TTL 和回源超时可以保留默认值，但运行时必须忽略。
- **BREAKING** 将 `DecodeStrict[T]` 的隐式 `ConfigDefaults()`、`ApplyDefaults()` hook 改为显式 decode options，调用方明确提供 defaults、normalize 和 validate。
- **BREAKING** 将 Nacos 环境、client 和文档读取迁入 `common/runtime/config/nacos`，由 user-service 显式创建 `DocumentSource` 并调用通用 source loading 入口。
- 新增 `user-service/internal/config.DefaultConfig()`，集中 common、resources、Ent、auth 和 RBAC 默认值；保留现有 YAML 字段名与校验路径。
- 为 feature cache 集中提供默认值、校验和转换为 `localcache.Config` 的能力，删除 auth 与 permission 的重复映射。
- 为 auth、RBAC、Ent 和 resources 提供窄 settings，feature/provider 不再依赖完整根配置。
- 保持 Nacos 缺省 dataId 顺序、YAML deep merge、unknown key 拒绝、raw settings digest、effective settings 渲染与敏感值脱敏行为，并先用现有测试锁定这些行为。

## Capabilities

### New Capabilities

无。

### Modified Capabilities

- `shared-platform-primitives`: 将 Nacos 降级为文档来源 adapter，新增显式 source、merge、raw digest、decode、defaults、normalize、validate 和 render 管线契约。
- `auth-session-management`: 集中 token-version feature cache 默认值与 `localcache.Config` 映射，并让 auth 只消费窄 settings。
- `rbac-access-control`: 集中 user-role feature cache 默认值与 `localcache.Config` 映射，并让 permission/RBAC 只消费窄 settings。

## Impact

- 代码影响：`common/runtime/config/`、新增的 `common/runtime/config/nacos/`、`user-service/internal/config/`、auth 与 permission/RBAC 的 Fx/provider 和 localcache 构造路径。
- 配置影响：Go 配置类型和 loader API 发生 breaking change；`auth.token_version_cache`、`rbac.user_role_cache`、`resources.redis.cache_redis`、`resources.postgres.primary_db` 等 YAML 字段名保持不变，不提供旧 Go API 兼容层。
- 部署影响：仍从 Nacos 加载，缺省 dataId 顺序保持 `base.yaml`、`resources.yaml`、`<service>.yaml`；Compose、Kubernetes 和 Helm 配置结构不应变化。
- 安全影响：unknown key 继续在启动前失败，raw digest 不包含默认值，effective render 继续脱敏；禁用缓存只能改变性能，不能放宽 token version 校验或 RBAC fail-closed 语义。
- API、数据库和观测影响：不改变 HTTP API、OpenAPI 生成物、Ent schema、Atlas migration、日志 message、metrics 名称或 dashboard。
- 文档影响：更新 `docs/DEVELOPMENT.md` 的配置加载说明，并在实现完成后把 delta 合并到三份主规格。
