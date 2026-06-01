## 1. Stacktrace 行为调整

- [x] 1.1 修改 `common/logger.NewWithConfig`，移除默认 `zap.AddStacktrace(zapcore.ErrorLevel)`，保留 `zap.AddCaller()`。
- [x] 1.2 在 `common/logger` 中提供显式堆栈记录方式，优先实现轻量辅助函数或明确使用 `zap.Stack("stacktrace")` 的调用约定。
- [x] 1.3 审查当前 Error 日志调用点，将请求校验失败、用户查询/创建失败等可预期错误保持为无堆栈日志。
- [x] 1.4 对关键错误调用点补充显式堆栈记录或确认已有显式堆栈字段，例如 HTTP server 异常和 panic recovery。

## 2. 日志轮转命名实现

- [x] 2.1 重构 `common/logger` daily writer，使活动文件保持 `prefix.level.log` 不带日期。
- [x] 2.2 实现跨天归档：关闭当前 writer，将昨日活动文件重命名为 `prefix-yyyy-mm-dd.level.log`，再打开新的活动文件。
- [x] 2.3 实现归档冲突处理，确保已有 `prefix-yyyy-mm-dd.level.log` 时不会覆盖历史内容。
- [x] 2.4 补充日期归档文件清理逻辑，使 `MaxAgeDays` 与 `MaxBackups` 对自定义日期归档文件生效。
- [x] 2.5 保留 `lumberjack` 的同日大小轮转能力，并确认 `MaxSizeMB` 继续作用于活动日志文件。

## 3. 测试覆盖

- [x] 3.1 更新 `common/logger/logger_test.go`，验证普通 Error 日志默认不包含 stacktrace 字段。
- [x] 3.2 添加测试验证显式 `zap.Stack("stacktrace")` 或 logger 堆栈辅助函数会输出 stacktrace 字段。
- [x] 3.3 更新跨天轮转测试，验证昨日活动文件归档为 `prefix-yyyy-mm-dd.level.log` 且新日志写入 `prefix.level.log`。
- [x] 3.4 添加归档冲突测试，验证同日期历史文件不会被覆盖。
- [x] 3.5 添加保留策略测试，验证日期归档文件会按 `MaxAgeDays` 或 `MaxBackups` 清理。

## 4. 验证与文档同步

- [x] 4.1 运行 `gofmt` 格式化修改过的 Go 文件。
- [x] 4.2 在 `common/` 运行 `go test ./...`。
- [x] 4.3 在 `user-services/` 运行 `go test ./...`，确认日志行为变更未破坏服务运行时。
- [x] 4.4 更新必要的开发说明或配置注释，说明 Error 日志默认无堆栈、关键错误需显式添加 `zap.Stack("stacktrace")`。
