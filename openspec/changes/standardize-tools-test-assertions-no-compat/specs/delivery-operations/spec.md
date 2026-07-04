## ADDED Requirements

### Requirement: 仓库级工具测试断言验证
仓库级工具测试 MUST 遵循统一 Go 测试断言规范。OpenAPI 转换、CLI 工具输入输出、文件生成和交付验证相关工具测试 MUST 优先使用 `testify/require` 的语义化断言；存在更具体的 `Len`、`ErrorContains`、`Contains`、`ElementsMatch`、`JSONEq`、`Regexp` 等断言时，测试 MUST NOT 使用 `True` / `False` 或手写 `if` 拼装同等检查。

#### Scenario: 迁移工具测试断言
- **WHEN** `tools/**/*_test.go` 或仓库级工具测试断言错误、文件内容、JSON/YAML 输出、集合长度、字符串匹配或生成物路径
- **THEN** 测试 MUST 使用 `require.NoError`、`require.ErrorContains`、`require.Len`、`require.Contains`、`require.ElementsMatch`、`require.JSONEq`、`require.Regexp` 或等价语义化断言表达检查
- **AND** 测试 MUST NOT 使用手写 `t.Fatalf` / `t.Errorf` 或 `require.True` / `require.False` 包装可由专属断言表达的检查

#### Scenario: 工具测试包为空
- **WHEN** 当前仓库级 `tools` 范围没有 Go 测试包或没有 `_test.go` 文件
- **THEN** change tasks MUST 记录实际包列表、扫描结果和替代验证命令
- **AND** 系统 MUST NOT 为了满足迁移任务而新增旧工具输出格式、旧 CLI flag 或旧文件路径兼容断言

#### Scenario: 多个独立工具输出差异
- **WHEN** 单个工具测试需要在一次执行中检查多个独立输出字段、文件内容差异或生成路径差异，且后续检查不依赖前置检查成功
- **THEN** 测试 MAY 使用 `testify/assert` 收集这些独立断言失败
- **AND** 初始化、命令执行、文件读取或解析失败 MUST 使用 `require` 立即终止当前测试
