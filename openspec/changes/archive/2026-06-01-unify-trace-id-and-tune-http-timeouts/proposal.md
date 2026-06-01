## Why

当前 trace 标识在 HTTP 头、Gin context、Go context 与日志字段之间存在命名差异，需要明确哪些名称属于对外协议、哪些名称属于日志字段，避免后续中间件和业务日志接入时产生不一致。用户服务请求量较大时，现有 HTTP timeout 配置偏短且 keep-alive 保留时间较低，可能增加连接重建开销并影响较慢客户端或长尾请求的稳定性。

## What Changes

- 统一 trace-id 约定：保持 HTTP header 使用 `X-Trace-ID`、Gin context key 使用 `trace_id`、日志字段使用 `trace-id`，并在共享日志和中间件规格中明确这是同一 trace 标识在不同边界的规范表达。
- 保持 trace-id 中间件行为兼容：继续从 `X-Trace-ID` 读取合法值，缺失或不安全时生成新值，并写回 Gin context、Go context 与响应 header。
- 调整用户服务示例 HTTP runtime 配置，将 `read_timeout` 改为 `30s`、`write_timeout` 改为 `60s`、`idle_timeout` 改为 `120s`、`shutdown_timeout` 改为 `25s`。
- 在设计中评估这些 timeout 对高请求量服务的合理性，强调它们适合普通 JSON API 和较高 keep-alive 复用场景，但仍需结合网关、LB、handler 延迟和资源容量监控校准。
- 不引入新的 trace header、不改变 API 响应 envelope、不改变数据库、Redis 或 Ent 行为。

## Capabilities

### New Capabilities

- 无。

### Modified Capabilities

- `shared-infrastructure`: 明确共享 logger context API 与 trace-id 中间件使用的是同一 trace 标识，日志字段规范为 `trace-id`，不得因 HTTP/Gin 内部 key 命名不同而输出不一致字段。
- `http-service-runtime`: 明确用户服务默认 HTTP timeout 配置适配较高请求量和 keep-alive 复用场景，并保持 graceful shutdown 使用配置值。

## Impact

- 影响代码与配置：`common/middleware/trace_id.go`、`common/logger/context.go`、`user-services/configs/config.yaml`，以及可能的相关单元测试或规格文档。
- 外部兼容性：HTTP header 仍为 `X-Trace-ID`，响应 header 不变；日志字段保持 `trace-id`，不会引入 breaking change。
- 运行时影响：更长的 read/write timeout 会提高慢请求容忍度，但在恶意慢客户端或 handler 堆积时会延长连接占用；更长的 idle timeout 可提高 keep-alive 复用率，但会增加空闲连接驻留资源；`shutdown_timeout: 25s` 可给长尾请求更多完成时间，但部署平台终止宽限期必须大于该值。
