## Why

当前共享日志组件默认对所有 Error 级别日志添加堆栈信息，容易在业务可预期错误中产生噪声并增加日志体积；同时日志文件需要更清晰的按天归档命名策略，方便运维按日期定位和保留历史日志。

## What Changes

- 移除 `common/logger.NewWithConfig` 中对 Error 级别自动 `AddStacktrace` 的默认行为。
- 为需要堆栈的关键错误提供显式记录方式，例如 `zap.Stack()` 或共享 logger 包装函数，避免普通 Error 日志自动携带 stacktrace。
- 调整文件日志轮转策略，明确活动日志与历史日志的命名约定、跨天轮转行为、大小轮转与保留策略的交互。
- 评估两种命名方案：每日活动文件带日期，以及当天活动文件不带日期、历史归档带日期；优先采用技术复杂度低且与 `lumberjack` 行为兼容的方案。
- 不改变 HTTP API、响应信封、数据库 schema、Redis/PostgreSQL 命名实例或业务接口。

## Capabilities

### New Capabilities

- 无。

### Modified Capabilities

- `shared-infrastructure`: 修改 Zap 日志的 stacktrace 默认行为与文件轮转命名要求。

## Impact

- 影响代码：`common/logger/logger.go` 及其测试；可能涉及少量关键调用点显式添加 `zap.Stack()` 或 logger 包装函数。
- 影响规格：更新 `openspec/specs/shared-infrastructure/spec.md` 对共享日志能力的要求。
- 配置兼容性：继续使用现有 `log.level`、`log.format`、`log.directory`、`log.filename`、`log.console`、`log.max_age_days`、`log.max_size_mb`、`log.max_backups` 配置，不引入新的必填配置。
- 运行行为：普通 Error 日志不再自动包含 stacktrace；只有显式请求堆栈的日志才输出 stacktrace 字段。日志文件按天轮转并使用新的归档命名规则。
