## Why

user 与 shared identity 相关历史测试仍混用 `t.Fatal`、`t.Errorf`、手写 if 断言和少量语义化断言，导致用户资料、HTTP response、PostgreSQL adapter、domain user 和 identity 状态判断测试的失败信息与前置条件处理不一致。需要集中迁移到统一断言规范，避免后续维护用户状态、软删除和分页行为时继续扩散旧断言风格。

本 change 明确不做旧断言兼容层：迁移后的 user 与 shared identity 测试应优先使用 `testify/require` 表达必须成立的前置条件和结果断言，仅在多个独立 response 字段需要同时收集失败时按 `docs/TESTING.md` 使用 `testify/assert`。

## What Changes

- 将 `user-service/internal/features/user/**/*_test.go` 和 `user-service/internal/shared/identity/**/*_test.go` 中的历史手写断言迁移为 `testify/require` 或必要时的 `assert` 语义化断言。
- 覆盖 user profile、HTTP controller/response、PostgreSQL adapter、domain user 和 shared identity 状态判断测试中的错误、相等性、集合、分页、软删除和状态语义断言。
- 在剩余 `t.Fatal` / `t.Error` / `Fail` 使用处只保留 `docs/TESTING.md` 允许的特殊测试控制流或诊断输出，并在 tasks 中列明剩余例外。
- 如 `user-service/go.mod` 尚未直接声明 `testify`，补充直接测试依赖，并通过 `go mod tidy` 确认依赖文件无无关漂移。
- 不修改用户资料、identity 状态、软删除、分页、HTTP envelope、数据库 schema 或 HTTP API 行为。
- 不新增旧 user 字段、旧状态兼容断言、旧响应 envelope 断言、机械 `Fail` / `Failf` 替换或旧手写断言兼容 helper。

## Capabilities

### New Capabilities

- 无。

### Modified Capabilities

- `user-identity-management`: 明确 user 与 shared identity 测试应使用语义化断言验证用户资料、状态判断、软删除、分页和 HTTP response 行为，不通过旧手写断言兼容层隐藏失败信息。
- `shared-platform-primitives`: 明确共享测试规范支持服务内 user/shared identity 测试直接使用 `testify/require` / `assert`，并约束保留 `t.Fatal` / `t.Error` / `Fail` 的例外范围。

## Impact

- 受影响测试代码限定在 `user-service/internal/features/user/**/*_test.go` 和 `user-service/internal/shared/identity/**/*_test.go`。
- 可能影响依赖声明：`user-service/go.mod` / `user-service/go.sum` 可能需要直接声明 `github.com/stretchr/testify` 测试依赖。
- 不影响 HTTP API、OpenAPI、数据库 schema、Atlas migration、Redis key 语义、用户状态机、软删除语义、分页契约、HTTP response envelope、配置项、部署资产或 Prometheus 指标运行时语义。
- 验证需要覆盖断言剩余例外扫描、`testify/require` 或 `assert` 使用点扫描、`go test ./user-service/internal/features/user/... ./user-service/internal/shared/identity/...` 和 `openspec validate standardize-user-identity-test-assertions-no-compat`。
