## ADDED Requirements

### Requirement: user 与 identity 测试语义化断言规范

user 与 shared identity 范围内的 Go 测试 MUST 使用语义化断言验证用户资料、用户状态、软删除、分页、HTTP response、PostgreSQL adapter 和 identity 状态判断行为。测试 MUST NOT 通过旧手写 if 断言、机械 `Fail` / `Failf` 替换或兼容 helper 隐藏失败信息。

#### Scenario: domain 与 shared identity 测试使用 require 表达状态断言

- **WHEN** user domain 或 `user-service/internal/shared/identity` 测试覆盖用户状态、账号可用性、软删除、ID、用户名、昵称或身份错误行为
- **THEN** 测试 MUST 优先使用 `testify/require` 的错误、对象、布尔、字符串和相等性断言表达预期
- **AND** 后续检查依赖当前结果时 MUST 使用阻塞式 `require` 避免级联失败

#### Scenario: HTTP response 测试使用语义化断言

- **WHEN** user HTTP transport 测试覆盖请求绑定、输入准备、HTTP status、共享 response envelope、pagination 或 response data 字段
- **THEN** 测试 MUST 使用 `require` 或必要时 `assert` 验证状态码、envelope code、success 标记、data shape、pagination 和字段存在性
- **AND** 测试 MUST NOT 增加旧 user 字段、旧状态兼容断言或旧响应 envelope 断言

#### Scenario: PostgreSQL adapter 测试使用语义化断言

- **WHEN** user PostgreSQL infrastructure 测试覆盖创建、查询、列表、软删除过滤、状态过滤、cursor 分页或错误映射
- **THEN** 测试 MUST 使用 `require` 或必要时 `assert` 表达错误、相等性、包含关系、空值、非空值、长度和布尔预期
- **AND** 生产数据库 schema、Ent predicate、软删除语义、分页语义和用户状态语义 MUST 保持不变

#### Scenario: 剩余 testing.T 直接失败调用受限

- **WHEN** user 与 shared identity 目标范围内的 `_test.go` 文件保留 `t.Fatal`、`t.Fatalf`、`t.Error`、`t.Errorf`、`require.Fail`、`require.Failf`、`assert.Fail` 或 `assert.Failf`
- **THEN** 每个剩余命中 MUST 属于 `docs/TESTING.md` 允许的自定义测试控制流、特殊诊断输出或测试辅助工具场景
- **AND** change tasks MUST 列明剩余例外，证明其不是可由现有语义化断言清晰表达的普通断言
