## 1. Redis metrics context 传播

- [x] 1.1 在 `common/runtime/observability/metrics/redis.go` 为 `RedisPingCollector` 增加 context-aware 收集路径，使 Redis PING 从 scrape request context 派生 timeout，并保留 `Collect(ch)` 兼容入口。
- [x] 1.2 在 `common/runtime/observability/metrics` 中实现业务中立的 context-aware gather/serve 桥接能力，确保普通 Prometheus collectors 继续按现有 registry 行为导出。
- [x] 1.3 更新 `user-service/internal/router/metrics.go` 使用 HTTP request context 暴露 metrics，并保持 metrics 路由路径、content type 协商和输出格式兼容。
- [x] 1.4 补充 Redis collector 和 metrics route 测试，覆盖 scrape request 取消传播到 Redis PING、collector timeout 保留、最小探测间隔复用快照和 metric family/label 不漂移。

## 2. Casbin 初始加载 context 传播

- [x] 2.1 调整 `user-service/internal/features/permission/infrastructure/casbin/enforcer.go`，使 `NewEngine` 只构造 Engine，不在 provider 构造阶段执行初始 `Reload(context.Background())`。
- [x] 2.2 增加 Fx lifecycle 注册函数，在 `OnStart(ctx)` 中调用 `Engine.Reload(ctx)` 完成初始 policy 加载，并在失败时保留 fail-closed、`LastError()` 和 reload metrics 语义。
- [x] 2.3 更新 `user-service/internal/features/permission/fx.go` 注册 Casbin 初始加载 lifecycle hook，并保持 authorization engine 与 policy reload engine provider 边界不变。
- [x] 2.4 更新 Casbin enforcer 测试，覆盖 `OnStart(ctx)` context 传播、启动 context 取消、初始加载失败 fail-closed、手动 `Reload(ctx)` 继续使用调用方 context。

## 3. 回归验证

- [x] 3.1 运行 `go test ./runtime/observability/metrics/...` 于 `common/`，确认 Redis metrics collector 与 provider 测试通过。
- [x] 3.2 运行 `go test ./internal/router ./internal/features/permission/...` 于 `user-service/`，确认 metrics route 与 RBAC/Casbin 测试通过。
- [x] 3.3 运行 `make user-service-architecture-lint`，确认 common 与 user-service 边界未被破坏。
- [x] 3.4 在 OpenSpec 实现、规格和文档任务全部完成后，先暂存本次预期变更，再运行 `make lint` 和 `make verify`，确认全仓 lint、测试、生成物 drift 检查通过。
- [x] 3.5 检查 `git diff --exit-code -- user-service/docs/openapi.json user-service/docs/openapi.yaml user-service/docs/openapi.go user-service/ent user-service/migrations`，确认本变更未产生 OpenAPI、Ent 或 migration 生成物漂移。
