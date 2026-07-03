## MODIFIED Requirements

### Requirement: 测试断言与失败处理风格

测试代码中的断言与失败处理 MUST 优先使用 `testify/require`，以提升测试可读性、减少手写失败判断样板代码、统一阻塞式失败处理方式，并提供一致、清晰的失败诊断信息。`common/contract`、`common/validation` 和 `common/testing` 的历史测试迁移或新增测试 MUST 将常见错误、相等性、集合、fixture 和容器测试断言表达为语义化 `require` 方法，除非该失败调用属于测试控制流、特殊诊断输出或测试框架边界。

#### Scenario: 常见阻塞式断言

- **WHEN** 测试需要检查错误返回值、前置条件、对象相等性、布尔条件、集合长度、错误类型或状态
- **THEN** 测试 MUST 使用语义化的 `require` 断言方法，例如 `require.NoError`、`require.Error`、`require.ErrorIs`、`require.Equal`、`require.True`、`require.False`、`require.Len`、`require.NotNil`
- **AND** 测试 SHOULD NOT 直接使用 `t.Fatal`、`t.Fatalf`、`t.Error` 或 `t.Errorf` 表达这些常见断言

#### Scenario: 共享基础包历史测试迁移

- **WHEN** `common/contract`、`common/validation` 或 `common/testing` 的 `_test.go` 文件迁移历史断言或新增常见阻塞式断言
- **THEN** 测试 MUST 优先使用 `require.NoError`、`require.ErrorIs`、`require.Equal`、`require.Len`、`require.Contains`、`require.NotNil` 或等价语义化 `require` 方法
- **AND** 目标模块 MUST 直接声明实际使用的 `github.com/stretchr/testify` 测试依赖
- **AND** 迁移 MUST NOT 改变对应生产包的公开 API、错误语义或运行时行为

#### Scenario: 避免机械 Fail 替换

- **WHEN** 测试迁移手写失败判断或新增失败处理
- **THEN** 测试 SHOULD NOT 将 `t.Fatal`、`t.Fatalf`、`t.Error` 或 `t.Errorf` 机械替换为 `require.FailNow`、`require.FailNowf`、`require.Fail`、`require.Failf`、`assert.Fail` 或 `assert.Failf`
- **AND** 当存在明确的语义化 `require` 或 `assert` 断言方法时，测试 MUST 优先使用对应断言方法

#### Scenario: 非阻塞式独立断言

- **WHEN** 单个测试用例需要在一次执行中继续收集多个相互独立的断言失败
- **THEN** 测试 MAY 使用 `testify/assert` 进行非阻塞式断言
- **AND** 初始化失败、前置条件失败或后续检查依赖当前结果的场景 MUST 使用 `testify/require` 立即终止当前测试

#### Scenario: 保留特殊失败控制流

- **WHEN** 测试存在无法通过现有 `require` 或 `assert` 语义化断言清晰表达的自定义测试控制流、特殊诊断输出，或测试辅助工具不适合依赖 `testify`
- **THEN** 测试 MAY 直接使用 `t.Fatal`、`t.Fatalf`、`t.Error`、`t.Errorf`、`require.FailNowf` 或 `assert.Failf`
- **AND** 此类用法 SHOULD 让保留原因在代码上下文中保持清晰
- **AND** 在 `common/contract`、`common/validation` 或 `common/testing` 迁移完成时，剩余命中 MUST 在实施任务记录中列明并确认符合例外规则
