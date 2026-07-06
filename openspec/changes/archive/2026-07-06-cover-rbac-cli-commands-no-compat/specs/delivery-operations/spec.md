## ADDED Requirements

### Requirement: CLI 命令测试语义化断言

`delivery-operations` 的 user-service 命令测试 MUST 使用语义化断言表达 CLI 错误、参数缺失、依赖初始化、cleanup 合并和命令属性检查。测试 MUST 优先使用 `require` fail-fast 断言；只有互相独立且不影响后续测试前置条件的命令 property 检查 MAY 使用 `assert`。

#### Scenario: 命令错误使用 fail-fast 断言

- **WHEN** CLI 测试验证参数缺失、配置错误、依赖初始化错误、命令执行错误或 cleanup 错误
- **THEN** 测试 MUST 使用 `require.Error`、`require.ErrorContains`、`require.ErrorIs`、`require.ErrorAs` 或等价 fail-fast 断言
- **AND** 测试 MUST NOT 使用 `require.True`、`assert.True` 或手写 `if` 拼装错误断言替代更具体的错误断言

#### Scenario: 后续检查依赖当前命令结果

- **WHEN** 后续断言需要依赖命令执行成功、初始化成功、返回对象非空或 error 类型匹配
- **THEN** 测试 MUST 使用 `require` 断言建立前置条件
- **AND** 失败后 MUST 停止当前测试，避免继续读取无效结果

#### Scenario: 独立命令属性允许 assert

- **WHEN** 多个命令 flag 默认值、短描述、Use 字符串或互相独立的布尔属性彼此不构成前置依赖
- **THEN** 测试 MAY 使用 `assert` 聚合这些独立属性检查
- **AND** 若存在 `Len`、`Contains`、`ElementsMatch`、`Regexp`、`Greater`、`LessOrEqual` 等更具体断言，测试 MUST 优先使用具体断言

#### Scenario: 禁止机械 failure helper 替换

- **WHEN** 新增或修改 user-service 命令测试
- **THEN** 测试 MUST NOT 使用机械 `Fail`、`Failf`、`FailNow`、`FailNowf` 或旧手写断言兼容 helper 表达常见断言
- **AND** `t.Fatal`、`t.Fatalf`、`t.Error` 和 `t.Errorf` 只允许出现在 `docs/TESTING.md` 明确允许的边界内
