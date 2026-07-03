## ADDED Requirements

### Requirement: common HTTP 测试断言规范

系统 MUST 在 `common/http/**/*_test.go` 中使用语义化 `testify` 断言验证共享 HTTP helper、binding、middleware、response、OpenAPI 和 pprof 相关行为。初始化失败、前置条件失败以及后续检查依赖当前结果的场景 MUST 使用 `testify/require`；只有需要在单次测试执行中收集多个相互独立响应字段失败时，系统 MAY 使用 `testify/assert`。

#### Scenario: 验证 HTTP status 和 header

- **WHEN** `common/http` 测试验证 HTTP 响应状态码、响应 header 或中间件写入结果
- **THEN** 测试 MUST 优先使用 `require.Equal`、`require.Contains`、`require.NotEmpty` 或等价语义化断言
- **AND** 测试 MUST NOT 将可语义化表达的检查迁移为 `require.Fail*`、`assert.Fail*`、`t.Fatal*` 或 `t.Error*`

#### Scenario: 验证 JSON envelope

- **WHEN** `common/http` 测试验证 JSON response envelope、错误详情或分页结构
- **THEN** 测试 MUST 优先使用 `require.JSONEq` 或在 `require.NoError` 解析后使用语义化字段断言
- **AND** 测试 MUST 验证当前稳定 envelope 结构，不得新增旧 envelope 兼容断言或双写断言

#### Scenario: 验证 binding 错误

- **WHEN** `common/http/binding` 测试验证请求绑定、校验失败或错误响应
- **THEN** 测试 MUST 使用 `require.NoError`、`require.Error`、`require.ErrorIs`、`require.Equal`、`require.Contains` 或等价语义化断言表达预期
- **AND** 只有无法通过现有语义化断言清晰表达的自定义测试控制流或特殊诊断输出 MAY 保留直接 `t.Fatal*`、`t.Error*` 或 `Fail*`

#### Scenario: 验证迁移完成度

- **WHEN** 断言迁移完成
- **THEN** `rg "t\\.Fatalf|t\\.Fatal\\(|t\\.Errorf|t\\.Error\\(|Failf?\\(" common/http --glob '*_test.go'` 的剩余命中 MUST 均符合 `docs/TESTING.md` 特殊例外规则
- **AND** `rg "github.com/stretchr/testify/(require|assert)" common/http --glob '*_test.go'` MUST 能定位到迁移后的实际使用点
