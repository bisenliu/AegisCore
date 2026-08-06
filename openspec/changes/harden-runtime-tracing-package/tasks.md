## 1. API 调研与影响确认

- [x] 1.1 搜索仓库内 `(*Provider).Start`、`Provider.Start`、`.Start(ctx, cfg` 和 tracing 包外直接启动调用，确认 `Start` 是否存在包外消费者。
- [x] 1.2 根据搜索结果记录 breaking impact：若无包外调用方，将 `Start` 收窄为包内 helper；若存在调用方，迁移到 `NewProvider(ctx, Options)` 或 `NewTracingProvider(lifecycle, cfg)`。
- [x] 1.3 确认变更仅影响 `common/runtime/observability/tracing/` 与 OpenSpec artifacts，不修改 HTTP、Redis、SQL、Ent 业务接线、OpenAPI、数据库 schema 或部署资产。

## 2. Tracing 包拆分

- [x] 2.1 将 `common/runtime/observability/tracing/provider.go` 收敛为 `Options`、`Provider`、公开 constructor、`Tracer`、`OTelTracerProvider`、`TextMapPropagator` 和 `Shutdown` 等 facade 入口。
- [x] 2.2 新增或迁移 `resource.go`，只承载 OTel resource 属性构造和相关常量。
- [x] 2.3 新增或迁移 `exporter.go`，只承载 OTLP exporter factory、默认 timeout 和 `create OTLP tracing exporter` 错误包装。
- [x] 2.4 新增或迁移 `dynamic.go`，只承载 dynamic tracer provider、dynamic tracer 和启动前/启动后/关闭后委托逻辑。
- [x] 2.5 新增或迁移 `lifecycle.go`，承载包内启动状态机、重复启动检查、非法输入错误和关闭状态转换。
- [x] 2.6 保持 `fx.go` 只负责共享 runtime config 到 tracing `Options` 的映射、Fx hook 注册和 Fx constructor 校验。

## 3. 生命周期语义实现

- [x] 3.1 实现普通 constructor 立即启动语义：启用 tracing 时在传入 context 预算内创建 exporter、batch processor 和 SDK provider；失败时保留可 wrapping 的 cause。
- [x] 3.2 实现 Fx constructor 延迟启动语义：constructor 阶段不创建 exporter、不连接 OTLP endpoint、不启动 batch processor，并返回稳定非 nil facade。
- [x] 3.3 实现重复启动保护：同一 provider 已启动后再次启动返回明确错误，且保持当前 SDK provider 不变，不泄漏旧 exporter。
- [x] 3.4 实现 `Shutdown(ctx)` 幂等语义：nil provider、未启动 provider、已关闭 provider 均返回 nil；首次关闭恢复 dynamic tracer no-op。
- [x] 3.5 确保 dynamic tracer 在启动前安全 no-op、启动后使用真实 SDK provider、Shutdown 后恢复 no-op，且不安装或修改 OpenTelemetry global provider。
- [x] 3.6 确保 disabled provider 不要求 OTLP endpoint、不调用 exporter factory、不连接网络，并保持可注入、可创建 span、可传播 context。

## 4. 文档与示例

- [x] 4.1 新增 `common/runtime/observability/tracing/doc.go`，说明普通 constructor 与 Fx adapter 的所有权、动态 tracer 语义和 Shutdown 契约。
- [x] 4.2 新增 `common/runtime/observability/tracing/example_test.go`，覆盖 disabled provider、span 创建、propagator inject/extract 和显式 `Shutdown`，且不连接真实 OTLP endpoint。
- [x] 4.3 如果 `Start` 导出 API 被移除或收窄，在包文档或变更说明中明确迁移到 `NewProvider` 或 `NewTracingProvider`。

## 5. 测试覆盖

- [x] 5.1 更新 tracing 单元测试，覆盖重复启动错误、重复 Shutdown 幂等、nil/非法启动输入、启动失败不替换既有 provider。
- [x] 5.2 更新 Fx lifecycle 测试，覆盖 exporter 只在 `OnStart` 创建、启动 context 被传递、后续 hook 失败触发 rollback 并恢复 no-op。
- [x] 5.3 更新 dynamic tracer 测试，覆盖 constructor 阶段保存的 tracer 在启动前 no-op、启动后真实 span、Shutdown 后 no-op。
- [x] 5.4 更新 propagator 测试，覆盖 W3C trace context 与 baggage inject/extract，且行为不依赖真实 exporter。

## 6. 验证与交付

- [x] 6.1 在 `common/` module 运行 `go test ./runtime/observability/tracing`。
- [x] 6.2 在 `common/` module 运行 `go test -race ./runtime/observability/tracing`。
- [x] 6.3 在 `common/` module 运行 `go vet ./runtime/observability/tracing`。
- [x] 6.4 在 `common/` module 运行 `go test ./runtime/observability/tracing -run Example` 验证 executable examples。
- [x] 6.5 运行 `make common-test`。
- [x] 6.6 运行 `make user-service-architecture-lint` 验证 OpenSpec 与架构边界。
- [x] 6.7 将本次预期代码、测试、文档和 OpenSpec artifact 变更加到暂存区。
- [x] 6.8 运行 `make lint`。
- [x] 6.9 运行 `make verify`。
- [x] 6.10 检查 `git diff --cached` 与必要的未暂存 diff，确认未包含无关生成物、业务 API、数据库 migration 或部署资产变更。
