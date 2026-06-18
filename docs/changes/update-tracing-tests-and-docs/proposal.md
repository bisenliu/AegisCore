# Update tracing tests and docs

## What

清理仓库中残留的自定义 trace-id 文档和测试契约，将当前行为统一描述为 OpenTelemetry tracing。

本变更关注文档与测试迁移：

- 更新 `docs/ARCHITECTURE.md`、`docs/DEVELOPMENT.md`、`docs/TESTING.md` 中关于 HTTP trace 传播、日志字段和测试断言的说明。
- 将测试说明改为：请求 `context.Context` 中存在有效 OTel span context，日志包含 OTel `trace_id` 和 `span_id`。
- 更新 e2e HTTP flow 测试，不再设置或断言 `X-Trace-ID` 请求头、响应头或私有 trace-id 兼容行为。
- 更新 OpenAPI 或运行时文档中涉及 trace header 的内容；如果当前 OpenAPI 没有 trace header 契约，则不新增。
- 明确当前阶段通过 OTel SDK provider 生成 `trace_id` / `span_id`，`observability.tracing.exporter: none` 不强制部署 Collector，也不提供 trace UI。
- 说明 `traceparent` / `tracestate` 是标准传播头，客户端不需要依赖服务返回 trace header。

## Why

用户服务已经迁移到 OTel Gin middleware 和 OTel span context 日志字段。`common/runtime/logger` 会从有效 OTel span context 自动派生 `trace_id` 和 `span_id`；用户服务不读取、回传或兼容 `X-Trace-ID`。

但测试文档和部分历史说明仍提到 trace-id 透传、生成、响应头和 e2e 断言。这会让后续实现者误以为当前 HTTP API 仍有私有 trace header 契约，进而在新测试、OpenAPI 或客户端文档里重新引入 `X-Trace-ID`。

本变更把当前文档和测试语义收敛到 OTel，降低后续改动中的误读风险，并补齐 `make verify` 前需要检查的文档、OpenAPI 和 Swagger 生成差异。

## Scope

包括：

- 更新长期规则文档：
  - `docs/ARCHITECTURE.md`
  - `docs/DEVELOPMENT.md`
  - `docs/TESTING.md`
- 更新当前产品或运行时说明中仍将 HTTP 请求描述为经过私有 trace-id middleware 的内容，例如 `docs/PRODUCT.md`。
- 扫描服务侧 OpenAPI 注解、生成产物和运行时文档，删除当前契约中的 `X-Trace-ID` 或 trace response header 描述。
- 如 OpenAPI 生成产物因注解调整发生变化，运行既有 OpenAPI 生成命令并提交生成结果。
- 更新 `user-service/tests/e2e` HTTP flow helper 和断言：
  - 不设置 `X-Trace-ID`。
  - 不断言 `X-Trace-ID` 响应头。
  - 如需要覆盖 tracing，改为断言请求链路中存在有效 OTel span context，或断言日志包含有效 `trace_id` 与 `span_id`。
- 更新其他当前测试中仍依赖私有 trace header 的断言。
- 对历史 change 文档只做必要的小范围修正：仅当它们仍被当前测试文档引用、或明显描述当前行为时更新，避免大规模重写历史记录。

不包括：

- 不新增功能代码。
- 不新增 `Trace-Id`、`X-Trace-ID` 或其他响应头替代品。
- 不恢复 `common/http/middleware/trace_id.go` 或自定义 trace-id context helper。
- 不引入 Collector、Jaeger、Tempo、dashboard、metrics exporter 或告警。
- 不改变 HTTP response envelope、业务错误码、日志等级策略或敏感字段规则。
- 不修改数据库 schema、Ent generated code、Atlas migration 或 Redis key schema。
- 不新增 `openspec/` 或 `docs/opsx/` 工件。

## Acceptance Criteria

- 当前长期文档不再声明 `X-Trace-ID` 是 HTTP trace header，也不要求服务返回 trace header。
- 文档明确 HTTP trace 传播使用 W3C `traceparent` / `tracestate`。
- 文档明确当前阶段使用 OTel SDK provider 生成标准 `trace_id` / `span_id`，`exporter: none` 不要求部署 Collector。
- 测试说明不再要求 trace-id 请求头、响应头、Gin context 私有 key 或自定义 trace-id 生成行为。
- e2e HTTP flow 测试不再设置或断言 `X-Trace-ID`。
- 如测试仍验证 tracing，应验证有效 OTel span context、有效日志 `trace_id` / `span_id`，或有效 `traceparent` 传播。
- OpenAPI 注解和生成产物不声明 `X-Trace-ID` 或新的 trace response header；如果本来没有该契约，则不新增。
- `make verify` 通过。
- 文档生成或 Swagger/OpenAPI 相关检查没有未提交差异。
- 没有新增 `openspec/` 或 `docs/opsx/` 目录。
