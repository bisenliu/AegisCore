## Why

当前服务运行时没有统一的全局时区初始化约定，日志日期轮转、默认时间格式化、数据库时间字段默认值和业务代码中依赖 `time.Local` 的行为可能随部署环境而变化。

参考现有 `go-micro-scaffold/common/pkg/timezone/module.go` 后，本变更将把可配置时区初始化迁移为 AegisCore 的共享基础设施能力，确保各服务在启动时使用一致且可覆盖的本地时区。

## What Changes

- 在共享配置中增加 `system.timezone`，支持 YAML 与 `AEGISCORE_SYSTEM_TIMEZONE` 环境变量覆盖。
- 在 `common` 模块新增可复用的 timezone Fx module/provider，基于配置加载 IANA 时区名称，默认使用 `Asia/Shanghai`。
- 时区初始化只执行一次，设置 `time.Local` 并同步 `TZ` 环境变量；无效时区应作为启动错误返回，阻止服务以不确定时间环境运行。
- 将用户服务启动流程接入共享 timezone module，确保 HTTP server、日志和业务依赖初始化前具备统一本地时区。
- 补充单元测试覆盖默认值、配置覆盖、环境变量覆盖、无效时区错误与一次性初始化行为。

## Capabilities

### New Capabilities

- 无。

### Modified Capabilities

- `shared-infrastructure`: 增加共享可配置 timezone 初始化能力，并扩展配置加载契约以包含 `system.timezone`。
- `http-service-runtime`: 用户服务启动时必须接入共享 timezone 初始化，使运行时使用配置指定的本地时区。

## Impact

- 受影响代码：`common/config/`、新增 `common/timezone/` 或等价共享包、`common/infrastructure/`、`user-services/internal/bootstrap/`、`user-services/configs/config.yaml`、相关测试。
- 配置兼容性：新增可选配置字段 `system.timezone`，缺省时保持 `Asia/Shanghai`，不破坏现有配置文件。
- API 兼容性：不新增或修改 HTTP 路由、响应信封、错误码或用户资料 API 行为。
- 数据兼容性：不修改 Ent schema、Atlas migration 或数据库连接约定。
- 运行时影响：无效时区配置会导致服务启动失败，并保留底层 `time.LoadLocation` 错误上下文。
