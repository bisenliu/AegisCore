## Why

当前“当天活动文件不带日期、跨天后重命名历史文件”的方案依赖下一次日志写入触发轮转；仅修改系统时间或服务长时间无日志写入时，`aegiscore-user-services.xx.log` 不会立即重命名为日期文件，行为不符合按日期直接定位日志文件的运维预期。

## What Changes

- 将文件日志命名策略改为每日活动文件直接带日期：`aegiscore-user-services.yyyy-mm-dd.all.log`、`.info.log`、`.warning.log`、`.error.log`。
- 移除跨天时重命名不带日期活动文件为日期归档文件的方案，避免依赖打开文件句柄 rename、下一次写入触发和冲突归档命名。
- 保留按级别分类、Zap caller、trace-id、显式 stacktrace、`MaxSizeMB`、`MaxBackups` 和 `MaxAgeDays` 配置语义。
- 使用日期变化时切换当前 `lumberjack.Logger` 目标文件的方式实现每日文件，不再维护活动文件与归档文件两套命名。
- 不改变 HTTP API、响应信封、数据库 schema、Redis/PostgreSQL 命名实例或业务接口。

## Capabilities

### New Capabilities

- 无。

### Modified Capabilities

- `shared-infrastructure`: 修改 Zap 文件日志的每日命名与跨天轮转要求。

## Impact

- 影响代码：`common/logger/logger.go` 和 `common/logger/logger_test.go`；可能同步更新 `docs/DEVELOPMENT.md`。
- 影响规格：更新 `openspec/specs/shared-infrastructure/spec.md` 中共享日志文件命名约定。
- 配置兼容性：继续使用现有 `log.directory`、`log.filename`、`log.max_age_days`、`log.max_size_mb`、`log.max_backups` 等配置，不引入新配置项。
- 运行行为：当天日志直接写入带日期文件；跨天后新日志写入新日期文件，旧日期文件保持原名，不再从无日期文件重命名。
