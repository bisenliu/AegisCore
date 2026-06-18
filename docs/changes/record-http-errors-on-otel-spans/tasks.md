# Tasks

## Preparation

- [x] 阅读 `AGENTS.md`、`docs/ARCHITECTURE.md` 和本 change 的 `proposal.md`、`design.md`。
- [x] 确认本 change 目录使用 `docs/changes/record-http-errors-on-otel-spans/`，不新增 `openspec/` 或 `docs/opsx/`。
- [x] 梳理 `common/http/middleware/recovery.go` 的 panic 恢复、日志和响应输出顺序。
- [x] 梳理 `common/http/middleware/logging.go` 的 `c.Next()` 后状态码读取和日志级别选择。
- [x] 梳理 `common/http/response/error.go`、`helper.go`、`writer.go` 中 `Fail`、`WriteError`、`ValidationFailedWithErrors` 和 `JSON` 的调用关系。
- [x] 梳理 `common/contract/errors.Error` 的 `Code`、`HTTPStatus`、`Message` 和 `Cause` 字段，确认哪些字段可进入 span。
- [x] 用以下命令记录当前相关引用：

```bash
rg -n "Recovery\\(|RequestLogger|WriteError\\(|ValidationFailedWithErrors\\(|response\\.Fail|RecordError|SetStatus|SetAttributes|contracterrors\\.Error" common user-service
```

- [x] 确认本变更不改变 response envelope、HTTP status、错误码、日志等级策略或日志敏感字段规则。

## Span Helper Implementation

- [x] 在合适的共享包内新增 span error helper，优先保持包内私有，避免新增业务可见 API。
- [x] helper 使用 `trace.SpanFromContext(ctx)` 获取当前 span。
- [x] 无有效 span context 或 non-recording span 时保持 no-op。
- [x] 为 panic 提供固定、sanitized 的 error event 记录逻辑。
- [x] 为 5xx server error 提供 span status error 标记逻辑。
- [x] 为应用错误提供 `error_code` 和 `http_status` attribute 标注逻辑。
- [x] 只在 HTTP status `>= 500` 时调用 `RecordError` 或 `SetStatus(codes.Error, ...)`。
- [x] 普通 4xx 应用错误只标注低基数字段，不设置 `codes.Error`。
- [x] 不将 `err.Error()`、`appErr.Message`、`appErr.Cause`、panic payload、stacktrace、请求体、header、Cookie、DSN 或 SQL 参数写入 span。
- [x] 保持新增 Go 注释为中文，日志消息如有新增则使用英文。

## Panic Recovery

- [x] 修改 `common/http/middleware/recovery.go`，panic 时先将当前 span 记录为 error。
- [x] panic span event 使用固定短文本和低基数字段，不记录 stacktrace 原文。
- [x] panic 时设置 span status 为 error。
- [x] 保持现有 `"panic recovered"` error 日志行为不变。
- [x] 保持现有 `zap.Any("panic", r)` 和 `zap.String("stack", string(debug.Stack()))` 日志字段行为不变，除非实现阶段已有安全规则要求进一步收敛。
- [x] 保持 `response.Fail(c, contracterrors.InternalError(nil))` 返回现有内部错误信封。
- [x] 保持 `c.Abort()` 行为。

## 5xx Status Marking

- [x] 修改 `common/http/middleware.RequestLoggerWithOptions`，在读取最终 status 后对 5xx 标记 span error status。
- [x] 确保 span status 标记发生在 request logger skip 判断之前或确认 skip 不会漏掉失败请求。
- [x] 保持现有 access log 字段 `trace_id`、`span_id`、`user_id`、`client_ip`、`method`、`path`、`status`、`latency_ms` 不变。
- [x] 保持现有日志级别策略：5xx 为 Error，4xx 为 Warn，其他为 Info。
- [x] 确认 request logger 适合承载该逻辑，无需新增轻量 middleware。
- [x] 未新增 middleware，因此无需调整 provider 中间件顺序；provider 测试覆盖现有顺序不影响 recovery、request log、CORS 或健康探针 skip。

## Application Error Annotation

