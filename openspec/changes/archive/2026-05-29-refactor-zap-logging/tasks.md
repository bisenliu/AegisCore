## 1. 配置与依赖

- [x] 1.1 在 `common/config.LogConfig` 中新增 Zap 文件输出配置字段：directory、filename、console、max_age_days、max_size_mb、max_backups。
- [x] 1.2 更新 `user-services/configs/config.yaml`，提供完整日志配置示例。
- [x] 1.3 在 `common/go.mod` 增加 `go.uber.org/zap` 和日志轮转依赖，优先使用 `gopkg.in/natefinch/lumberjack.v2` 或明确等价替代。
- [x] 1.4 如 `user-services` 需要直接引用 Zap 类型或字段，更新 `user-services/go.mod` 依赖声明。

## 2. Zap Logger 实现

- [x] 2.1 重写 `common/logger`，实现 `New(cfg)` 初始化 Zap logger、多 core 输出和配置化 encoder。
- [x] 2.2 实现 all、info、warning、error 四类文件 writer，输出到 `xxx.all.log`、`xxx.info.log`、`xxx.warning.log`、`xxx.error.log`。
- [x] 2.3 实现按天轮转逻辑，并应用 max_age_days、max_size_mb、max_backups 限制。
- [x] 2.4 实现 trace-id context helper：`WithTraceID`、`TraceIDFromContext`、`WithContext`、`ToContext`、`FromContext`。
- [x] 2.5 实现业务调用 helper：`Debug(ctx, ...)`、`Info(ctx, ...)`、`Warn(ctx, ...)`、`Error(ctx, ...)`，确保输出包含 `trace-id`。
- [x] 2.6 更新 `common/infrastructure.NewLogger`，通过 Fx 提供 `*zap.Logger` 并在 lifecycle stop 时 sync/close logger。

## 3. Middleware 与 Runtime 迁移

- [x] 3.1 将 `common/middleware/request_id.go` 重命名为 `trace_id.go`，将 `RequestID`/`RequestIDKey` 改为 `TraceID`/`TraceIDKey`，并使用 `X-Trace-ID`。
- [x] 3.2 更新 trace-id 中间件，将 trace-id 写入 Gin context、Go context 和响应头。
- [x] 3.3 更新 `common/middleware.RequestLogger` 使用 Zap，输出 trace-id、method、path、status、latency、client_ip。
- [x] 3.4 更新 `common/middleware.Recovery` 使用 Zap，输出 trace-id、panic 和 stack，并保持 `common/response.Envelope` 失败响应。
- [x] 3.5 确认 `user-services/internal/bootstrap.NewGinEngine` 中 trace-id 中间件顺序早于 request logger 和 recovery。

## 4. 服务代码适配

- [x] 4.1 将 `common/infrastructure` 中 Redis/PostgreSQL 日志调用从 slog 迁移到 Zap。
- [x] 4.2 将 `user-services/internal/bootstrap`、Redis/PostgreSQL provider 和 HTTP server 日志调用从 slog 迁移到 Zap。
- [x] 4.3 将 `user-services/internal/entclient` 中 logger 注入与关闭日志迁移到 Zap。
- [x] 4.4 在一个业务层位置补充 context logger 调用示例，优先选择 `user-services/internal/service/user_service.go` 或测试示例，避免改变业务语义。

## 5. 测试与示例

- [x] 5.1 新增 `common/logger` 单元测试，覆盖 logger 初始化、文件分类输出和 trace-id 字段输出。
- [x] 5.2 新增或更新日志轮转测试，验证按天切换或轮转 writer 逻辑可控。
- [x] 5.3 更新 `common/middleware` 测试，覆盖 `X-Trace-ID` 透传、缺失时生成、Go context 写入、请求日志字段和 recovery 字段。
- [x] 5.4 更新基础设施和用户服务 bootstrap 测试，使用 Zap test logger 或 noop logger 替代 slog test logger。
- [x] 5.5 在代码注释、测试或配置中提供业务调用示例，展示 `logger.Info(ctx, "...", zap.String(...))` 的用法。

## 6. 验证

- [x] 6.1 对修改过的 Go 文件运行 `gofmt -w`。
- [x] 6.2 在 `common/` 运行 `go test ./...`。
- [x] 6.3 在 `user-services/` 运行 `go test ./...`。
- [x] 6.4 检查 `slog` 使用点，确认不再遗留在运行时代码中。
