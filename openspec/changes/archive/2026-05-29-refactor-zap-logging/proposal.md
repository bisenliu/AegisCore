## Why

当前项目使用 `log/slog` 和 stdout 输出，缺少按 trace-id 贯穿的统一链路日志，也没有按天切分和按级别分类的文件输出。参考项目的 Zap 实现提供了多输出 core、context logger helper 和请求追踪思路，但需要适配当前项目的 Gin/Fx/config 结构，并补齐 warning 文件、trace-id 字段强制输出和按天轮转策略。

## What Changes

- 将 `common/logger` 从 `slog` 重构为基于 `go.uber.org/zap` 的日志组件。
- 增加日志配置字段，支持 level、format、目录、文件名前缀、是否输出控制台、保留天数、单文件大小和备份数量。
- 引入日志文件轮转机制，按天切分日志文件，并保留大小/保留天数/备份数量限制。
- 增加多文件分类输出：`xxx.all.log`、`xxx.info.log`、`xxx.warning.log`、`xxx.error.log`。
- 实现 trace-id 在 Gin context 与 Go `context.Context` 中传递，并保证通过 context 输出的日志都包含 `trace-id` 字段。
- 将 `common/middleware/request_id.go` 重命名为 `trace_id.go`，并把 HTTP header 从 `X-Request-ID` 改为 `X-Trace-ID`。
- 更新 HTTP 请求日志、recovery 日志和基础设施日志使用 Zap logger。
- 提供初始化代码、配置示例和业务代码调用示例。
- **BREAKING**：共享日志依赖类型将从 `*slog.Logger` 迁移为 `*zap.Logger` 或项目封装的 Zap logger API，现有注入点和日志调用需要同步调整。

## Capabilities

### New Capabilities

- 无。

### Modified Capabilities

- `shared-infrastructure`: 共享日志基础设施从 slog stdout 输出升级为 Zap 初始化、上下文 trace-id 注入、多文件分类输出和按天轮转。
- `http-service-runtime`: HTTP runtime 中间件必须将 trace-id 写入 Go context，并在请求日志和 panic recovery 日志中输出 trace-id。

## Impact

- 影响共享日志代码：`common/logger/`、`common/infrastructure/logger.go`。
- 影响共享中间件：`common/middleware/request_id.go` 将重命名为 `trace_id.go`，并更新 `common/middleware/logging.go`、`common/middleware/recovery.go`。
- 影响配置结构与示例：`common/config/config.go`、`user-services/configs/config.yaml`。
- 影响用户服务 Fx 注入和日志调用：`user-services/internal/bootstrap/`、controller/service/repository 等依赖 logger 的位置。
- 新增依赖：`go.uber.org/zap` 与用于日志轮转的库。优先评估 `gopkg.in/natefinch/lumberjack.v2`；若需要严格按天生成当前文件名，可结合项目内 daily writer 或使用参考项目的 `file-rotatelogs` 思路。
- 不改变 HTTP API 路由、响应信封、数据库模型或 Ent schema。
