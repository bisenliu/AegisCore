## Why

`common/runtime` 和 `common/security` 中仍保留较多历史手写断言，导致测试失败信息不一致、样板判断较多，也容易在前置条件失败后继续执行并产生级联错误。当前 `docs/TESTING.md` 和主规格已经要求优先使用 `testify/require`，需要对共享 runtime 与安全原语测试做一次集中迁移，统一断言风格并明确允许保留的特殊失败控制流。

## What Changes

- 将 `common/runtime/**/*_test.go` 与 `common/security/**/*_test.go` 中可语义化表达的常见业务断言迁移为 `testify/require`。
- 在需要一次执行中收集多个独立指标字段失败的测试中，允许按 `docs/TESTING.md` 使用 `testify/assert`。
- 保留并记录并发协调、panic/recovery、benchmark 或测试框架边界中确实不适合语义化断言的 `t.Fatal`、`t.Fatalf`、`t.Error`、`t.Errorf` 或 `Fail*` 用法。
- 不修改 runtime primitive、安全原语、metrics 名称、tracing、logger、password KDF、JWT/auth 或 Casbin shared wrapper 的生产行为。
- 不新增旧断言风格兼容 helper，不用 `require.Fail`、`assert.Fail` 机械替换手写失败判断。

## Capabilities

### New Capabilities

- 无。

### Modified Capabilities

- `shared-platform-primitives`: 收紧 `common/runtime`、`common/security` 测试断言与失败处理的执行要求，确保共享 runtime primitive、安全原语和测试基础设施遵循统一断言规范。
- `runtime-observability`: 覆盖 metrics、tracing、logger、localcache、scheduler、workerpool 等 runtime observability 相关测试的断言迁移，但保持观测指标、日志和 tracing 运行时语义不变。
- `auth-session-management`: 覆盖 `common/security/auth` 与 `common/security/password` 测试断言迁移，但保持 JWT、token version、password KDF 和认证安全语义不变。
- `rbac-access-control`: 覆盖 `common/security/casbin` shared wrapper 测试断言迁移，但保持 Casbin 三元组授权、`ErrNotConfigured`、`ErrDenied` 和 RBAC 授权语义不变。

## Impact

- 影响代码范围：`common/runtime/**/*_test.go`、`common/security/**/*_test.go`。
- 影响依赖：测试文件将补充或调整 `github.com/stretchr/testify/require`，必要时使用 `github.com/stretchr/testify/assert`。
- 不影响外部 API、数据库 schema、OpenAPI、部署资产、metrics 名称、日志字段、tracing span、JWT claims、password KDF 参数或 Casbin authorizer 生产行为。
- 验证命令包括断言残留扫描、`go test ./common/runtime/... ./common/security/...` 和 `openspec validate standardize-common-runtime-security-assertions-no-compat`。
