## Why

`common/runtime/logger.SetDefault` 会替换进程级兜底 logger，当前部分测试用它捕获日志或验证默认 logger 行为，容易对同一进程内的跨包测试、并行测试和日志断言造成串扰。需要把非必要的日志捕获改为 context logger 或局部 logger 注入，并把必须覆盖 `SetDefault` 的测试集中隔离。

## What Changes

- 收敛 `common/runtime/logger` 测试中对 `SetDefault` 的直接调用，只在验证进程级默认 logger 行为时使用。
- 为必须修改默认 logger 的测试提供保存和恢复 helper，确保测试结束后恢复原默认值，并明确这些测试不得并行运行。
- 将 `common/http/binding` 的日志捕获测试从修改进程级默认 logger 改为通过 request context 注入 logger。
- 保持 `SetDefault`、`FromContext`、`WithContext`、`SQL` 对外行为以及 `trace_id`、`span_id` 字段名不变。

## Capabilities

### New Capabilities

- 无。

### Modified Capabilities

- `runtime-observability`: 约束 logger 默认值测试的进程级状态影响，确保日志观测 helper 的测试不会污染其他测试。
- `shared-platform-primitives`: 约束 `common/` 共享 runtime primitive 的测试隔离方式，优先使用 context logger 或局部注入替代进程级状态替换。

## Impact

- 影响代码范围：`common/runtime/logger/context.go`、`common/runtime/logger/logger_test.go`、`common/http/binding/validation_test.go`。
- 不改变生产 API、HTTP API、OpenAPI、数据库 schema、部署资产或运行时日志字段契约。
- 验证范围包括 `common` 模块下的 `go test -race ./runtime/logger` 和 `go test -race ./http/binding`。
