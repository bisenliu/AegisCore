## ADDED Requirements

### Requirement: 服务装配边界测试断言依赖与例外治理

服务装配边界测试 MUST 可以直接使用标准 `testify/require` 与 `testify/assert` 表达常见错误、对象、数值范围、集合、字符串、JSON、正则、时间和 panic 断言。系统 MUST NOT 为迁移 router、provider 或 bootstrap 历史测试断言新增跨服务兼容 helper、机械失败包装器、共享断言 facade 或仅服务于测试的生产 API。

#### Scenario: 直接使用标准 testify 断言

- **WHEN** `user-service/internal/router`、`providers` 或 `bootstrap` 测试需要验证错误、对象和值、数值范围、集合长度、元素集合、字符串包含、JSON 等价、正则匹配、时间边界或 panic 行为
- **THEN** 测试 MUST 优先使用 `require.NoError`、`require.Error`、`require.ErrorContains`、`require.Equal`、`require.NotNil`、`require.Len`、`require.Greater`、`require.Less`、`require.ElementsMatch`、`require.JSONEq`、`require.Regexp`、`require.WithinDuration`、`require.Panics` 或等价语义化断言
- **AND** 测试 MUST NOT 使用 `require.True`、`require.False`、手写 `if` 或多个基础断言拼凑上述已有语义化断言可以清晰覆盖的检查

#### Scenario: 多个独立检查可使用 assert

- **WHEN** 单个测试需要在一次执行中收集多个互相独立的 route、provider 输出、metric family、label、日志字段或 health check 结果失败
- **THEN** 测试 MAY 使用 `testify/assert` 进行非阻塞式断言
- **AND** 初始化失败、前置条件失败或后续检查依赖当前结果时 MUST 使用 `testify/require`

#### Scenario: 禁止新增断言兼容层

- **WHEN** 迁移历史 `t.Fatal`、`t.Error` 或泛化布尔断言
- **THEN** 系统 MUST NOT 新增旧断言风格兼容 helper、共享 wrapper、机械 `Fail*` 替换、测试专用生产分支或仅为单元测试暴露的运行时 API
- **AND** 迁移 MUST 基于现有实现和合理的测试可读性完成

#### Scenario: testing.T 直接失败例外

- **WHEN** 目标测试保留直接 `testing.T` 失败方法或 `Fail*` 调用
- **THEN** 保留项 MUST 符合 `docs/TESTING.md` 中自定义测试控制流、特殊诊断输出或测试辅助工具不适合依赖 `testify` 的例外规则
- **AND** 普通错误、相等性、包含关系、长度、空值、数值范围、字符串、JSON、正则、时间或 panic 断言 MUST 使用语义化 `require` 或必要时 `assert`
