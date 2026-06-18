# Tasks

## 1. Baseline Review

- [x] 阅读 `AGENTS.md` 和 `docs/ARCHITECTURE.md`，确认本仓库不新增 `openspec/` 或 `docs/opsx/`。
- [x] 阅读本 change 的 `proposal.md` 和 `design.md`，确认本次只迁移文档和测试契约，不新增 tracing 功能代码。
- [x] 扫描当前 trace 相关残留：
  ```bash
  rg -n "X-Trace-ID|HeaderTraceID|TraceID\\(|WithTraceID|TraceIDFromContext|trace-id|trace_id|span_id|traceparent|tracestate" common user-service docs/ARCHITECTURE.md docs/DEVELOPMENT.md docs/TESTING.md docs/PRODUCT.md
  ```

## 2. Update Current Documentation

- [x] 更新 `docs/TESTING.md` 的 Middleware 验证说明，删除 trace-id 透传、生成、写入 Gin context/Go context/响应头表述。
- [x] 更新 `docs/TESTING.md` 的 Logging 验证说明，改为 OTel `trace_id` / `span_id` 字段和无有效 span context 不伪造字段。
- [x] 更新 `docs/TESTING.md` 的 e2e HTTP flow 说明，不再列出 trace-id 响应头；如提及 tracing，改为 OTel span context 或日志字段。
- [x] 检查 `docs/ARCHITECTURE.md`，确认 HTTP trace 传播是 `traceparent` / `tracestate`，且未声明服务返回 trace header。
- [x] 检查 `docs/DEVELOPMENT.md`，确认本地 tracing 说明不依赖 `X-Trace-ID`，并明确 `exporter: none` 不强制 Collector。
- [x] 更新 `docs/PRODUCT.md` 中 HTTP 请求流程，把 trace-id middleware 改为 OTel tracing middleware。
- [x] 如发现当前运行时文档或 OpenAPI 说明仍声明 `X-Trace-ID`，删除或迁移为 W3C Trace Context 说明；如果 OpenAPI 没有 trace header 契约，不新增。

## 3. Update Tests

- [x] 扫描 e2e 测试：
  ```bash
  rg -n "X-Trace-ID|HeaderTraceID|TraceID\\(|trace-id|trace_id|span_id|traceparent|tracestate" user-service/tests/e2e
  ```
- [x] 删除 e2e request helper 中对 `X-Trace-ID` 的自动设置。
- [x] 删除 e2e response helper 或 flow 断言中对 `X-Trace-ID` 响应头的依赖。
- [x] 保留 e2e 对登录、受保护用户 API、强制改密、旧密码拒绝、登出当前设备、refresh session 失效和统一 response envelope 的覆盖。
- [x] 如 e2e 仍需要 tracing 覆盖，改为断言有效 OTel span context、日志中有效 `trace_id` / `span_id`，或合法 `traceparent` 的传播效果。
- [x] 扫描 provider、middleware 和 logger 测试：
  ```bash
  rg -n "X-Trace-ID|HeaderTraceID|WithTraceID|TraceIDFromContext|TraceID\\(" common/http common/runtime/logger user-service/internal/providers
  ```
- [x] 将仍依赖自定义 trace-id helper 的测试改为 OTel span context 测试 helper。
- [x] 确认无有效 span context 的日志测试不要求空字符串 `trace_id` 或私有 fallback trace ID。

## 4. OpenAPI And Generated Artifacts

- [x] 扫描 OpenAPI 注解和生成产物：
  ```bash
  rg -n "X-Trace-ID|Trace-Id|trace header|trace-id|traceparent|tracestate" user-service/internal user-service/docs docs
  ```
- [x] 如果源码注解删除或调整 trace header 说明，运行：
  ```bash
  make openapi-generate
  ```
- [x] 检查 `user-service/docs/` 生成产物；如果 OpenAPI 原本没有 trace header 契约，确认未新增 `traceparent` 或其他 trace header 参数。

## 5. Verification

- [x] 运行用户服务测试：
  ```bash
  make test-user-service
  ```
- [x] 运行共享模块测试：
  ```bash
  make test-common
  ```
- [x] 运行完整验证：
  ```bash
  make verify
  ```
- [x] 检查当前文档、生产代码和测试不再依赖 `X-Trace-ID`：
  ```bash
  rg -n "X-Trace-ID|HeaderTraceID" docs/ARCHITECTURE.md docs/DEVELOPMENT.md docs/TESTING.md docs/PRODUCT.md common user-service
  ```
- [x] 检查 `docs/TESTING.md` 和 e2e 测试不再描述 trace-id 响应头或自定义 trace-id 生成：
  ```bash
  rg -n "trace-id 响应头|trace-id 透传|trace-id 生成|TraceID\\(" docs/TESTING.md user-service/tests/e2e
  ```
- [x] 检查 OpenAPI 或 Swagger 相关文件没有未提交的生成差异。
- [x] 扫描确认没有新增 OpenSpec/OPSX 工件：
  ```bash
  find . -maxdepth 3 \( -path './openspec' -o -path './docs/opsx' \) -print
  ```

## 6. Guardrails

- [x] 不新增功能代码。
- [x] 不新增 `Trace-Id`、`X-Trace-ID` 或其他响应头替代品。
- [x] 不引入 Collector、dashboard、metrics exporter 或告警。
- [x] 不修改 Ent generated code、Atlas migration、数据库 schema 或 Redis key schema。
- [x] 不新增 `openspec/` 或 `docs/opsx/` 工件。
