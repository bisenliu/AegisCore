# Tasks

## 1. Preparation

- [x] 1.1 阅读 `AGENTS.md`、`docs/ARCHITECTURE.md`、本 change 的 `proposal.md` 与 `design.md`。
- [x] 1.2 梳理 `common/runtime/observability/metrics` 现有 provider、label helper、README 和测试。
- [x] 1.3 梳理 `common/http/middleware` 现有 logging、recovery、span error middleware 的风格与测试写法。
- [x] 1.4 梳理 `user-service/internal/providers/gin.go` middleware 顺序和已有 tracing/logging tests。
- [x] 1.5 确认本 change 使用 `docs/changes/add-http-server-metrics/`，不新增 `openspec/` 或 `docs/opsx/`。

## 2. Common HTTP Metrics Middleware

- [x] 2.1 在 `common/http/middleware` 新增 HTTP server metrics middleware。
- [x] 2.2 定义 middleware options，至少支持 metrics provider、route fallback 和完成后 skip 策略。
- [x] 2.3 metrics provider nil 或 disabled 时保持零副作用。
- [x] 2.4 注册 `http_server_requests_total` counter。
- [x] 2.5 注册 `http_server_request_duration_seconds` histogram。
- [x] 2.6 注册 `http_server_in_flight_requests` gauge。
- [x] 2.7 使用 `metrics.Provider.Register` 注册 collector，重复注册不导致启动失败。
- [x] 2.8 保持 metric name 不包含服务名或环境名；服务名和环境由 provider const labels 注入。

## 3. Label Contract

- [x] 3.1 method label 使用标准 HTTP method，空值或异常值使用稳定 fallback。
- [x] 3.2 route label 优先使用 Gin route template `c.FullPath()`。
- [x] 3.3 未匹配路由使用稳定 fallback label，例如 `__unmatched__`。
- [x] 3.4 status label 使用 `status_class` 或 `status_code`，避免无限 cardinality。
- [x] 3.5 如实现 `code` label，只读取稳定低基数应用错误码，不解析 response body 或原始错误。
- [x] 3.6 确认 label 不包含 user/session/token/cursor/raw URL/IP/User-Agent/trace/span/SQL/Redis key。

## 4. In-Flight Semantics

- [x] 4.1 在请求进入 middleware 时增加 in-flight gauge。
- [x] 4.2 在请求结束、panic recover 或 handler 返回后减少 in-flight gauge。
- [x] 4.3 通过测试确认 route label 不会导致 in-flight gauge 泄漏。
- [x] 4.4 如果全局 middleware 无法在请求开始时获得 route template，使用稳定 fallback 记录 in-flight，并在设计注释中说明。

## 5. Runtime Endpoint Filtering

- [x] 5.1 支持完成后 skip 策略，使调用方能按最终 status 过滤。
- [x] 5.2 用户服务成功健康探针请求默认不记录 HTTP server RED 指标。
- [x] 5.3 用户服务成功 metrics scrape 默认不记录 HTTP server RED 指标。
- [x] 5.4 失败健康探针或失败 metrics scrape 如未过滤，应能按稳定 route/fallback label 记录。
- [x] 5.5 不改变 request logger 的 skip 行为。

## 6. User-Service Wiring

- [x] 6.1 在 `user-service/internal/providers/gin.go` 的 `GinParams` 注入 metrics provider。
- [x] 6.2 在 Gin middleware 链中接入 HTTP server metrics middleware。
- [x] 6.3 保持 OTel tracing、span rename、panic recovery、request logger 和 CORS 的既有语义。
- [x] 6.4 使用 `router.IsLowNoiseRuntimePath` 和 metrics config 构造成功 runtime endpoint skip 逻辑。
- [x] 6.5 确认 metrics disabled 时用户服务启动和请求行为不变。

## 7. Common Tests

- [x] 7.1 测试 disabled provider 时 middleware 零副作用。
- [x] 7.2 测试业务请求增加 `http_server_requests_total`。
- [x] 7.3 测试业务请求写入 `http_server_request_duration_seconds` histogram。
- [x] 7.4 测试 5xx 请求进入错误/状态维度。
- [x] 7.5 测试 route label 使用 Gin route template。
- [x] 7.6 测试未匹配路由使用稳定 fallback，不出现 raw path。
- [x] 7.7 测试 in-flight gauge 在阻塞 handler 中增加，完成后归零。
- [x] 7.8 测试完成后 skip 策略过滤成功 runtime endpoint。
- [x] 7.9 测试失败 runtime endpoint 可按预期记录或单独标记。
- [x] 7.10 测试重复构造 middleware 不因 collector already registered 失败。

## 8. User-Service Tests

- [x] 8.1 更新 `user-service/internal/providers/gin_test.go`，覆盖 metrics middleware 接入。
- [x] 8.2 测试 metrics enabled 时业务 route 请求可在 scrape 中看到 HTTP server metrics。
- [x] 8.3 测试成功 `/livez`、`/readyz`、`/startupz` 不污染业务 HTTP metrics。
- [x] 8.4 测试成功 `/metrics` scrape 不污染业务 HTTP metrics。
- [x] 8.5 测试未匹配用户服务路由使用稳定 fallback label。
- [x] 8.6 测试 panic 或 5xx 请求仍被记录为 server error status class。
- [x] 8.7 回归测试 request logger 行为不变。

## 9. Documentation

- [x] 9.1 更新 `common/runtime/observability/metrics/README.md`，说明 HTTP server RED 指标和 label 约束。
- [x] 9.2 更新 `docs/ARCHITECTURE.md`，说明用户服务 Gin provider 接入 HTTP metrics middleware。
- [x] 9.3 明确健康探针和 metrics scrape 成功请求的过滤策略。
- [x] 9.4 明确不得在 metrics label 中写入高基数或敏感值。
- [x] 9.5 不更新 OpenAPI 文档。

## 10. Verification

- [x] 10.1 格式化修改过的 Go 文件：

```bash
gofmt -w <changed-go-files>
```

- [x] 10.2 运行 common 测试：

```bash
make test-common
```

- [x] 10.3 运行用户服务测试：

```bash
make test-user-service
```

- [x] 10.4 如改动影响边界规则，运行架构检查：

```bash
make architecture-lint
```

- [x] 10.5 扫描确认没有新增 OpenSpec/OPSX 工件：

```bash
find . -maxdepth 3 \( -path './openspec' -o -path './docs/opsx' \) -print
```

- [x] 10.6 检查最终 diff，确认未修改 Ent generated code、Atlas migration、RBAC baseline、业务 API response 或部署 chart。

## 11. Guardrails

- [x] 11.1 不新增 tracing 逻辑。
- [x] 11.2 不新增业务指标或依赖指标。
- [x] 11.3 不在 label 中放用户、会话、token、cursor、raw URL、IP 或 User-Agent。
- [x] 11.4 不改变 request logger 行为。
- [x] 11.5 不改变健康探针、metrics endpoint、JWT、RBAC 或 OpenAPI 行为。
- [x] 11.6 不新增 `openspec/` 或 `docs/opsx/`。