- [x] 修改 `common/http/response.WriteError`，写响应前标注当前 span。
- [x] 确保 nil app error 仍映射为 `contracterrors.InternalError(nil)`。
- [x] 修改 `common/http/response.Fail`，复用 `WriteError` 或确保不会重复标注同一个应用错误。
- [x] 为应用错误 span attribute 写入 `error_code`。
- [x] 为应用错误 span attribute 写入 `http_status`。
- [x] 对 5xx 应用错误记录 sanitized error event，并设置 span status error。
- [x] 对普通 4xx 应用错误不记录 error event，不设置 span status error。
- [x] 修改或保留 `ValidationFailedWithErrors`，若标注 span，只写 `CodeValidationFailed` 和 400，不写字段明细。
- [x] 保持 `common/contract/response.Envelope` 输出结构不变。
- [x] 保持客户端可见 message、code 和 HTTP status 不变。

## Common Tests

- [x] 在 `common/http/middleware` 测试中新增或复用 OTel test span recorder helper。
- [x] 更新 panic recovery 测试，断言响应仍为 500 且包含现有失败信封。
- [x] 更新 panic recovery 测试，断言 panic 日志行为不变。
- [x] 更新 panic recovery 测试，断言 span status 为 error。
- [x] 更新 panic recovery 测试，断言 span 记录 error event，且 event/attribute 不包含 stacktrace 原文。
- [x] 更新 request logger 5xx 测试，断言 access log 仍为 error 级别。
- [x] 更新 request logger 5xx 测试，断言 span status 为 error。
- [x] 更新 request logger 4xx 测试，断言 access log 仍为 warn 级别。
- [x] 更新 request logger 4xx 测试，断言 span status 不为 error。
- [x] 更新 request logger success 测试，断言 2xx 不新增 error status。
- [x] 在 `common/http/response` 测试中覆盖 `WriteError` 5xx 应用错误的 span attribute、event 和 status。
- [x] 在 `common/http/response` 测试中覆盖 `WriteError` 4xx 应用错误不设置 error status。
- [x] 在 `common/http/response` 测试中覆盖 `Fail` unknown error 映射 internal error 后的 span 行为。
- [x] 在 `common/http/response` 测试中覆盖 `ValidationFailedWithErrors` 不泄漏字段明细或请求体到 span。

## User Service Tests

- [x] 更新 `user-service/internal/providers/routes_test.go` 或相关 provider 测试，覆盖实际 middleware 链下 panic span status。
- [x] 保留 panic recovery envelope、panic 日志和 request log 原有断言。
- [x] 增加或调整普通 4xx 路径测试，确认不会设置 span error status。
- [x] 增加或调整非 panic 5xx 路径测试，确认实际 Gin provider 链会标记 span error status。
- [x] 确认健康探针成功请求仍不产生业务 error status；失败探针如返回 5xx 应可标记错误。

## Documentation

- [x] 确认只新增私有 helper，未新增公共函数或新的 middleware。
- [x] 确认实现未改变架构规则，无需更新长期文档。
- [x] 不更新历史 change 文档，除非其中仍被当前实现引用且会直接误导本变更。
- [x] 确认文档不重新引入 OpenSpec/OPSX 流程或目录。

## Verification

- [x] 格式化修改过的 Go 文件：

```bash
gofmt -w <changed-go-files>
```

- [x] 运行 common HTTP middleware 和 response 测试：

```bash
cd common && go test ./http/middleware ./http/response
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

- [x] 按需运行架构边界检查：

```bash
make architecture-lint
```

- [x] 扫描 span 记录点，确认没有写入敏感字段：

```bash
rg -n "RecordError|SetStatus|SetAttributes|attribute\\." common user-service
```

- [x] 扫描确认没有新增 OpenSpec/OPSX 工件：

```bash
find . -maxdepth 3 \( -path './openspec' -o -path './docs/opsx' \) -print
```

- [x] 检查变更范围：

```bash
git diff -- common/http/middleware common/http/response docs/changes/record-http-errors-on-otel-spans user-service/internal/providers
```

## Guardrails

- [x] 不新增 `openspec/` 或 `docs/opsx/`。
- [x] 不引入 Sentry、Rollbar、Datadog 或其他错误上报后端。
- [x] 不实现 OTLP exporter、Collector 部署、dashboard 或告警。
- [x] 不新增 metrics 指标。
- [x] 不改变客户端 response envelope、错误码、message 或 HTTP status。
- [x] 不把普通 4xx 业务拒绝统一标记为系统异常。
- [x] 不在 span attribute、span event、日志新增字段或文档中记录 password、token、Authorization header、Cookie、原始请求体、DSN、SQL 参数或 stacktrace 原文。
- [x] 不修改数据库 schema、Ent generated code、Atlas migration 或 Redis key schema。
