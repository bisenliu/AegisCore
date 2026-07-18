## Why

当前部分 Fx constructor 在构造阶段创建 tracing batch processor、worker pool、Redis client、Ent wrapper 等运行资源，并仅通过 `fx.StopHook` 注册清理逻辑。若后续 constructor 或 Invoke 失败导致 App 未成功启动，这些 stop hook 不能被视为 constructor rollback，可能遗留后台 goroutine、网络连接或缓存清理资源。

## What Changes

- 强化 Fx 资源生命周期约束：优先让 constructor 只创建无后台副作用的 holder 或轻量对象，把真正运行资源延迟到 `OnStart` 创建，并在 `OnStop` 关闭。
- 对必须保留在 constructor 中创建的资源，要求在部分构造失败时立即清理已创建部分，并降低后续 wiring 出现可预期错误的概率。
- 优先处理高风险资源，包括 tracing batch processor、worker pool、带内部清理行为的 local cache、Redis client 和 Ent wrapper。
- 不迁移所有对象；例如 `sql.Open` 这类通常只创建轻量 pool handle 的构造可保留现状。

## Capabilities

### New Capabilities

- 无。

### Modified Capabilities

- `runtime-observability`: 约束 tracing provider 等观测运行资源的 Fx 生命周期创建与关闭语义。
- `shared-platform-primitives`: 约束跨服务 runtime primitive 中 Redis、workerpool、localcache 等共享资源的 Fx 生命周期安全语义。
- `auth-session-management`: 约束认证会话相关 worker pool、local cache 等主动后台资源在 user-service auth 接线中的启动与停止语义。
- `rbac-access-control`: 约束权限/RBAC policy sync 等 Redis watcher 或后台资源在 user-service permission 接线中的启动与停止语义。

## Impact

- 影响代码：`common/runtime/observability/tracing/fx.go`、`common/runtime/datastore/redis_fx.go`、`user-service/internal/features/auth/fx.go`、`user-service/internal/features/permission/fx.go`、`user-service/internal/providers/ent.go` 及相关测试。
- API 影响：不改变 HTTP API、OpenAPI 契约、配置项名称或响应结构。
- 数据库影响：不改变 Ent schema 或 Atlas migration。
- 运维影响：降低启动失败路径下后台资源、连接和 goroutine 泄漏风险；不改变部署拓扑。
- 验证影响：需要补充或调整 Fx 启动失败/停止路径测试，并运行相关 Go 测试与架构 lint。
