## MODIFIED Requirements

### Requirement: Provide Zap logging with trace-id and file rotation

系统必须提供基于 Zap 的共享日志组件。日志组件必须支持从 YAML 与 `AEGISCORE_` 环境变量加载日志级别、格式、目录、文件名前缀、控制台输出、保留天数、单文件大小和备份数量。所有通过项目 logger context API 输出的日志必须包含 `trace-id` 字段。日志必须按天轮转，并按照级别写入当天活动文件 `xxx.all.log`、`xxx.info.log`、`xxx.warning.log`、`xxx.error.log`。跨天归档时，前一日活动日志必须重命名为 `xxx-yyyy-mm-dd.all.log`、`xxx-yyyy-mm-dd.info.log`、`xxx-yyyy-mm-dd.warning.log`、`xxx-yyyy-mm-dd.error.log` 格式的历史文件。普通 Error 级别日志不得默认自动包含 stacktrace；需要堆栈的关键错误必须通过显式字段记录。

#### Scenario: Initialize Zap logger from config
- **Given** YAML 配置包含 log level、format、directory、filename、console、max_age_days、max_size_mb 和 max_backups
- **When** `common/logger.New` 被调用
- **Then** 系统必须创建 Zap logger
- **Then** logger 必须按配置输出 JSON 或 console 格式
- **Then** logger 必须在配置的目录下准备分类日志文件 writer
- **Then** logger 必须记录 caller 信息
- **Then** logger 不得默认对所有 Error 及以上日志自动添加 stacktrace

#### Scenario: Write logs to classified files
- **Given** 日志文件名前缀为 `aegiscore-user-services`
- **When** 系统输出 Debug、Info、Warn 和 Error 日志
- **Then** 达到全局 level 的所有日志必须写入当天活动文件 `aegiscore-user-services.all.log`
- **Then** Info 级别日志必须写入当天活动文件 `aegiscore-user-services.info.log`
- **Then** Warn 级别日志必须写入当天活动文件 `aegiscore-user-services.warning.log`
- **Then** Error 及以上日志必须写入当天活动文件 `aegiscore-user-services.error.log`

#### Scenario: Rotate log files daily
- **Given** 服务持续运行跨过本地日期边界
- **When** 新日期的第一条日志写入
- **Then** logger 必须关闭前一日活动日志 writer
- **Then** logger 必须将前一日活动日志归档为包含前一日本地日期的历史文件
- **Then** logger 必须继续向不带日期的当天活动日志文件写入新日志
- **Then** 保留天数、单文件大小和备份数量限制必须按配置生效

#### Scenario: Archive daily logs without overwriting existing history
- **Given** 归档目标文件 `aegiscore-user-services-2026-05-31.error.log` 已存在
- **When** logger 需要归档 2026-05-31 的 `aegiscore-user-services.error.log`
- **Then** logger 必须选择不覆盖已有文件的归档名称
- **Then** logger 必须保留原有历史文件内容

#### Scenario: Include trace-id from context
- **Given** `context.Context` 中存在 trace-id
- **When** 业务代码调用 `common/logger.Info(ctx, ...)`、`Warn(ctx, ...)` 或 `Error(ctx, ...)`
- **Then** 输出日志必须包含字段 `trace-id` 且值等于 context 中的 trace-id

#### Scenario: Log without request context
- **Given** `context.Context` 中不存在 trace-id
- **When** 系统启动流程或基础设施代码输出日志
- **Then** 输出日志仍必须包含 `trace-id` 字段
- **Then** 字段值必须为空字符串或系统明确生成的 trace-id

#### Scenario: Log expected error without stacktrace
- **Given** 业务代码调用 `common/logger.Error(ctx, "query user profile failed", zap.Error(err))`
- **When** logger 写出该 Error 日志
- **Then** 日志必须包含错误字段、caller 字段和 `trace-id` 字段
- **Then** 日志不得因为 Error 级别自动包含 stacktrace 字段

#### Scenario: Log critical error with explicit stacktrace
- **Given** 关键错误调用点显式传入 `zap.Stack("stacktrace")` 或共享 logger 堆栈辅助函数生成的字段
- **When** logger 写出该 Error 日志
- **Then** 日志必须包含显式请求的 stacktrace 字段
- **Then** 该行为不得重新启用所有 Error 级别日志的自动 stacktrace
