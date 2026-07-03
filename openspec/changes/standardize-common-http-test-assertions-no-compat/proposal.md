## Why

`common/http` 历史测试中仍存在手写 `t.Fatal*`、`t.Error*` 或非语义化失败断言，导致 HTTP status、header、JSON envelope、OpenAPI 输出、pprof route 和 binding 错误的断言风格不一致，失败信息也不够稳定。当前 `docs/TESTING.md` 已明确优先使用 `testify/require` 和语义化断言，本变更将把 `common/http` 测试迁移到同一规范，并明确不保留旧 header、旧 envelope 或旧 CORS 行为兼容断言。

## What Changes

- 迁移 `common/http/**/*_test.go` 中的历史断言，优先使用 `require.JSONEq`、`require.Equal`、`require.Contains`、`require.NoError`、`require.ErrorIs` 等语义化方法。
- 对需要在一次测试执行中收集多个独立响应字段失败的场景，允许使用 `testify/assert`；初始化失败、前置条件失败和后续检查依赖当前结果的场景仍使用 `require`。
- 清理机械式 `Fail` / `Failf` 替换风险，避免用非语义化失败方法替代可表达的 HTTP、JSON、header、OpenAPI、pprof 或 binding 断言。
- 列明仍需保留的 `t.Fatal*`、`t.Error*`、`Fail*` 特殊例外，且仅允许符合 `docs/TESTING.md` 特殊例外规则的测试控制流或诊断场景。
- 不修改 `common/http` 生产行为、HTTP response envelope、middleware 语义、OpenAPI helper 行为或 pprof route 行为。
- 不迁移 `user-service` HTTP transport 测试，不新增旧断言风格兼容 helper，也不双写新旧行为断言。

## Capabilities

### New Capabilities

- 无。

### Modified Capabilities

- `shared-platform-primitives`: 补充 `common/http` 测试断言规范，要求共享 HTTP helper 的测试使用语义化 `testify` 断言验证绑定、middleware、response、OpenAPI 和 pprof 等基础行为。
- `runtime-observability`: 补充 OpenAPI、pprof 和 HTTP middleware 可观测性相关测试的断言规范，确保测试验证当前稳定输出而不是旧兼容行为。

## Impact

- 影响测试代码：`common/http/**/*_test.go`，覆盖 `binding`、`middleware`、`openapi`、`pprof`、`response` 等包。
- 影响测试依赖：继续使用 `github.com/stretchr/testify/require`，仅在需要收集多个独立字段失败时使用 `github.com/stretchr/testify/assert`。
- 不影响生产 API、HTTP response envelope、middleware 运行时语义、OpenAPI helper 输出、pprof route 行为、数据库 schema、部署资产或用户服务 HTTP transport 测试。
- 验收验证包括 `rg "t\\.Fatalf|t\\.Fatal\\(|t\\.Errorf|t\\.Error\\(|Failf?\\(" common/http --glob '*_test.go'`、`rg "github.com/stretchr/testify/(require|assert)" common/http --glob '*_test.go'`、`go test ./common/http/...` 和 `openspec validate standardize-common-http-test-assertions-no-compat`。
