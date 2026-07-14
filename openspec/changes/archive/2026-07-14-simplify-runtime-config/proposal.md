## Why

`common/runtime/config.Config` 当前同时拥有基础 server 生命周期、服务资源、本地缓存、诊断入口和文件日志策略。旧契约让 `common` 随单个服务依赖膨胀，也使配置迁移无法严格拒绝已经失效的字段。

这次迁移明确不保留旧 Go 类型、字段别名、YAML 路径或 fallback。核心类型删除、资源类型迁移、user-service 接线、严格解码、部署配置和观测行为必须形成一个可统一验证的原子 change；若拆成多个独立归档的 breaking change，中间状态必然无法通过仓库级编译和验证门禁。

## What Changes

- **BREAKING** 将核心 `Config` 收敛为 `app`、`server`、`log` 和 `observability`，删除 `system`、顶层 `http`、`local_cache`、顶层 `redis`、顶层 `postgres`、pprof、trusted proxies、文件日志和 tracing exporter 旧契约。
- 新增 `common/runtime/resources`，提供可复用的 Redis/PostgreSQL 具名资源类型、默认值和校验，但不把资源声明放回核心 `Config`。
- **BREAKING** user-service 使用服务自有 `resources.redis`、`resources.postgres` 和 feature 缓存配置，并迁移 datastore、providers、HTTP server 和健康检查接线。
- 配置加载严格拒绝 unknown key，并在错误中报告具体字段路径；新环境变量使用嵌套后的配置路径。
- 日志收敛到 stdout/stderr 结构化输出，tracing 收敛到 OTLP，pprof 改为默认关闭的独立诊断入口，trusted proxy 边界不再由核心 Config 治理。
- 同步更新测试 fixture、本地配置、Compose、Kubernetes、Helm、文档、能力地图和长期规格。

## Capabilities

### New Capabilities

无。

### Modified Capabilities

- `shared-platform-primitives`: 收敛核心配置契约，新增共享资源配置类型，迁移 datastore 并启用严格解码。
- `runtime-observability`: 收敛 HTTP server、stdout/stderr logging、OTLP tracing 和独立 pprof 诊断边界。
- `auth-session-management`: 将认证资源和 token version cache 配置迁移到 user-service 自有边界。
- `rbac-access-control`: 将 permission/Casbin cache 和资源读取迁移到 user-service 自有边界。
- `delivery-operations`: 迁移配置样例、环境变量、Compose、Kubernetes、Helm 和相关交付文档。

## Impact

- Go 代码：影响 `common/runtime/config`、`common/runtime/resources`、`common/runtime/datastore`、`common/runtime/logger`、`common/runtime/observability`、`common/testing`、user-service 配置、bootstrap、providers、router、auth 和 permission/RBAC cache。
- 配置契约：旧 YAML 和环境变量路径立即失效，不提供兼容层；未知字段在启动前失败。
- 部署与观测：配置样例和部署资产迁移到新结构；日志只输出 stdout/stderr；pprof 不再挂业务端口。
- HTTP API、Ent schema、Atlas migration 和 OpenAPI 业务契约不变。
- 安全：生产环境 pprof 只允许 loopback 监听；trusted proxy 由入口基础设施或服务显式策略拥有。
