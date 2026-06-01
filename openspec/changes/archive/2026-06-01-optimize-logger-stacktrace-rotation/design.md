## Context

`shared-infrastructure` 当前负责提供 Zap logger、trace-id 字段和日志文件分类输出。`common/logger/logger.go` 的 `NewWithConfig` 使用 `zap.New(..., zap.AddCaller(), zap.AddStacktrace(zapcore.ErrorLevel))`，导致所有 Error 级别日志默认包含 stacktrace。项目中现有 Error 使用包含请求校验失败、用户查询/创建失败、HTTP server 异常和 panic recovery，其中 panic recovery 已显式记录 `debug.Stack()`，普通业务/输入错误不需要自动堆栈。

日志文件当前通过 `lumberjack` 写入 `aegiscore-user-services.all.log`、`.info.log`、`.warning.log`、`.error.log`，并有自定义 writer 在日期变化时调用 `Rotate()`。这满足按天触发轮转，但历史文件命名由 `lumberjack` 的默认备份命名决定，不符合明确的 `yyyy-mm-dd` 归档命名策略。

## Goals / Non-Goals

**Goals:**

- 默认 Error 日志不自动输出 stacktrace，降低可预期业务错误和输入错误的日志噪声。
- 提供显式堆栈记录方式，供 panic、不可恢复后台 goroutine 错误或关键基础设施错误使用。
- 明确日志轮转命名策略，并保留现有日志配置字段、分类文件和 trace-id 行为。
- 在 `common/logger` 中集中实现轮转策略，避免在 `user-services` 复制日志能力。

**Non-Goals:**

- 不新增认证、支付、管理 API 或新的业务能力。
- 不改变 HTTP 响应信封、错误码映射、数据库 schema 或 Ent 生成代码。
- 不引入新的必填日志配置项或替换 Zap。
- 不为每个业务 Error 调用点强制添加 stacktrace。

## Decisions

1. 移除全局 `zap.AddStacktrace(zapcore.ErrorLevel)`。

普通 Error 日志仍通过 `zap.Error(err)` 记录错误对象和 caller。需要堆栈的场景显式追加 `zap.Stack("stacktrace")`，或通过 `common/logger.WithStackTrace(fields ...zap.Field) []zap.Field` 之类的轻量包装添加统一字段名。当前 `common/middleware/recovery.go` 已在 panic 场景中显式记录 `debug.Stack()`，因此移除全局 stacktrace 不会丢失 panic 堆栈。

2. Error 调用点按语义分类处理。

请求校验失败、用户资料查询失败、用户创建失败属于可预期或已带错误上下文的业务/输入错误，保留 `zap.Error(err)` 即可。HTTP server `ListenAndServe` 非预期失败和 panic recovery 是候选关键错误，可在实现阶段显式添加 `zap.Stack("stacktrace")` 或保留 recovery 的 `debug.Stack()` 字段。

3. 采用方案二：当天活动日志不带日期，历史归档带日期。

方案二可行，但不适合直接依赖 `lumberjack.Rotate()` 的默认命名，因为 `lumberjack` 归档名包含时间戳且不可配置为 `aegiscore-user-services-yyyy-mm-dd.xx.log`。最佳实践是在 `common/logger` 外层自定义 daily writer：跨天时先关闭当前 `lumberjack.Logger`，将活动文件原子重命名为日期归档文件，再为新的活动文件创建新的 `lumberjack.Logger`。同一天内由 `lumberjack` 继续按大小阈值轮转活动文件；跨天归档由 daily writer 控制日期命名。

4. 归档文件命名规则使用 `prefix-yyyy-mm-dd.level.log`。

活动文件保持现有路径：`prefix.all.log`、`prefix.info.log`、`prefix.warning.log`、`prefix.error.log`。跨天后旧活动文件重命名为 `prefix-YYYY-MM-DD.all.log` 等。如果目标归档文件已存在，追加递增序号避免覆盖，例如 `prefix-YYYY-MM-DD.1.all.log`。这处理服务重启、同日多次归档和外部文件残留。

5. 保留现有配置兼容性。

`MaxSizeMB`、`MaxBackups`、`MaxAgeDays` 继续传给 `lumberjack` 处理大小轮转备份和清理。自定义 daily writer 负责日期边界上的活动文件归档。由于 `lumberjack` 只清理自己命名的备份文件，实现阶段需要补充按日期归档文件的清理逻辑，确保 `MaxAgeDays` 和 `MaxBackups` 也覆盖 `prefix-YYYY-MM-DD*.level.log` 历史文件。

## Risks / Trade-offs

- [Risk] 方案二需要自定义跨天 rename 与历史清理，复杂度高于方案一。→ Mitigation：将逻辑限制在 `common/logger` writer 内，并增加单元测试覆盖跨天、归档冲突、活动文件写入和无活动文件场景。
- [Risk] 同一天大小轮转的 `lumberjack` 备份仍使用其默认时间戳命名，不完全符合每日归档命名格式。→ Mitigation：明确 `prefix-YYYY-MM-DD.level.log` 是跨天归档名；同日大小阈值备份由 `lumberjack` 管理，保留 `MaxSizeMB` 语义。
- [Risk] 跨天重命名活动文件期间写入失败可能导致日志短暂不可写。→ Mitigation：在 writer mutex 下关闭、重命名、重开；错误返回给 Zap，并在下一次写入重试轮转。
- [Risk] 多进程同时写入同一活动日志文件时 rename 可能竞争。→ Mitigation：不支持多进程共享同一 `log.filename`/directory；部署应保证单进程单文件前缀。
- [Risk] 移除自动 stacktrace 会减少普通 Error 日志上下文。→ Mitigation：关键错误显式添加 `zap.Stack("stacktrace")`；保留 `zap.AddCaller()` 和 `zap.Error(err)`。

## Migration Plan

- 修改 `common/logger.NewWithConfig` 去掉 `zap.AddStacktrace(zapcore.ErrorLevel)`。
- 在 `common/logger` 提供显式堆栈辅助函数，或在关键调用点直接使用 `zap.Stack("stacktrace")`。
- 重构 daily writer，使跨天归档将活动文件重命名为带日期的历史文件，并补充历史文件清理。
- 更新 `common/logger/logger_test.go` 覆盖无默认 stacktrace、显式 stacktrace、跨天归档命名和现有分类文件行为。
- 运行 `go test ./...` 于 `common/` 和 `user-services/`。

## Open Questions

- 无。实现阶段默认采用方案二；若单元测试发现与 `lumberjack` 清理语义冲突不可控，再回退到方案一每日活动文件带日期的低复杂度实现。
