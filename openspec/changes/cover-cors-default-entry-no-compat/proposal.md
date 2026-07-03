## Why

`common/http/middleware.CORS()` 是共享 CORS 默认策略的公开入口，但当前缺少直接测试覆盖，重构时容易让默认 allow origin、method、header、预检短路或业务 handler 继续执行语义漂移。现在补齐默认入口测试，可以固定当前稳定策略，并避免旧 header、旧 origin 反射默认值或旧安全兼容开关回流。

## What Changes

- 为 `common/http/middleware.CORS()` 增加直接单元测试，确认其默认策略与 `CORSWithOptions(defaultCORSOptions)` 一致。
- 测试覆盖默认 `Access-Control-Allow-Origin`、`Access-Control-Allow-Methods`、`Access-Control-Allow-Headers`、`OPTIONS` 预检 `204 No Content` 直接返回，以及普通请求继续进入业务 handler。
- 保留并只补齐当前 `CORSWithOptions` 自定义策略的稳定字段测试缺口，不扩大 CORS 配置 API。
- 新增测试遵循当前测试断言规范，优先使用语义化 `require`，不新增旧式手写失败判断、机械 `Fail`/`Failf` 替换或兼容断言 helper。
- 不修改服务运行时是否挂载 CORS middleware 的策略，不修改 auth、request ID、logging、metrics 或 recovery middleware。

## Capabilities

### New Capabilities

无。

### Modified Capabilities

- `shared-platform-primitives`: 补充共享 CORS 默认入口的稳定测试覆盖约束，锁定当前默认 CORS 响应头、预检短路和普通请求传递行为。
- `runtime-observability`: 补充 `common/http/middleware` 中与观测链路同域维护的 CORS middleware 回归测试约束，确保测试补齐不改变既有 middleware 组合和观测行为。

## Impact

- 影响代码路径：`common/http/middleware/cors.go` 及同包测试。
- 不影响 HTTP API 路由、OpenAPI 生成物、数据库 schema、部署资产、外部依赖或服务运行时 CORS 挂载策略。
- 验证重点为 `go test -cover ./common/http/middleware`、`go tool cover -func` 中 `CORS`、`CORSWithOptions`、`normalizeCORSOptions` 覆盖情况，以及 `openspec validate cover-cors-default-entry-no-compat`。
