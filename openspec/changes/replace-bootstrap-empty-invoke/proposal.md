## Why

`RuntimeModule` 当前使用两个空匿名 `fx.Invoke` 只为强制解析 `*http.Server` 与 `*PprofServer`，读者需要从参数类型和注释反推其目的。将该意图收敛到具名函数可以让 runtime lifecycle 注册点更明确，并降低后续维护时误删或误改的风险。

## What Changes

- 将 `user-service/internal/bootstrap/app.go` 中的空匿名 `fx.Invoke` 替换为具名 `registerRuntimeServers`。
- 保持 `*http.Server` 和 `*PprofServer` 仍由 Fx 解析，以触发构造函数注册 lifecycle hook。
- 更新或补充 bootstrap 测试，验证 runtime server 注册意图在源码和 Fx graph 中保持明确。
- 不改变 HTTP API、pprof 行为、Fx module 边界、配置项、数据库 schema 或部署资产。

## Capabilities

### New Capabilities

- 无。

### Modified Capabilities

- `runtime-observability`：明确正式 App 的 runtime server lifecycle 注册必须可由 composition root 清晰表达并可验证。

## Impact

- 受影响代码：`user-service/internal/bootstrap/app.go`、`user-service/internal/bootstrap/app_test.go`、`user-service/internal/bootstrap/validation_test.go`。
- API 影响：无 HTTP、OpenAPI 或外部契约变化。
- 数据影响：无 Ent schema、Atlas migration 或持久化数据变化。
- 运行时影响：保持 HTTP server 与可选 pprof server 的 lifecycle hook 注册语义不变。
- 验证影响：优先运行 `go test ./user-service/internal/bootstrap` 或对应包测试；合并前仍可运行 `make lint` 与 `make verify`。
