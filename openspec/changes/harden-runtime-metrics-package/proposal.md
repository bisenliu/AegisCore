## Why

`common/runtime/observability/metrics` 已承载 provider、registry、context-aware gather、runtime collector 与多类 adapter，文件职责边界不够清晰，审查时难以确认启停、注册、scrape context 与低基数约束是否被稳定保持。现在需要在不改变导出 API、指标名称或采集语义的前提下整理 package 结构，并用规格、包文档和可执行示例固化共享 metrics provider 的使用契约。

## What Changes

- 调整 `common/runtime/observability/metrics` 的同 package 文件组织，将 Provider/registry、`ContextCollector`/gather wrapper、runtime collector 与 SQL、Redis、scheduler、workerpool、localcache、component status collector adapter 分离到职责清晰的文件。
- 保持现有 package、import path、导出 API、指标名称、label、bucket 和采集语义兼容，不新增业务指标，不安装 Prometheus global registry。
- 新增 `doc.go`，说明 enabled/disabled provider、独立 registry、重复注册、`HTTPHandler`、`GatherContext`、collector context 与 label cardinality 约束。
- 新增 `example_test.go`，使用本地 registry 演示 enabled/disabled provider、自定义 collector 注册、gather 和 `HTTPHandler`，不访问公网或真实 datastore。
- 更新 `runtime-observability` 与 `shared-platform-primitives` 的 OpenSpec delta，记录 metrics provider 的稳定使用契约和 common 业务中立边界。

## Capabilities

### New Capabilities

无。

### Modified Capabilities

- `runtime-observability`: 补充 metrics provider 的 enabled/disabled、独立 registry、context-aware scrape/gather、低基数 label 与 HTTP handler 使用契约。
- `shared-platform-primitives`: 补充 `common/runtime/observability/metrics` 作为业务中立共享 primitive 的文件职责、导出 API 兼容、示例测试和禁止引入业务语义的契约。

## Impact

- 影响代码：`common/runtime/observability/metrics` 包内文件组织、包文档、示例测试和相关普通测试。
- 影响规格：新增 `openspec/changes/harden-runtime-metrics-package/specs/runtime-observability/spec.md` 与 `openspec/changes/harden-runtime-metrics-package/specs/shared-platform-primitives/spec.md` delta。
- API 兼容性：保持原 package、调用方 import path 与导出 API；不改变 Prometheus 指标名称、label、bucket、collector 注册语义或采集语义。
- 依赖与系统：不新增外部依赖，不连接公网，不访问真实 datastore，不使用 Prometheus global registry，不引入 user-service feature DTO 或业务状态。
- 验证：需要通过 metrics 包普通测试、race 测试、go vet、示例测试、`make common-test`、`make lint` 和 `make verify`。
