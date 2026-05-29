## Context

当前项目的日志实现位于 `common/logger`，基于标准库 `log/slog`，由 `common/infrastructure.NewLogger` 提供 `*slog.Logger`。HTTP 请求日志和 panic recovery 中间件手动把 `request_id` 字段写入日志，但基础设施日志和业务日志并不天然携带 trace-id，也没有文件轮转或多文件级别分类输出。

参考项目 `/Users/liubisen/Desktop/sander/Project/my/go-micro-scaffold` 的 `common/logger/logger.go` 使用 Zap、`zapcore.NewTee`、多 writer、context logger helper 和 trace middleware。该实现值得复用的部分是 Zap core 组合、context helper 和请求中间件注入 logger 的方向；需要优化的部分是：参考项目使用 `file-rotatelogs` 而不是用户要求的 lumberjack 类机制，文件分类只有 all/info/error，字段名是 `traceID`，当前项目统一使用结构化字段名 `trace-id` 与 HTTP header `X-Trace-ID`。

## Goals / Non-Goals

**Goals:**

- 将共享日志组件升级为 Zap，并通过 Fx 提供项目统一 logger。
- 所有通过项目 logger API 输出的日志必须包含 `trace-id` 字段；没有 trace-id 时输出空字符串或生成的 trace ID。
- Gin trace-id 中间件必须将 `X-Trace-ID` 写入 Gin context、Go context 和响应头。
- 支持按天轮转并分类输出到 `xxx.all.log`、`xxx.info.log`、`xxx.warning.log`、`xxx.error.log`。
- 提供配置示例和业务代码调用示例，明确如何从 `context.Context` 记录日志。
- 保持 controller/service/repository 分层，不在业务层重复实现 logger 或中间件。

**Non-Goals:**

- 不新增外部日志采集系统、OpenTelemetry、ELK 或远程日志 sink。
- 不改变 HTTP API 路由、响应信封或错误码。
- 不修改 Ent schema 或数据库模型。

## Decisions

- 使用 Zap 作为核心日志库。Zap 已是参考项目使用方案，性能、结构化字段和多 core 输出成熟，适合替换当前 `slog`。
- 在 `common/logger` 提供封装 API，而不是要求业务代码直接处理 trace-id 字段。建议 API 包括 `New(cfg) (*zap.Logger, error)`、`WithTraceID(ctx, traceID)`、`TraceIDFromContext(ctx)`、`WithContext(base, ctx)`、`ToContext(ctx, logger)`、`FromContext(ctx)`、`Info(ctx, msg, fields...)`、`Warn(ctx, msg, fields...)`、`Error(ctx, msg, fields...)`、`Debug(ctx, msg, fields...)`。
- trace-id 字段名统一为 `trace-id`。HTTP header 使用 `X-Trace-ID`，将 `common/middleware/request_id.go` 重命名为 `trace_id.go`，Gin key 改为 `TraceIDKey`，Go context 使用私有 key 避免冲突。
- 多文件输出使用 Zap core 级别过滤：all 文件写入所有达到全局 level 的日志；info 文件只写 Info 级别；warning 文件只写 Warn 级别；error 文件写 Error 及以上。控制台输出按配置可选。
- 日志文件命名使用当前活跃文件名 `xxx.<level>.log`，按天轮转时归档旧文件。为了满足“类似 lumberjack”的大小/备份/保留配置和“按天”要求，可实现一个 `dailyLumberjackWriteSyncer`：按日期包装 `lumberjack.Logger`，日期变化时关闭旧 writer 并打开新 writer，文件名保持 `xxx.<level>.log`，旧文件由 lumberjack 备份策略处理。若实现复杂度过高，可采用 `file-rotatelogs` 达到严格按天轮转，但需要在实现说明中记录偏离用户指定示例库的原因。
- 配置扩展保留当前 `log.level` 与 `log.format`，新增字段示例：

```yaml
log:
  level: info
  format: json
  directory: ./logs
  filename: aegiscore-user-services
  console: true
  max_age_days: 7
  max_size_mb: 100
  max_backups: 30
```

- 初始化代码形态：

```go
func NewLogger(cfg *config.Config) (*zap.Logger, error) {
    return logger.New(cfg)
}
```

- HTTP 中间件顺序必须保证 trace-id 注入先于 request logger 和 recovery：

```go
engine.Use(commonmw.TraceID(), commonmw.Recovery(log), commonmw.RequestLogger(log), commonmw.CORS())
```

- 业务代码调用示例：

```go
func (s *UserService) GetByID(ctx context.Context, id uuid.UUID) (*dto.UserResponse, error) {
    logger.Info(ctx, "query user profile", zap.String("user_id", id.String()))
    user, err := s.repo.GetByID(ctx, id)
    if err != nil {
        logger.Error(ctx, "query user profile failed", zap.String("user_id", id.String()), zap.Error(err))
        return nil, err
    }
    return dto.FromUser(user), nil
}
```

## Risks / Trade-offs

- [Risk] 从 `*slog.Logger` 改为 `*zap.Logger` 是破坏性依赖类型变更。→ Mitigation: 一次性更新 common、user-services 和测试注入点，避免混用两个 logger 类型。
- [Risk] daily writer 和 lumberjack 组合可能增加实现复杂度。→ Mitigation: 将轮转 writer 封装在 `common/logger` 并用可控 clock 或临时目录测试文件命名与分类输出。
- [Risk] “所有日志必须包含 trace-id” 对非请求启动日志不天然成立。→ Mitigation: 项目 logger helper 在没有 trace-id 时仍注入 `trace-id: ""`，启动日志可显式传空 trace-id 或使用 base logger with 默认字段。
- [Risk] 业务代码直接使用 `*zap.Logger` 可能绕过 context trace-id。→ Mitigation: 文档和示例要求业务层优先使用 `common/logger` context helper；中间件将 request-scoped logger 写入 context。

## Migration Plan

1. 扩展 `config.LogConfig` 和 `user-services/configs/config.yaml` 日志配置。
2. 用 Zap 重写 `common/logger`，实现多 writer、多 core、trace-id context helper 和示例中的业务调用 API。
3. 更新 `common/infrastructure.NewLogger` 返回 `*zap.Logger` 并注册 Fx lifecycle `Sync`。
4. 将 `common/middleware/request_id.go` 重命名为 `trace_id.go`，更新 trace-id、request logger、recovery 为 Zap 版本，并把 trace-id 写入 Go context。
5. 更新 `common/infrastructure` 和 `user-services` 中所有 `slog` 注入点与日志调用。
6. 增加 logger 单元测试、中间件 trace-id 测试和 bootstrap 编译/行为测试。
7. 运行 `go test ./...` 于 `common/` 和 `user-services/`。

## Open Questions

- 是否必须严格使用 `gopkg.in/natefinch/lumberjack.v2`，还是允许采用参考项目的 `file-rotatelogs` 达成更直接的按天文件名轮转。默认按用户要求优先使用 lumberjack 类实现。
