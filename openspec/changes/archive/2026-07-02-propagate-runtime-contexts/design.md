## Context

当前代码中有两类有 IO 的运行时路径未传播所属生命周期 context。

Redis runtime metrics collector 在 `common/runtime/observability/metrics/redis.go` 的 `Collect -> snapshot -> Ping` 路径中使用 `context.WithTimeout(context.Background(), ...)`。该 collector 由 user-service 在 `user-service/internal/providers/metrics.go` 注册，并通过 `user-service/internal/router/metrics.go` 的 `promhttp.HandlerFor(gatherer, ...)` 暴露。`prometheus.Collector.Collect` 与 `prometheus.Gatherer.Gather` 没有 `context.Context` 参数，因此不能只替换一行来获得 scrape request context，需要在 common metrics provider 和 user-service metrics route 之间建立可传递 request context 的薄封装。

Casbin Engine 在 `user-service/internal/features/permission/infrastructure/casbin/enforcer.go` 的 `NewEngine` 构造期间同步调用 `Reload(context.Background())`。`Reload` 已经接收 context，`Loader.LoadPolicies(ctx)` 最终执行 Ent 查询，因此问题集中在初始加载发生在 Fx provider 构造阶段且没有使用 Fx 启动 lifecycle context。现有语义要求初始加载失败后服务继续装配，Engine fail-closed，并通过 `LastError()` 与 reload metrics 暴露失败。

本变更横跨 `common` 与 `user-service`，但不改变数据库 schema、HTTP API、OpenAPI、部署清单或外部依赖。

## Goals / Non-Goals

**Goals:**

- Redis PING metrics 探测在 HTTP scrape request 取消时能够取消底层 Redis PING，并继续受 collector timeout 约束。
- Redis metrics 的 metric family、label key、label value、计数语义和 metrics endpoint 路径保持稳定。
- Casbin 初始 policy reload 使用 Fx `OnStart(ctx)` 传入的启动 context。
- Casbin 初始加载失败继续保持 fail-closed，不因本次变更改为启动失败。
- 用测试覆盖 context 传播、timeout 保留和 fail-closed 保留。

**Non-Goals:**

- 不把 Redis 探测改为后台定时任务，不改变当前 scrape 触发探测并缓存最小间隔内快照的模型。
- 不新增 Prometheus 指标、label、配置项或外部依赖。
- 不改变 Casbin policy loader 的权威数据来源、policy sync 机制、授权热路径或超级管理员通配授权。
- 不改动 Ent schema、Atlas migration、OpenAPI 生成物、Docker、Compose、Kubernetes、Helm 或 Grafana/Prometheus 资产。

## Decisions

### Decision: 为 Redis collector 增加 context-aware 收集路径

在 `common/runtime/observability/metrics` 内保留现有 `prometheus.Collector` 兼容实现，同时为需要 request context 的 collector 增加受控的 context-aware 收集能力。`RedisPingCollector.Collect(ch)` 继续作为兼容入口使用 `context.Background()`，新增内部或导出的 `CollectWithContext(ctx, ch)` 路径，并让 `snapshot` 从传入 context 派生 timeout。

选择该方案是因为上游 `prometheus.Collector` 接口没有 context 参数，直接修改 `Collect` 签名不可行；保留兼容入口可以维持现有 registry 注册和测试工具使用方式。

备选方案：把 Redis PING 移到后台 probe goroutine。该方案能使用服务 root context，但会把指标语义从 scrape 触发改为后台采样，改变观测时效性和故障表现，本次不采用。

### Decision: 在 metrics provider/route 提供 request context 到 gather 的薄桥接

在 common metrics provider 中提供能够使用 `context.Context` 执行 gather/render 的薄封装，或提供 context-aware gatherer 与 HTTP handler helper；user-service metrics route 使用 Gin request context 调用该封装。普通 Prometheus collectors 仍走现有 `Gatherer()`，只有支持 context-aware 的 Redis collector 在 request context 可用时消费该 context。

选择该方案是因为 user-service 当前 metrics endpoint 是 request context 的拥有者，而 common metrics provider 是 collectors 的注册中心；桥接逻辑应保持业务中立，不能在 common 中引入 user-service 路由、Gin 业务语义或 RBAC 语义。

备选方案：直接在 user-service route 中特殊调用 Redis collector。该方案会破坏统一 metrics provider 边界，并让服务路由了解具体 Redis collector，不采用。

### Decision: Casbin 初始 reload 从构造器迁移到 Fx lifecycle

`NewEngine` 只构造 `Engine` 并注入依赖，不执行初始 IO。新增 Fx invoke 或注册函数把初始 `engine.Reload(ctx)` 放入 `OnStart`，使用 Fx 提供的启动 context。`Reload` 失败时不返回错误阻止启动，保留现有 fail-closed 语义和 metrics/LastError 记录。

选择该方案是因为 Fx lifecycle 明确提供启动 context，且能避免 provider 构造阶段发生不可取消 IO。相比让 `NewEngine` 接收 context，Fx provider 构造器没有天然的启动 context 参数，生命周期 hook 更符合框架模型。

备选方案：初始 reload 失败时返回 `OnStart` 错误并阻止服务启动。该方案会改变现有可用性语义和运维行为，本次不采用。

## Risks / Trade-offs

- [Risk] 自定义 context-aware metrics bridge 可能绕开 `promhttp.HandlerFor` 的部分编码、错误处理或 content negotiation 行为。→ Mitigation：优先复用 Prometheus `expfmt` 和现有 `promhttp` 行为能复用的部分；通过 HTTP route 测试验证 `/metrics` 输出和错误行为不漂移。
- [Risk] `RedisPingCollector.Collect` 兼容入口仍会使用 `context.Background()`。→ Mitigation：生产 `/metrics` 路由必须走 context-aware 封装；保留兼容入口仅用于标准 registry 或测试工具，且仍受 collector timeout 约束。
- [Risk] Casbin 初始 reload 移到 `OnStart` 后，部分测试或调用方可能假设 `NewEngine` 返回时已完成加载。→ Mitigation：调整测试显式调用 `Reload(ctx)` 或执行 lifecycle；user-service 装配路径通过 `fx.Invoke` 注册初始加载 hook。
- [Risk] 初始 reload 失败继续允许服务启动，可能被误解为健康。→ Mitigation：不改变现有 fail-closed 与 health/startup 检查语义；验证 `LastError()`、reload metrics 和授权拒绝行为保留。

## Migration Plan

1. 在 common metrics 中添加 context-aware Redis 收集路径和 provider/handler 桥接能力。
2. 更新 user-service metrics route 使用 request context 暴露 metrics，保持路径和输出格式兼容。
3. 将 Casbin Engine 初始 reload 迁移到 Fx lifecycle，并更新 permission module 注册。
4. 更新或新增测试覆盖 Redis scrape cancellation、Redis timeout、Casbin `OnStart(ctx)` 传播和初始加载失败 fail-closed。
5. 回滚时可恢复为 `promhttp.HandlerFor` 和构造器内初始 reload；由于无 schema、API 或配置变化，回滚不需要数据迁移。

## Open Questions

- 无。
