## Why

auth 相关历史测试仍混用 `t.Fatal`、`t.Errorf`、手写 if 断言和少量语义化断言，导致失败信息、前置条件失败处理和诊断风格在 credential、session、token、HTTP controller、Redis/PostgreSQL adapter 与 provider 测试之间不一致。需要一次性收敛到统一断言规范，降低后续认证会话行为演进时维护历史断言分支和兼容 helper 的成本。

本 change 明确不做旧断言兼容层：迁移后的 auth 测试应优先使用 `testify/require` 表达必须成立的前置条件和结果断言，仅在特殊测试控制流或诊断输出下保留 `t.Fatal` / `t.Error`。

## What Changes

- 将 `user-service/internal/features/auth/**/*_test.go` 和 `user-service/internal/providers/auth_test.go` 中的历史手写断言迁移为 `testify/require` 或必要时的 `assert` 语义化断言。
- 覆盖 auth application、domain、transport/http、infrastructure/postgres、infrastructure/redis、metrics 和 Fx/provider 测试中的 credential、session、token、password change、HTTP response、Redis/PostgreSQL adapter 断言。
- 在剩余 `t.Fatal` / `t.Error` 使用处只保留 `docs/TESTING.md` 允许的特殊测试控制流或诊断输出，并在 tasks 中列明剩余例外。
- 如 `user-service/go.mod` 尚未直接声明 `testify`，补充直接测试依赖，并通过 `go mod tidy` 确认依赖文件无无关漂移。
- 不修改认证、token version、refresh session、password KDF、Redis 或 PostgreSQL 生产行为。
- 不新增旧 auth HTTP 字段、旧错误码、旧 token 类型、旧状态兼容断言、机械 `Fail` / `Failf` 替换或旧手写断言兼容 helper。

## Capabilities

### New Capabilities

- 无。

### Modified Capabilities

- `auth-session-management`: 明确 auth 测试应使用语义化断言验证认证会话、token、强制改密、HTTP controller、Redis/PostgreSQL adapter 和 provider 行为，不通过旧手写断言兼容层隐藏失败信息。
- `shared-platform-primitives`: 明确共享测试基础设施支持服务测试直接使用 `testify/require` / `assert`，并约束保留 `t.Fatal` / `t.Error` 的例外范围。

## Impact

- 受影响测试代码限定在 `user-service/internal/features/auth/**/*_test.go` 和 `user-service/internal/providers/auth_test.go`。
- 可能影响依赖声明：`user-service/go.mod` / `user-service/go.sum` 可能需要直接声明 `github.com/stretchr/testify` 测试依赖。
- 不影响 HTTP API、OpenAPI、数据库 schema、Atlas migration、Redis key 语义、JWT claims、token version、refresh session 生命周期、password KDF 参数、配置项、部署资产或 Prometheus 指标运行时语义。
- 验证需要覆盖断言剩余例外扫描、`go test ./user-service/internal/features/auth/... ./user-service/internal/providers` 和 `openspec validate standardize-auth-test-assertions-no-compat`。
