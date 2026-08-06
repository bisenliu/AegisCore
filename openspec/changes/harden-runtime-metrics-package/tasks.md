## 1. 规格与现状确认

- [x] 1.1 阅读 `proposal.md`、`design.md`、`specs/runtime-observability/spec.md` 和 `specs/shared-platform-primitives/spec.md`，确认本次只整理 `common/runtime/observability/metrics` 且不改变导出 API、指标名称、label、bucket 或采集语义。
- [x] 1.2 梳理 `common/runtime/observability/metrics` 现有文件、导出 symbol、调用方和测试覆盖，记录 Provider/registry、context-aware gather、runtime collector、SQL、Redis、scheduler、workerpool、localcache、component status adapter 的目标文件职责。

## 2. metrics package 文件整理

- [x] 2.1 将 Provider、enabled/disabled、独立 registry、registerer/gatherer、重复注册和 nil collector 错误逻辑保留在 provider/registry 职责文件中，保持 `NewProvider`、`Enabled`、`Registerer`、`Gatherer`、`Register` 和 `MustRegister` 行为兼容。
- [x] 2.2 将 `ContextCollector`、context collector wrapper、`GatherContext`、`HTTPHandler` 和当前 gather context 管理整理到独立职责文件，保持 HTTP scrape 使用 request context、普通 gather 使用 background context 的语义。
- [x] 2.3 确认 `runtime.go`、`sql.go`、`redis.go`、`scheduler.go`、`workerpool.go`、`localcache.go` 和 `status.go` 各自只承载对应 runtime collector 或 adapter 职责，不引入 user-service feature DTO、业务状态或服务私有配置。
- [x] 2.4 运行格式化，确保拆分后的 Go 文件 package、import、注释和导出 symbol 文档符合现有风格。

## 3. 文档与示例

- [x] 3.1 新增 `common/runtime/observability/metrics/doc.go`，说明 enabled/disabled provider、独立 registry、重复注册、`HTTPHandler`、`GatherContext`、collector context 和低基数 label 约束。
- [x] 3.2 新增 `common/runtime/observability/metrics/example_test.go`，使用本地 registry、自定义内存 collector 和 `httptest` 展示 enabled provider、自定义 collector 注册、gather 和 `HTTPHandler`。
- [x] 3.3 在 `example_test.go` 中覆盖 disabled provider 的 no-op 行为，并确保示例不访问公网、PostgreSQL、Redis、scheduler、workerpool 或真实 localcache datastore。
- [x] 3.4 确认 `go doc` 可从 package 文档导航到主要 executable examples，且示例文字不承诺普通 `Collect` 感知 HTTP cancellation。

## 4. 测试与验证

- [x] 4.1 运行 `go test ./common/runtime/observability/metrics`，确认普通测试和示例测试通过。
- [x] 4.2 运行 `go test -race ./common/runtime/observability/metrics`，确认 context-aware gather 与文件整理没有引入竞态。
- [x] 4.3 运行 `go vet ./common/runtime/observability/metrics`，确认包文档、示例和拆分后的 Go 代码通过 vet。
- [x] 4.4 运行 `make common-test`，确认共享模块测试通过。
- [x] 4.5 将本次预期代码、文档和 OpenSpec artifact 变更加到暂存区，再运行 `make lint`。
- [x] 4.6 在本次预期变更已暂存的状态下运行 `make verify`，确认完整门禁通过且无生成物 drift。

## 5. 收尾检查

- [x] 5.1 检查 `git diff --cached`，确认只包含本 change 预期的 OpenSpec、metrics package 文档、示例、文件整理和测试相关变更。
- [x] 5.2 将已完成任务逐项改为 `- [x]`，并确认 `openspec status --change "harden-runtime-metrics-package"` 显示 apply-ready。
