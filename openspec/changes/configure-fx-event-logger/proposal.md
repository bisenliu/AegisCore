## Why

user-service 已经通过共享 runtime logger 提供结构化 `*zap.Logger`，但顶层 Fx App 未配置 `fx.WithLogger`，导致 Fx 构图、Invoke、constructor、rollback 和 lifecycle 事件仍使用默认 console logger。当前 Fx 版本已经暴露 `fxevent.Run.Runtime`、`BeforeRun`、lifecycle executing/executed 和 module trace 等初始化观测信息，需要接入统一日志以便生产环境定位慢构造、失败初始化和关闭耗时。

## What Changes

- 为共享 logger runtime 增加 Fx event logger provider，基于当前 App 注入的 `*zap.Logger` 创建命名为 `fx` 的 `fxevent.ZapLogger`。
- 在 user-service 顶层 `AppOptions` 中通过 `fx.WithLogger` 启用该 event logger，使 Fx 自身事件进入统一日志格式、字段和输出目标。
- Fx event logger 使用 debug 级别记录常规构图和运行事件，使用 error 级别记录失败事件，并保持快速、非阻塞，不在 `LogEvent` 路径执行网络 I/O。
- 增加必要测试，覆盖 App options 包含自定义 Fx logger，以及 logger 构造不改变现有 logger 生命周期同步语义。

## Capabilities

### New Capabilities

- 无。

### Modified Capabilities

- `runtime-observability`: 扩展运行时日志观测要求，使 Fx 初始化、构图、Invoke、rollback 和 lifecycle 事件必须使用统一结构化 logger 输出。

## Impact

- 影响代码：`common/runtime/logger/fx.go`、`user-service/internal/bootstrap/app.go` 以及相关单元测试。
- 影响观测：Fx 初始化和 lifecycle 事件将进入与业务日志一致的 zap 输出链路，日志字段和目标与服务 logger 保持一致。
- 不影响 HTTP API、OpenAPI 文档、数据库 schema、Atlas migration、RBAC 权限、部署资产或外部依赖版本。
