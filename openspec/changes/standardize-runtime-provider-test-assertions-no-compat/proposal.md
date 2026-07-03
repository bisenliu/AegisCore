## Why

`user-service/internal/router`、`providers` 和 `bootstrap` 的历史测试仍大量使用 `t.Fatal`、`t.Fatalf`、手写 if 断言和少量泛化布尔断言，导致 HTTP runtime、Fx provider、health、metrics、OpenAPI、pprof 与 server lifecycle 测试的失败信息和前置条件处理不一致。当前 `docs/TESTING.md` 与 `delivery-operations` 主规格已经要求优先使用语义化 `testify/require`，需要把这些运行时装配边界测试集中迁移到统一断言规范。

本 change 不引入旧断言兼容层，也不修改生产路由、provider 装配、HTTP server lifecycle、OpenAPI 生成物或 metrics 输出格式。

## What Changes

- 将 `user-service/internal/router/**/*_test.go`、`user-service/internal/providers/**/*_test.go` 和 `user-service/internal/bootstrap/**/*_test.go` 中可语义化表达的历史断言迁移为 `testify/require`。
- 对多个互相独立的 route、provider 输出、metrics family、日志字段或 health check 结果检查，按 `docs/TESTING.md` 使用 `testify/assert` 收集独立失败。
- 使用 `require.Len`、`require.Greater`、`require.ErrorContains`、`require.ElementsMatch`、`require.JSONEq`、`require.Regexp`、`require.WithinDuration`、`require.Panics` 等更具体断言替代可覆盖的 `True` / `False` 或手写 if。
- 保留并记录确实属于并发协调、channel 控制流、goroutine handoff 或测试辅助工具边界的直接 `testing.T` 失败调用例外。
- 不修改生产路由注册、health、metrics、OpenAPI、pprof、Fx provider、bootstrap validation 或 HTTP server lifecycle 行为。
- 不新增旧 metrics path、旧 pprof path、旧 route alias 兼容断言，不新增机械 `Fail` / `Failf` / `FailNow` / `FailNowf` 替换或旧手写断言兼容 helper。

## Capabilities

### New Capabilities

- 无。

### Modified Capabilities

- `runtime-observability`: 明确 router、health、metrics、OpenAPI、pprof、Gin middleware 和 runtime endpoint 相关测试应使用语义化断言，同时保持观测路由、指标、日志与 tracing 运行时语义不变。
- `delivery-operations`: 明确 user-service bootstrap validation、HTTP server lifecycle 和 Fx provider 装配测试应遵循统一 Go 测试断言规范，并通过残留扫描记录例外。
- `shared-platform-primitives`: 明确服务装配边界测试直接使用 `testify/require` / `assert`，不得为迁移历史断言新增共享兼容 helper 或生产测试专用 API。

## Impact

- 影响测试代码范围限定为 `user-service/internal/router/**/*_test.go`、`user-service/internal/providers/**/*_test.go` 和 `user-service/internal/bootstrap/**/*_test.go`。
- `user-service/go.mod` 已直接声明 `github.com/stretchr/testify`，预计无需新增依赖；若 tidy 产生与本 change 无关的漂移不得提交。
- 不影响 HTTP API、OpenAPI 生成物、数据库 schema、Atlas migration、Prometheus metric family、label key/value、日志字段、tracing span、Gin middleware 生产行为、Fx provider 装配语义或 HTTP server lifecycle。
- 验证命令包括断言残留扫描、`rg "github.com/stretchr/testify/(require|assert)" ...`、`go test ./user-service/internal/router ./user-service/internal/providers ./user-service/internal/bootstrap` 和 `openspec validate standardize-runtime-provider-test-assertions-no-compat`。
