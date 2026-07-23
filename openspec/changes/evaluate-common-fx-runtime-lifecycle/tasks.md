## 1. common runtime API 调整

- [x] 1.1 梳理 `common/runtime/observability/metrics/fx.go` 和 `common/runtime/observability/tracing/fx.go` 的 `NewFxProvider` 当前签名、返回值和所有仓库内调用点。
- [x] 1.2 将 metrics provider 公开入口重命名为能表达 metrics 能力的名称，并同步更新直接调用点和相关测试，不新增无真实外部消费需求的兼容别名。
- [x] 1.3 将 tracing provider 公开入口重命名为能表达 tracing 能力的名称，并同步更新直接调用点和相关测试，保持 constructor 阶段 no-op 和 lifecycle 启停语义不变。
- [x] 1.4 评估并调整 `common/runtime/timezone/fx.go`：如果仅包装 `Init`，移除或停止由 common 暴露该 Fx provider，并保留 `common/runtime/timezone.Init` 作为 framework-neutral primitive。

## 2. user-service composition root 绑定

- [x] 2.1 更新 `user-service/internal/providers/fx.go` 中 runtime/observability module 的 provider 调用，使 metrics、tracing 和服务资源 provider 的职责从命名和装配顺序上可读。
- [x] 2.2 更新 `user-service/internal/bootstrap/app.go` 中 Fx option/module 组合，使 process runtime 初始化在 runtime server 启动前显式执行。
- [x] 2.3 确认 `permissionfeature.LifecycleModule`、`registerRuntimeServers` 和 runtime observability provider 的相对顺序仍满足启动、停止和 rollback 语义。

## 3. 测试与边界验证

- [x] 3.1 更新或新增最小测试，覆盖 metrics/tracing provider 新命名后的 Fx 构图，以及 tracing 启用失败、禁用 no-op 或 rollback 既有语义不回退。
- [x] 3.2 更新或新增最小测试，覆盖 timezone 初始化由 user-service composition root 显式绑定且在 runtime server 启动前发生。
- [x] 3.3 运行相关 package 测试，例如 `go test ./common/runtime/observability/... ./common/runtime/timezone/... ./user-service/internal/bootstrap/... ./user-service/internal/providers/...`，并修复失败。
- [x] 3.4 运行 `make user-service-architecture-lint`，确认 common、user-service 和 feature 边界未被扩大。

## 4. 最终验证

- [x] 4.1 确认本次变更未修改 HTTP API、OpenAPI 生成物、Ent schema、Atlas migration、Prometheus/Grafana 资产或部署清单；如发现生成物 drift，说明原因并按对应流程处理。
- [x] 4.2 将本次预期代码、OpenSpec artifact 和文档变更加到暂存区。
- [x] 4.3 运行 `make lint` 并修复失败；未运行或失败时不得标记完成。
- [x] 4.4 运行 `make verify` 并修复失败；未运行或失败时不得标记完成。
