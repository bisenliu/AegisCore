## ADDED Requirements

### Requirement: 服务测试断言依赖与例外规范

服务测试 MUST 可以直接使用标准 `testify/require` 与 `testify/assert` 断言库表达常见错误、对象、布尔、集合、字符串和诊断预期。系统 MUST NOT 为迁移 user 与 shared identity 历史测试断言新增跨服务兼容 helper、机械失败包装器或隐藏标准断言语义的共享抽象。

#### Scenario: 服务模块声明直接测试依赖

- **WHEN** 服务模块的测试代码直接导入 `github.com/stretchr/testify/require` 或 `github.com/stretchr/testify/assert`
- **THEN** 该 Go module MUST 在自身 `go.mod` 中直接声明 `github.com/stretchr/testify`
- **AND** `go mod tidy` 后依赖文件 MUST NOT 出现与本次测试断言迁移无关的漂移

#### Scenario: 优先使用语义化断言

- **WHEN** 测试需要验证错误、对象和值、布尔状态、集合长度、字符串内容、类型、nil 状态、HTTP response 字段或 pagination 字段
- **THEN** 测试 MUST 优先使用 `require.NoError`、`require.Error`、`require.ErrorIs`、`require.Equal`、`require.NotEqual`、`require.Nil`、`require.NotNil`、`require.True`、`require.False`、`require.Len`、`require.Empty`、`require.NotEmpty`、`require.Contains` 或等价语义化断言
- **AND** 测试 MUST NOT 将普通手写失败判断机械替换为 `require.Fail`、`require.Failf`、`assert.Fail` 或 `assert.Failf`

#### Scenario: 多字段响应诊断可以使用 assert

- **WHEN** 单个 HTTP response、pagination 或 DTO 测试需要同时收集多个互不依赖字段的失败信息
- **THEN** 测试 MAY 使用 `testify/assert` 验证这些独立字段
- **AND** 任何会影响后续解码、类型断言或字段访问安全性的前置条件 MUST 使用 `require`

#### Scenario: 保留直接 testing.T 失败调用的例外

- **WHEN** 测试保留 `t.Fatal`、`t.Fatalf`、`t.Error` 或 `t.Errorf`
- **THEN** 该调用 MUST 用于无法通过现有语义化断言清晰表达的自定义测试控制流、特殊诊断输出或不适合依赖 `testify` 的测试辅助工具
- **AND** 普通前置条件失败、错误返回值、相等性、包含关系、长度、空值或布尔状态断言 MUST 使用 `require` 或必要时 `assert`
