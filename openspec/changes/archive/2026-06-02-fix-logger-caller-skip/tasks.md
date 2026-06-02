## 1. Caller 行为实现

- [x] 1.1 复现并确认 `common/logger.Info(ctx, ...)` 输出的 `caller` 当前指向 `common/logger/context.go` 包装层。
- [x] 1.2 在 `common/logger` 中调整共享 Zap logger 的 caller skip，使 context API 输出时跳过 logger 包装层并记录业务调用点。
- [x] 1.3 确认调整不会改变 `trace-id` 注入、日志格式、分类文件 writer 或 Error 默认 stacktrace 行为。

## 2. 测试覆盖

- [x] 2.1 增加 logger context API caller 测试，断言 `Info(ctx, ...)` 的 `caller` 指向测试调用文件而不是 `common/logger/context.go`。
- [x] 2.2 覆盖 default logger 与 `ToContext(ctx, log)` 注入 logger 两条路径，避免 caller skip 缺失或重复叠加。
- [x] 2.3 保留并运行现有日志测试，验证分类文件、`trace-id` 和显式 stacktrace 行为未回退。

## 3. 验证与整理

- [x] 3.1 对修改的 Go 文件执行 `gofmt`。
- [x] 3.2 在 `common/` 执行 `go test ./...`。
- [x] 3.3 如实现涉及用户服务调用验证，在 `user-services/` 执行相关测试或 `go test ./...`，确认业务层无需改动调用方式。
