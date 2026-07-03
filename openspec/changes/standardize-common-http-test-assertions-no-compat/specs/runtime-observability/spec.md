## ADDED Requirements

### Requirement: common HTTP 可观测性测试断言规范

系统 MUST 在 `common/http` 的 OpenAPI、pprof 和 middleware 可观测性相关测试中使用语义化 `testify` 断言验证当前稳定输出。测试 MUST 聚焦当前 OpenAPI 输出、pprof route 和 middleware 可观测性行为，不得新增旧 header、旧 CORS、旧 OpenAPI 输出或旧 pprof route 兼容断言。

#### Scenario: 验证 OpenAPI 输出

- **WHEN** `common/http/openapi` 测试验证 OpenAPI JSON、YAML、HTML 或转换结果
- **THEN** 测试 MUST 优先使用 `require.NoError`、`require.JSONEq`、`require.Equal`、`require.Contains` 或结构化解析后的语义化断言
- **AND** 测试 MUST NOT 通过 `Fail*`、`t.Fatal*` 或 `t.Error*` 表达可语义化验证的输出差异

#### Scenario: 验证 pprof route

- **WHEN** `common/http/pprof` 测试验证 pprof route 注册、HTTP status 或响应 header
- **THEN** 测试 MUST 使用 `require` 或必要的 `assert` 语义化断言表达当前 route 行为
- **AND** 测试 MUST NOT 新增旧 pprof route 兼容断言或测试专用生产分支

#### Scenario: 验证 middleware 可观测性输出

- **WHEN** `common/http/middleware` 测试验证日志、metrics、request ID、recovery、CORS 或 tracing 相关 HTTP 输出
- **THEN** 前置条件和依赖性检查 MUST 使用 `require`
- **AND** 只有多个相互独立的响应字段需要一次性收集失败时 MAY 使用 `assert`
- **AND** 测试 MUST 验证当前稳定行为，不得双写旧 header、旧 CORS 或旧 envelope 断言

#### Scenario: 验证 common HTTP 测试通过

- **WHEN** OpenAPI、pprof 和 middleware 断言迁移完成
- **THEN** `go test ./common/http/...` MUST 通过
- **AND** `openspec validate standardize-common-http-test-assertions-no-compat` MUST 通过
