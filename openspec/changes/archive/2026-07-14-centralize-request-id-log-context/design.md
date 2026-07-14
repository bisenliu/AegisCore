## Context

`common/http/middleware.RequestID` 已在 Gin middleware 链中先于 recovery、request logger 和业务 handler 执行，并把最终 request ID 写入 `c.Request.Context()`。但是该值使用 `common/http/middleware` 私有 context key 保存，`common/runtime/logger.fieldsFromContext` 无法读取；通用日志 helper 因而只附加有效 OTel span 中的 `trace_id` 和 `span_id`。HTTP access log 则在 `common/http/middleware/log_fields.go` 单独读取 request ID 并附加字段，形成两套日志关联路径。

本变更跨越 `common/runtime/logger` 与 `common/http/middleware` 的共享边界，并会删除公开 Go API。仓库内已知调用方位于 common middleware 测试和 `user-service/internal/providers/gin_request_id_test.go`，必须在同一次实施中原子迁移。`common/http/binding` 仍只负责绑定、校验、错误响应和业务错误字段，不应为了补齐 request ID 而依赖或重复实现 HTTP middleware 的上下文提取逻辑。

## Goals / Non-Goals

**Goals:**

- 让请求生命周期内通过 `logger.WithContext`、`logger.FromContext` 和 `logger.Debug|Info|Warn|Error` 产生的日志自动携带相同的 `request_id`。
- 让 `trace_id`、`span_id` 与 `request_id` 独立提取：无有效 span 时仍可记录 request ID，有有效 span 时三者并存。
- 将 request ID 的日志字段名和 context API 统一归属到 `common/runtime/logger`，消除 HTTP access log 的重复字段拼装。
- 一次性删除 `common/http/middleware` 的旧公开 request ID context API，并迁移仓库内所有调用方，不保留别名、转发函数或 deprecated wrapper。
- 保持 `X-Request-ID`、HTTP access log 其他字段、tracing 传播、日志级别和错误响应行为不变。

**Non-Goals:**

- 不修改 request ID 的合法性规则、生成算法、header 名称或响应头行为。
- 不把 request ID 加入 Prometheus label、span attribute、业务响应 envelope 或持久化数据。
- 不调整 Gin middleware 顺序，不引入新的 logger abstraction、外部依赖或服务级配置。
- 不修改业务 endpoint、OpenAPI、数据库 schema、Atlas migration、部署清单、Grafana dashboard、Prometheus alert 或安全授权边界。

## Decisions

### Decision: request ID 日志上下文归属 `common/runtime/logger`

`RequestIDField`、`WithRequestID` 和 `RequestIDFromContext` 迁移到 `common/runtime/logger`。logger 使用自己的不可导出 context key 保存 request ID，HTTP middleware 调用该公开 API 写入最终值。`common/runtime/logger` 不导入 Gin 或 `common/http/middleware`，从而保持 runtime logger 对 HTTP 框架无依赖；`common/http/middleware` 作为上层适配器可以依赖 logger。

未选择在 `common/http/binding.BindOrAbort` 中手工追加 `request_id`，因为该方式只能修复参数校验日志，recovery、认证失败和后续应用日志仍会继续漂移。也不保留 middleware 包中的兼容转发函数，因为本次已明确采用不兼容迁移，双入口会延续所有权不清并增加后续清理成本。

### Decision: 关联字段按来源独立提取

`logger.fieldsFromContext` 先建立小容量字段集合：有效 OTel span 贡献 `trace_id` 和 `span_id`，logger request ID context 独立贡献 `request_id`。实现不能因为 span 无效而提前返回，否则关闭 tracing、未采样或非 HTTP 测试上下文中的 request ID 会再次丢失。

该设计保持现有规则：无有效 span 时仍省略 `trace_id` 和 `span_id`；仅当 request ID context 存在且值非空时输出 `request_id`。不会从任意 HTTP header、Gin key 或普通字符串 context key 推断字段。

### Decision: access log 复用通用 logger 关联字段

