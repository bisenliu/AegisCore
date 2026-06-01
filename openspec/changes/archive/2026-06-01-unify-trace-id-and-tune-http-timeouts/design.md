## Context

`common/middleware/trace_id.go` 负责在 Gin 请求链路中读取或生成 trace 标识，并写入 `X-Trace-ID` 响应头、Gin context key `trace_id` 和 Go `context.Context`。`common/logger/context.go` 负责从 Go context 读取 trace 标识并输出 Zap 字段 `trace-id`。这三个名称处在不同边界：HTTP 协议、Gin 内部 key、日志字段；当前代码行为基本可用，但需要通过规格和少量实现整理明确它们属于同一 trace 标识，避免后续代码误以为需要多个 trace 字段。

`user-services/configs/config.yaml` 当前 HTTP timeout 为 `read_timeout: 10s`、`write_timeout: 10s`、`idle_timeout: 60s`、`shutdown_timeout: 10s`。在请求量较大、客户端和网关复用 keep-alive 的场景中，较短的 idle timeout 会增加连接重建和 TLS/代理层开销；较短的 write timeout 也可能放大长尾请求或较慢客户端导致的失败。用户提出的 `30s/60s/120s/25s` 对普通 JSON API 是合理的偏稳健配置，但需要认识到它不是单纯“越大越好”：过长 timeout 会增加慢连接和长尾 handler 占用资源的时间。

## Goals / Non-Goals

**Goals:**

- 明确 trace 标识统一策略：`X-Trace-ID` 是 HTTP header，`trace_id` 是 Gin context key，`trace-id` 是日志字段，它们必须承载同一个值。
- 保持 `common` 作为 trace 与 logger 共享能力的归属位置，避免在 `user-services` 重复实现中间件或日志字段转换。
- 将用户服务示例配置调整为 `read_timeout: 30s`、`write_timeout: 60s`、`idle_timeout: 120s`、`shutdown_timeout: 25s`。
- 通过测试覆盖 trace 值在 HTTP header、Gin context、Go context 和日志字段之间的一致性，以及配置文件 timeout 反序列化结果。

**Non-Goals:**

- 不引入 W3C `traceparent`、OpenTelemetry 或分布式追踪后端。
- 不更改 HTTP header 名称、响应 envelope、错误码或 API 路由。
- 不更改 `common/config.Load` 的职责；配置 loader 仍只读取、覆盖和反序列化，不做 timeout 范围校验。
- 不调整数据库、Redis、Ent schema 或 Atlas migration。

## Decisions

1. 保持现有三个边界名称，不强制改成单一字符串。

   原因：`X-Trace-ID` 符合 HTTP header 命名习惯，`trace_id` 符合 Gin context key 和已有内部调用习惯，`trace-id` 已是共享日志规格要求。把它们全部改成同一个字符串会造成不必要的外部或内部兼容风险。替代方案是改日志字段为 `trace_id` 或改 Gin key 为 `trace-id`，但这会破坏现有日志检索约定或增加内部 key 迁移成本。

2. 在 `common/logger` 中保留 `TraceIDField = "trace-id"`，在 `common/middleware` 中保留 `TraceIDKey = "trace_id"` 与 `HeaderTraceID = "X-Trace-ID"`。

   原因：这些常量清晰表达不同边界的规范名称，且当前调用方可以继续复用。实现层面只需要确保中间件写入 Go context 后，所有 `logger.FromContext` 和 `logger.Info/Warn/Error` 输出都使用同一个 trace 值。

3. 将 HTTP timeout 示例配置调整为 `30s/60s/120s/25s`，不在代码中增加硬编码特殊逻辑。

   原因：该项目已经通过 YAML 与 `AEGISCORE_` 环境变量覆盖运行时配置，默认配置文件应表达推荐基线，具体生产环境仍可按网关、LB、P99 延迟和部署平台 termination grace period 覆盖。替代方案是在启动代码里针对零值或环境自动计算 timeout，但这会扩大运行时行为复杂度，也违背 config loader 不做策略校验的边界。

4. 对高请求量场景采用“更高 keep-alive 复用 + 有限长尾容忍”的策略。

   原因：`idle_timeout: 120s` 有利于请求量大且连接复用明显的服务降低握手和连接建立开销；`read_timeout: 30s` 和 `write_timeout: 60s` 给请求读取和响应写出更宽松窗口；`shutdown_timeout: 25s` 给已有请求在发布或终止期间完成的机会。该策略要求外层网关、负载均衡器 idle timeout 不短于或明确协调该值，并需要监控并发连接数、goroutine、文件描述符、请求 P95/P99 和超时错误。

## Risks / Trade-offs

- 慢客户端或恶意 slowloris 风险增加 → 继续保留有限 `read_timeout`，生产环境应由网关、WAF 或 LB 做连接层防护，并监控读取超时和连接占用。
- 空闲连接驻留时间变长导致文件描述符和内存占用上升 → 通过进程 fd limit、Gin/Go server 指标、LB 连接数和资源告警校准 `idle_timeout`。
- `write_timeout: 60s` 可能掩盖 handler 长尾问题 → 应结合请求日志 latency 与数据库/Redis 慢查询监控定位根因，不能把 timeout 调大作为性能优化替代品。
- `shutdown_timeout: 25s` 与部署平台终止宽限期不匹配 → Kubernetes 或进程管理器的 termination grace period 必须大于 25s，并预留 Fx stop 和连接关闭时间。
- trace 命名统一被误解为需要改字段名 → 规格中明确这是跨边界语义统一，不是对 HTTP header、Gin key 或日志字段的重命名。
