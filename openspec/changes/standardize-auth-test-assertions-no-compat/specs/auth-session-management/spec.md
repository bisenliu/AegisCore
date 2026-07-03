## ADDED Requirements

### Requirement: auth 测试语义化断言规范

auth 范围内的 Go 测试 MUST 使用语义化断言验证认证会话、credential、refresh session、token、password change、HTTP response、Redis/PostgreSQL adapter、metrics 和 provider 行为。测试 MUST NOT 通过旧手写 if 断言、机械 `Fail` / `Failf` 替换或兼容 helper 隐藏失败信息。

#### Scenario: application 测试使用 require 表达安全路径断言

- **WHEN** auth application、credentials、sessions、tokens、validators 或 authctx 测试覆盖登录、刷新、强制改密、改密、退出、token version 或 client/session context 行为
- **THEN** 测试 MUST 优先使用 `testify/require` 的错误、对象、布尔、集合、字符串和类型断言表达预期
- **AND** 后续检查依赖当前结果时 MUST 使用阻塞式 `require` 避免级联失败

#### Scenario: HTTP controller 和 input 测试使用语义化断言

- **WHEN** auth HTTP transport 测试覆盖请求输入归一化、use case 调用、HTTP status、response envelope、强制改密响应、错误码或响应 data 字段
- **THEN** 测试 MUST 使用 `require` 或必要时 `assert` 验证状态码、envelope code、success 标记、data shape 和字段存在性
- **AND** 测试 MUST NOT 增加旧 auth HTTP 字段、旧错误码、旧 token 类型或旧状态兼容断言

#### Scenario: adapter 和 provider 测试使用语义化断言

- **WHEN** auth Redis/PostgreSQL infrastructure、metrics、Fx/provider 或 `user-service/internal/providers/auth_test.go` 测试覆盖 store、key schema、TTL、token version cache、credential update、metrics collector 或 provider 构造行为
- **THEN** 测试 MUST 使用 `require` 或必要时 `assert` 表达错误、相等性、包含关系、空值、非空值、长度和布尔预期
- **AND** 生产 Redis key、PostgreSQL schema、JWT claims、配置和 provider 装配语义 MUST 保持不变

#### Scenario: 剩余 testing.T 直接失败调用受限

- **WHEN** auth 目标范围内的 `_test.go` 文件保留 `t.Fatal`、`t.Fatalf`、`t.Error`、`t.Errorf`、`require.Fail`、`require.Failf`、`assert.Fail` 或 `assert.Failf`
- **THEN** 每个剩余命中 MUST 属于 `docs/TESTING.md` 允许的自定义测试控制流、特殊诊断输出或测试辅助工具场景
- **AND** change tasks MUST 列明剩余例外，证明其不是可由现有语义化断言清晰表达的普通断言
