## Why

当前 Redis metrics collector 在 scrape 触发的 Redis PING 中使用独立的 `context.Background()`，导致 `/metrics` 请求取消时探测不会随 scrape 生命周期及时结束；Casbin Engine 初始 policy 加载也在 Fx provider 构造期间使用 `context.Background()`，导致服务启动取消或超时无法传播到数据库查询。

本变更用于收敛运行时依赖探测和 RBAC 初始授权策略加载的 context 生命周期，使有 IO 的操作能受所属 scrape 或 Fx 启动上下文约束，同时保持现有指标、授权和 fail-closed 语义稳定。

## What Changes

- Redis PING metrics 探测需要具备 context-aware 执行路径，使 HTTP scrape request context 取消时能够传播到 Redis PING，同时继续保留 collector 自身 timeout 和最小探测间隔。
- metrics endpoint 需要使用能够把 Gin/HTTP request context 传递到 Redis 探测路径的 gather/serve 封装；既有 Prometheus metric family、label key、label value 和数值语义保持不变。
- Casbin Engine 构造器不再在 provider 构造阶段用 `context.Background()` 执行初始 policy reload；初始加载迁移到 Fx lifecycle `OnStart(ctx)`，使用 Fx 提供的启动 context。
- Casbin 初始加载失败仍保持当前 fail-closed 行为：服务可继续装配，Engine 记录 `LastError()` 和 reload metrics，授权请求在没有可用 enforcer 时拒绝访问。
- 补充单元测试或集成测试，覆盖 Redis scrape 取消传播、Redis timeout 保留、Casbin 初始加载使用启动 context、初始加载失败继续 fail-closed。
- 不引入数据库 schema、HTTP API、OpenAPI、部署资产或对外配置项变更。

## Capabilities

### New Capabilities

- 无。

### Modified Capabilities

- `runtime-observability`: 明确 Redis runtime metrics 探测必须受 scrape request context 和自身 timeout 约束，且不改变既有指标契约。
- `rbac-access-control`: 明确 Casbin 初始 policy 加载必须使用服务启动 lifecycle context，并在初始加载失败时保持 fail-closed 语义。

## Impact

- 影响代码：`common/runtime/observability/metrics/redis.go`、`common/runtime/observability/metrics/provider.go`、`user-service/internal/router/metrics.go`、`user-service/internal/features/permission/infrastructure/casbin/enforcer.go`、`user-service/internal/features/permission/fx.go` 及相关测试。
- API 影响：无 HTTP API、OpenAPI、CLI 参数或响应契约变化。
- 数据库影响：无 Ent schema 或 Atlas migration 变化。
- 观测影响：Redis metrics 的 metric family 和 labels 保持稳定；scrape 取消时 Redis PING 可更早终止，避免无意义后台 IO。
- 安全与授权影响：授权决策语义保持不变；初始 policy 加载失败仍 fail-closed，不放行未授权请求。
- 依赖影响：不新增外部依赖。
