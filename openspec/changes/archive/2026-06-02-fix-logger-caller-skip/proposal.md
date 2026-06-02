## Why

当前业务代码通过 `common/logger` context API 输出日志时，`caller` 字段会记录到 `common/logger/context.go` 的封装函数，而不是实际业务调用点，例如 `user-services/internal/service/user_service.go`。这会降低排障效率，使日志无法直接定位触发日志的业务代码行。

## What Changes

- 调整共享 Zap logger 的 caller 记录行为，使通过 `common/logger.Info`、`Debug`、`Warn`、`Error` 等 context API 输出的日志记录实际调用方文件和行号。
- 保留现有 `trace-id` 注入、日志格式、分类文件输出和 Error 日志不默认输出 stacktrace 的行为。
- 补充测试覆盖封装 logger 场景，验证 caller 不再停留在 `common/logger/context.go` 包装层。
- 不引入新的日志 API，不改变业务代码现有调用方式。

## Capabilities

### New Capabilities

- 无。

### Modified Capabilities

- `shared-infrastructure`: 明确共享 logger context API 输出日志时，`caller` 必须指向业务调用点而不是 logger 包装层。

## Impact

- 影响代码：`common/logger/` 中 Zap logger 初始化、context API 或默认 logger 设置逻辑，以及相关测试。
- 影响外部可观察行为：日志 `caller` 字段会从封装层位置变为实际业务调用位置。
- 兼容性：不改变 HTTP API、错误码、配置字段、日志字段名、trace-id 字段名或数据模型。
- 依赖：继续使用现有 Zap caller 机制，必要时通过 `zap.AddCallerSkip` 调整调用栈跳过层数。