`RequestLoggerWithOptions` 已通过 `logger.WithContext(c.Request.Context(), httpLog)` 创建请求 logger，因此 `requestLogFields` 删除 request ID 的手工读取和追加，只保留 method、path、status、latency、client IP 与 user ID 等 access log 专用字段。这样 request ID 只有一个字段来源，避免同名 zap field 重复。

参数校验代码不增加生产逻辑；`BindOrAbort` 继续调用 `logger.Warn(c.Request.Context(), ...)`，并通过新增测试证明其日志自然获得 request ID。

### Decision: 公开 API 原子迁移且不保留兼容层

删除 `common/http/middleware.RequestIDField`、`WithRequestID` 和 `RequestIDFromContext`。仓库内引用全部改为 `common/runtime/logger` 对应符号，包含 common middleware 测试和 user-service provider 测试。生产代码和测试中不得保留旧符号、type alias、常量 alias、转发函数或 deprecated wrapper。

该变更要求所有基于本仓库 common module 的消费方同步升级；不支持旧 middleware API 与新 logger API 混用。

### Decision: 模块与文档同步边界

- `common/runtime/logger`：拥有通用日志关联 context、字段常量与字段提取。
- `common/http/middleware`：拥有 `X-Request-ID` 的校验、生成、透传、响应头和 Gin 接线，不再拥有日志关联 context API。
- `common/http/binding`：不新增跨包 request ID 读取，仅补充行为测试。
- `user-service`：只迁移测试调用方，不新增服务私有 request ID adapter。
- `deployments`：无变更；日志输出字段新增不要求修改 dashboard 或 alert。
- `docs/openspec`：通过本 change delta 更新稳定观测行为；能力地图仍归属 `runtime-observability`，无需新增 capability。

## Risks / Trade-offs

- [Risk] access log 仍手工追加 request ID 会产生重复同名 zap field -> Mitigation：删除 `requestLogFields` 中的追加逻辑，并增加字段唯一性测试。
- [Risk] `fieldsFromContext` 的控制流修改可能导致无 span 场景错误附加空 trace 字段 -> Mitigation：分别覆盖仅 request ID、仅有效 span、两者并存和空 context 四类单元测试。
- [Risk] 删除公开 middleware API 会使未同步的外部消费方编译失败 -> Mitigation：将 breaking change 写入 proposal/spec，并要求所有仓库内调用方原子迁移；不以兼容层掩盖编译期迁移信号。
- [Risk] 将高基数 request ID 自动加入更多应用日志会增加日志存储量 -> Mitigation：仅在显式携带 request ID 的请求 context 中增加单个字符串字段，不加入 metrics label，不复制到后台 context。
- [Trade-off] logger 包理解 `request_id` 这一关联概念，扩大了其字段契约 -> 该字段已是稳定日志关联要求，集中所有权比让每个 HTTP helper 重复拼装更可验证。

## Migration Plan

1. 在 `common/runtime/logger` 增加 request ID 字段常量和 context API，并扩展关联字段提取及单元测试。
2. 修改 HTTP Request ID middleware 使用 logger context API，删除 middleware 旧公开符号。
3. 删除 access log 的 request ID 手工拼装，迁移 common 与 user-service 的全部引用并补充参数校验日志测试。
4. 运行定向测试、`make user-service-architecture-lint`、暂存本次预期变更后运行 `make lint` 和 `make verify`。
5. 以一次原子提交交付。回滚时整体恢复旧 middleware API、私有 context key 和 access log 手工字段提取；不进行新旧 API 混合回滚。

## Verification

- `go test ./common/runtime/logger/...` 使用对应 module 工作目录或 workspace 可识别的等价命令验证 logger context 行为。
- `go test ./common/http/binding/... ./common/http/middleware/...` 使用对应 module 工作目录或 workspace 可识别的等价命令验证参数校验与 HTTP 日志行为。
- `go test ./user-service/internal/providers/...` 使用对应 module 工作目录或 workspace 可识别的等价命令验证 Gin middleware 接线。
- `rg` 检查仓库不存在旧 middleware request ID context API 的定义或引用。
- `openspec validate centralize-request-id-log-context` 与 `make user-service-architecture-lint` 验证规格和架构约束。
- 实施与文档任务完成并暂存预期变更后运行 `make lint` 和 `make verify`。

## Open Questions

无。
