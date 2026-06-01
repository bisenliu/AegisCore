## Context

当前 `common/logger/logger.go` 使用 `dailyLumberjackWriteSyncer` 包装 `lumberjack.Logger`。实现方式是当天写入不带日期的活动文件，例如 `aegiscore-user-services.all.log`；当 `Write` 检测到日期变化时，关闭当前 logger，将活动文件重命名为 `aegiscore-user-services-yyyy-mm-dd.all.log`，再打开新的不带日期活动文件。

这解释了当前现象：修改 macOS 系统日期本身不会让日志文件立即改名，因为轮转逻辑只在下一次日志写入时执行；如果服务没有产生新日志，或者旧进程仍持有文件句柄，就不会发生重命名。即使有新日志，该方案也要处理 rename、目标文件冲突、保留策略和 `lumberjack` 自身大小轮转文件命名之间的差异，技术复杂度较高。

## Goals / Non-Goals

**Goals:**

- 将每日文件命名改为直接带日期，格式为 `aegiscore-user-services.yyyy-mm-dd.<level>.log`。
- 日期变化后，新日志直接写入新日期文件，不依赖旧活动文件 rename。
- 保留 `lumberjack` 处理单文件大小轮转、保留数量和保留天数的能力。
- 保留现有日志级别分类、trace-id、caller 和显式 stacktrace 行为。
- 简化 `common/logger` 文件 writer，减少自定义归档冲突和清理逻辑。

**Non-Goals:**

- 不实现文件系统 watcher 或定时器来“无日志写入也立即重命名”。
- 不改变 HTTP API、响应信封、错误码、数据库 schema、Redis/PostgreSQL 配置或业务逻辑。
- 不修改 Ent schema 或 Atlas migration。
- 不重新启用 Error 日志自动 stacktrace。

## Decisions

1. 放弃方案二作为长期默认策略。

方案二可实现，但它的触发条件是写入路径，而不是系统日期变化事件。操作系统时间改变不会自动调用 Go writer；`lumberjack` 也不会主动把当前打开文件重命名为业务指定的日期格式。因此它对“我改了系统日期，文件名应立即变化”的预期不友好。

2. 采用每日活动文件带日期的命名方案。

`newFileWriters` 不再传入 `filename + ".all.log"`，而是传入逻辑前缀和级别，由 daily writer 基于当前日期生成实际文件名：`prefix.yyyy-mm-dd.level.log`。例如 `filename=aegiscore-user-services` 时，Info 文件为 `aegiscore-user-services.2026-06-02.info.log`。

3. 日期变化时切换目标文件，不再重命名旧文件。

`dailyLumberjackWriteSyncer.Write` 继续在每次写入时检查当前日期。日期变化时关闭旧 `lumberjack.Logger`，创建新的 `lumberjack.Logger{Filename: datedFilename(...)}`。旧日期文件保持原名，天然就是归档文件，不需要 rename、冲突处理或历史文件重命名。

4. 保留 `lumberjack` 的大小轮转能力。

每个日期级别文件仍由 `lumberjack` 写入并按 `MaxSizeMB` 做同日大小轮转。`MaxAgeDays` 和 `MaxBackups` 继续传给 `lumberjack`。因为基础文件名已经带日期，`lumberjack` 的备份文件也会关联对应日期前缀，运维按日期检索更直接。

5. 更新规格和文档，消除“活动文件不带日期”的约定。

主规格应描述长期稳定行为：所有文件日志都写入带日期的每日分类文件。文档中不再推荐 `aegiscore-user-services.<level>.log` 作为当天活动文件名。

## Risks / Trade-offs

- [Risk] 与刚归档的“活动文件无日期”规格相反。→ Mitigation：用本 change 修改 `shared-infrastructure` 主规格，明确新长期行为。
- [Risk] 低频日志服务在跨天后仍要等下一条日志才创建新日期文件。→ Mitigation：这是写路径自然行为；区别是旧文件无需重命名，下一条日志会直接落到新日期文件。
- [Risk] `lumberjack` 大小轮转备份名仍包含时间戳。→ Mitigation：基础文件名已带业务日期，按日期检索不依赖重命名逻辑。
- [Risk] 已存在的无日期活动文件不会自动迁移。→ Mitigation：实现只影响新写入；旧无日期文件可由人工或运维脚本处理，不在运行时代码中迁移历史文件。

## Migration Plan

- 修改 `common/logger` writer，将实际 `lumberjack.Logger.Filename` 生成为 `prefix.yyyy-mm-dd.level.log`。
- 删除或停用不带日期活动文件归档重命名、冲突处理和自定义日期归档清理逻辑。
- 更新 `common/logger/logger_test.go`，断言分类日志写入带日期文件，并验证跨天后新日期文件被创建、旧日期文件保持不变。
- 更新 `docs/DEVELOPMENT.md` 的日志文件命名说明。
- 运行 `gofmt`，并分别在 `common/` 与 `user-services/` 运行 `go test ./...`。

## Open Questions

- 无。实现阶段采用每日活动文件带日期方案。
