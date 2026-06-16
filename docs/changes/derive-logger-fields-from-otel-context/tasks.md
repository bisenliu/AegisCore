# Tasks

## Preparation

- [x] 阅读 `AGENTS.md`、`docs/ARCHITECTURE.md` 和本 change 的 `proposal.md`、`design.md`。
- [x] 确认本 change 目录使用 `docs/changes/derive-logger-fields-from-otel-context/`，不新增 `openspec/` 或 `docs/opsx/`。
- [x] 梳理当前 `common/runtime/logger/context.go` 中 `TraceIDField`、`WithTraceID`、`TraceIDFromContext`、`WithContext` 和 `FromContext` 的行为。
- [x] 梳理 `common/http/middleware` 中 request logger、recovery 和相关测试对 `trace_id` 的断言。
- [x] 梳理 `user-service/internal/providers` 中 request log、panic recovery、Gin OTel tracing 和 Ent SQL debug 测试对 logger 字段的断言。
- [x] 用以下命令记录需要清理的引用：

```bash
rg -n "WithTraceID|TraceIDFromContext|traceIDContextKey|TraceIDField|trace_id|span_id|SpanContextFromContext" common user-service docs AGENTS.md
```

- [x] 确认本变更不新增响应 trace header、不保留 `X-Trace-ID` 兼容路径、不实现 OTLP exporter、不改变日志等级策略或日志文件轮转。

## Logger Implementation

- [x] 在 `common/runtime/logger/context.go` 中新增 `SpanIDField = "span_id"` 字段常量。
- [x] 将 `TraceIDField` 注释从“请求关联 ID”调整为 OTel trace ID 语义。
- [x] 新增包内 helper，从 `trace.SpanContextFromContext(ctx)` 派生日志字段。
- [x] 有效 span context 时返回 `trace_id` 和 `span_id` 两个 zap 字段。
- [x] 无效 span context、nil context 或无 span context 时不返回 trace/span 字段。
- [x] 修改 `WithContext`，只在有效 OTel span context 存在时追加 `trace_id` 和 `span_id`。
- [x] 保持 `WithContext(nil, nil)`、`WithContext(context.Background(), nil)` 等 fallback 行为可用。
- [x] 保持 `FromContext` 继续优先使用 `logger.ToContext` 写入的 base logger，再追加 OTel 字段。
- [x] 保持 `Debug`、`Info`、`Warn`、`Error` 的签名和 caller skip 行为不变。
- [x] 保持 `SQL(base)` 只负责命名 logger，不自行读取或生成 trace/span 字段。

## Remove Custom Trace-ID Helpers

- [x] 删除 `traceIDContextKey`。
- [x] 删除 `WithTraceID`。
- [x] 删除 `TraceIDFromContext`。
- [x] 如果实现阶段必须分步迁移，先为 `WithTraceID` 和 `TraceIDFromContext` 添加 deprecated 注释，但同一变更内继续完成调用方迁移并优先删除。
- [x] 确保生产代码不通过 context value 覆盖 OTel trace ID。
- [x] 确保测试不再通过 `logger.WithTraceID` 伪造日志关联字段。

## Logger Tests

- [x] 更新 `common/runtime/logger/logger_test.go`，使用 OTel span context 构造固定 trace/span ID。
- [x] 增加或调整测试，断言有效 OTel context 下文件日志包含 `"trace_id":"<otel_trace_id>"`。
- [x] 增加或调整测试，断言有效 OTel context 下文件日志包含 `"span_id":"<otel_span_id>"`。
- [x] 增加或调整测试，断言无有效 span context 时日志消息正常输出。
- [x] 增加或调整测试，断言无有效 span context 时不写入空字符串 `trace_id` 或 `span_id`。
- [x] 更新 context logger caller skip 测试，确保新增 helper 不破坏调用方位置。
- [x] 更新 default logger caller skip 测试，确保新增 helper 不破坏调用方位置。
- [x] 更新 SQL logger 文件分类测试，确保常规日志和 SQL 日志仍写入正确文件。

## Common Middleware Tests

- [x] 更新 `common/http/middleware/logging_test.go` 或同类测试中的 OTel context helper，使其同时构造固定 trace ID 和 span ID。
- [x] 更新 request logger 测试，断言 request log 字段包含 `logger.TraceIDField` 和 `logger.SpanIDField`。
- [x] 更新 request logger 测试，保留 method/path/status/client_ip/user_id/latency_ms 等既有字段断言。
- [x] 更新 panic recovery 测试，断言 panic 日志包含 `trace_id` 和 `span_id`。
- [x] 增加或保留无有效 span context 的 request/recovery 日志测试，确认不需要旧 trace-id middleware 也能正常输出。
- [x] 确认 common middleware 不直接调用 OTel API 拼日志字段，字段派生只在 `common/runtime/logger` 内完成。

