## Why

`common/runtime/scheduler/scheduler.go` 同时承载配置类型、调度器生命周期、任务执行、锁续租和校验逻辑，文件职责过宽，后续维护 scheduler、lock 或 workerpool 相关 runtime primitive 时难以快速定位变更范围。

本次变更通过包内拆分文件降低单文件复杂度，保持 `common/runtime/scheduler` 作为跨服务 runtime primitive 的稳定边界，并为后续 scheduler 行为演进提供更清晰的代码组织。

## What Changes

- 将 `common/runtime/scheduler/scheduler.go` 中的公开配置类型、构造与生命周期、任务执行流程、锁续租辅助、校验逻辑拆分到更聚焦的源码文件。
- 保持 `package scheduler`、导出符号、错误语义、metrics 调用、日志字段、cron parser、锁策略、续租策略、并发控制和 shutdown 行为不变。
- 调整或保留现有单元测试，覆盖拆分后的 scheduler 生命周期、任务执行、锁策略和 Redis locker 行为。
- 不新增数据库 schema、HTTP API、OpenAPI、部署资产或外部依赖。

## Capabilities

### New Capabilities

- 无。

### Modified Capabilities

- `shared-platform-primitives`: 明确 `common/runtime/scheduler` 可以按公开类型、生命周期、任务执行、锁策略、续租和校验职责拆分包内文件，同时必须保持导出 API 与运行时行为不变。

## Impact

- 影响代码：`common/runtime/scheduler/` 下 Go 源码和必要测试。
- 影响 API：不改变导出 API、Go package 名称或调用方式。
- 影响行为：不改变 cron 注册、任务触发、local overlap、global concurrency、Redis lock、auto renew、metrics、日志和 shutdown 语义。
- 影响系统：不涉及 user-service feature、数据库 migration、OpenAPI、部署、观测资产或安全策略变更。
