## Why

`common/runtime/logger` 的按日分割 lumberjack writer 当前把 Zap `Sync()` 实现为关闭底层 writer。Zap 调用方通常将 `Sync()` 视为 flush 语义，如果运行期同步日志后继续写入，可能导致后续日志写入失败或静默丢失。

## What Changes

- 调整 `dailyLumberjackWriteSyncer.Sync()`，避免将同步语义实现为关闭当前 `lumberjack.Logger`。
- 保留按日期轮转时关闭旧 writer 的行为，确保文件句柄在真实轮转场景中释放。
- 增加或更新测试，覆盖运行期调用 `Sync()` 后继续写入日志仍然成功的行为。
- 不改变日志文件命名、轮转日期格式、lumberjack 配置项或 Zap logger 对外创建方式。

## Capabilities

### New Capabilities

- 无。

### Modified Capabilities

- `shared-infrastructure`: 明确共享 Zap 日志 writer 的 `Sync()` 不应关闭仍在使用的按日分割 writer，运行期同步后应允许继续写入。

## Impact

- 影响代码：`common/runtime/logger/daily_writer.go` 及其相关测试。
- 影响 capability：`shared-infrastructure`。
- API 兼容性：不改变 HTTP API、响应信封、错误码或配置格式。
- 数据兼容性：不涉及数据库 schema、Ent 生成代码或 Atlas migration。
- 运行时影响：降低运行期调用 `logger.Sync()` 后日志丢失或写入失败风险；按日期轮转仍负责关闭旧 writer。