## User Service Provider Tests

- [x] 更新 `user-service/internal/providers/routes_test.go` 中日志字段断言，增加 `span_id` 有效性检查。
- [x] 保留 request log 的 method/path/status/user_id/client_ip/latency_ms 行为断言。
- [x] 保留 panic recovery envelope 和 panic 日志行为断言，并增加 `span_id` 有效性检查。
- [x] 增加或调整带 `traceparent` 请求的断言，确认日志 `trace_id` 等于 parent trace ID。
- [x] 增加或调整无 `traceparent` 请求的断言，确认日志 `trace_id` 和 `span_id` 均为有效 OTel ID。
- [x] 更新 `user-service/internal/providers/ent_test.go`，用 OTel span context 替代 `logger.WithTraceID`。
- [x] 更新 Ent SQL debug 测试，断言 SQL 日志包含 `trace_id`、`span_id` 和 SQL statement。
- [x] 确认 Gin tracing provider、span rename、health probe filter 测试不因 logger helper 改动退化。

## Direct Caller Migration

- [x] 扫描并迁移所有 `logger.WithTraceID` 调用。
- [x] 扫描并迁移所有 `logger.TraceIDFromContext` 调用。
- [x] 如果测试需要固定值断言，使用测试内 OTel span context helper 替代。
- [x] 如果生产代码需要日志关联字段，保持传递原始 `ctx`，不要手动读取 trace/span ID 后追加 zap field。
- [x] 运行以下扫描并确认没有遗留引用：

```bash
rg -n "WithTraceID|TraceIDFromContext|traceIDContextKey" common user-service
```

## Documentation

- [x] 更新 `AGENTS.md` 中日志规则，说明 `common/runtime/logger` context helper 从 OTel span context 自动追加 `trace_id` 和 `span_id`。
- [x] 更新 `docs/ARCHITECTURE.md` 中日志基础设施说明，补充 `span_id` 字段和无效 span 不伪造 ID 的行为。
- [x] 更新 `docs/DEVELOPMENT.md` 中 observability/logging 说明，补充 `span_id` 和 OTel span context 来源。
- [x] 检查 `docs/changes/replace-custom-trace-id-with-otel-gin/` 是否有仍会误导本变更实现的“只输出 trace_id”描述；如需要，仅做小范围当前行为补充。
- [x] 不为了重写历史而大规模修改旧 change 文档。

## Verification

- [x] 格式化修改过的 Go 文件：

```bash
gofmt -w <changed-go-files>
```

- [x] 运行 common 受影响测试：

```bash
cd common && go test ./runtime/logger ./http/middleware
```

- [x] 运行 user-service provider 测试：

```bash
cd user-service && go test ./internal/providers
```

- [x] 运行 common 测试入口：

```bash
make test-common
```

- [x] 运行 user-service 测试入口：

```bash
make test-user-service
```

- [x] 运行架构边界检查：

```bash
make architecture-lint
```

- [x] 扫描确认没有遗留自定义 trace-id helper：

```bash
rg -n "WithTraceID|TraceIDFromContext|traceIDContextKey" common user-service
```

- [x] 扫描确认没有重新引入旧 HTTP trace header：

```bash
rg -n "X-Trace-ID|HeaderTraceID" common user-service docs AGENTS.md
```

- [x] 扫描确认没有新增 OpenSpec/OPSX 工件：

```bash
find . -maxdepth 3 \( -path './openspec' -o -path './docs/opsx' \) -print
```

- [x] 检查变更范围：

```bash
git diff -- common/runtime/logger common/http/middleware user-service/internal/providers docs/ARCHITECTURE.md docs/DEVELOPMENT.md AGENTS.md docs/changes/derive-logger-fields-from-otel-context
```

## Guardrails

- [x] 不新增 `openspec/` 或 `docs/opsx/`。
- [x] 不保留 `X-Trace-ID` 兼容路径。
- [x] 不新增 `Trace-Id`、`X-Trace-ID` 或其他响应头替代品。
- [x] 不在业务代码中直接依赖 OTel API 读取日志字段。
- [x] 不在 middleware、provider 或 feature use case 中手动拼接 `trace_id` / `span_id` 字段。
- [x] 不改变日志等级策略、日志文件轮转、日志格式、敏感字段规则或命名 logger 规则。
- [x] 不新增日志采样。
- [x] 不实现 OTLP exporter、metrics exporter、dashboard 或告警。
- [x] 不修改数据库 schema、Ent generated code、Atlas migration 或 Redis key schema。
- [x] 不在日志、错误消息、配置样例或文档中写入 password、token、Authorization header、Cookie、真实 OTLP endpoint 或其他敏感凭据。
