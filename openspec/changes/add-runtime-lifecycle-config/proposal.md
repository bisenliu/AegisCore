## Why

`user-service` 的 Fx 启动和关闭超时目前由 `cmd/serve.go` 中的硬编码常量控制，无法按不同环境或不同运行负载调整。随着启动依赖、优雅关闭和潜在后台任务 drain 需求增加，需要将进程级生命周期预算纳入配置文件管理。

## What Changes

- 在运行时配置中新增 `runtime.lifecycle.start_timeout`，用于声明 `aegiscore-user-services serve` 启动 Fx app 的最长等待时间。
- 在运行时配置中新增 `runtime.lifecycle.stop_timeout`，用于声明收到 `SIGINT` 或 `SIGTERM` 后停止 Fx app 的最长等待时间。
- 移除 `user-service/cmd/serve.go` 中的 Fx app 启动和关闭硬编码 timeout 常量，将默认值统一迁移到配置层。
- 保持现有启动和停止编排方式，不改动 HTTP、scheduler、workerpool 或业务处理逻辑。
- 保持配置缺省行为可预测，未声明新字段时使用默认生命周期超时。

## Capabilities

### New Capabilities

- 无。

### Modified Capabilities

- `delivery-operations`: user-service 通过配置文件启动服务时，允许配置 Fx app 进程级启动和关闭超时。

## Impact

- 影响 `common/runtime/config` 的配置结构、默认值、加载和校验。
- 影响 `user-service/cmd/serve.go` 对 Fx app `Start` 和 `Stop` context timeout 的取值来源，并移除该文件中的 lifecycle timeout 默认常量。
- 影响 user-service 示例配置或文档中运行时配置字段的说明。
- 不改变 HTTP API、OpenAPI、数据库 schema、Ent 生成物、RBAC、认证会话、metrics 指标契约或部署资产语义。
