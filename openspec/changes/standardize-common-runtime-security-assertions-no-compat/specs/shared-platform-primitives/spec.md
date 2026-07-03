## ADDED Requirements

### Requirement: common 测试断言统一迁移

`common/runtime` 和 `common/security` 的测试代码 MUST 优先使用 `testify/require` 表达可语义化的常见断言，包括错误返回、错误类型、对象和值相等性、nil、布尔条件、集合长度、字符串包含关系和状态检查。测试代码 MUST NOT 将历史手写失败判断机械替换为 `require.Fail`、`require.Failf`、`assert.Fail` 或 `assert.Failf`；当存在明确语义化断言方法时，MUST 使用对应的 `require` 或 `assert` 方法。

#### Scenario: 迁移常见断言

- **WHEN** `common/runtime` 或 `common/security` 的 `_test.go` 需要检查错误、对象状态、布尔条件、集合或字符串结果
- **THEN** 测试 MUST 使用 `require.NoError`、`require.Error`、`require.ErrorIs`、`require.Equal`、`require.True`、`require.False`、`require.Len`、`require.NotNil` 或等价语义化断言
- **AND** 测试 SHOULD NOT 使用 `t.Fatal`、`t.Fatalf`、`t.Error` 或 `t.Errorf` 表达这些常见断言

#### Scenario: 独立字段聚合诊断

- **WHEN** 单个测试需要在一次执行中收集多个相互独立字段、指标或统计值的失败信息
- **THEN** 测试 MAY 使用 `testify/assert` 进行非阻塞式断言
- **AND** 初始化失败、前置条件失败或后续检查依赖当前结果的场景 MUST 使用 `testify/require`

#### Scenario: 避免 Fail helper 机械替换

- **WHEN** 迁移历史手写失败判断
- **THEN** 测试 MUST NOT 使用 `require.Fail`、`require.Failf`、`require.FailNow`、`require.FailNowf`、`assert.Fail` 或 `assert.Failf` 替代可语义化表达的普通断言

#### Scenario: 特殊失败控制流例外

- **WHEN** 测试存在并发协调、panic/recovery、benchmark、goroutine 内控制流、测试框架边界或无法通过现有语义化断言清晰表达的特殊诊断
- **THEN** 测试 MAY 保留 `t.Fatal`、`t.Fatalf`、`t.Error`、`t.Errorf` 或 `Fail*` 用法
- **AND** 保留项 MUST 能通过代码上下文或实施任务清单说明其符合 `docs/TESTING.md` 的例外规则

#### Scenario: 不引入兼容 helper

- **WHEN** 统一 common 测试断言风格
- **THEN** 系统 MUST NOT 新增旧断言风格兼容 helper、双写断言 wrapper 或仅服务于断言迁移的生产代码
