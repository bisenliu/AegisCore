## ADDED Requirements

### Requirement: Role 与 RBAC baseline 测试断言规范

role feature 和 shared RBAC baseline 的 Go 测试 MUST 优先使用 `testify/require` 表达错误、对象、状态、集合、字符串、HTTP response、store 结果和 baseline catalog 等语义化断言。只有当单个测试需要收集多个互相独立的字段失败，且后续检查不依赖前置检查成功时，测试 MAY 使用 `testify/assert`。role 与 baseline 测试 MUST NOT 通过机械 `Fail` / `Failf` 替换、自定义兼容 helper、旧字段断言、旧 binding 断言或旧 baseline catalog 断言来保留历史断言形态。

#### Scenario: 迁移 role 历史断言

- **WHEN** role command、query、seed、domain、HTTP boundary、PostgreSQL store、RoleStore、UserRoleStore 或 RolePermissionStore 测试检查错误返回、对象字段、布尔状态、集合长度、字符串内容、HTTP status 或绑定结果
- **THEN** 测试 MUST 使用 `require.NoError`、`require.Error`、`require.ErrorIs`、`require.Equal`、`require.NotNil`、`require.True`、`require.False`、`require.Len`、`require.Contains` 等语义化断言表达预期
- **AND** 测试 MUST NOT 使用 `t.Fatalf`、`t.Fatal`、`t.Errorf`、`t.Error` 或 `Fail` 类调用表达已有语义化断言可以清晰覆盖的失败

#### Scenario: 迁移 baseline catalog 历史断言

- **WHEN** shared RBAC baseline 测试检查系统角色、系统权限、默认绑定、超级管理员常量或 catalog 唯一性
- **THEN** 测试 MUST 使用 `require` 或必要时 `assert` 表达错误、相等性、包含关系、空值、非空值、长度、唯一性和布尔预期
- **AND** 测试 MUST NOT 新增旧 baseline 常量、旧 catalog 条目或旧绑定关系兼容断言

#### Scenario: 收集互相独立字段失败

- **WHEN** 多字段 HTTP response、角色列表摘要、权限摘要或 baseline catalog 测试需要在一次执行中展示多个互相独立字段的差异
- **THEN** 测试 MAY 使用 `assert` 收集这些字段失败
- **AND** 初始化失败、错误返回、nil 检查、响应解析、store 连接或后续检查依赖的前置条件仍然 MUST 使用 `require` 立即终止当前测试

#### Scenario: 保持 collaborator 契约表达

- **WHEN** role application、transport/http、seed 或 store 相关测试依赖已有 gomock 生成物表达外部协作者调用、失败注入或调用顺序
- **THEN** 测试 MUST 保持既有生成 mock 使用方式
- **AND** 本次断言迁移 MUST NOT 回退为手写 store double、notifier double、fake 或通过 helper 隐藏 collaborator expectation

#### Scenario: 不保留旧兼容断言

- **WHEN** role 与 baseline 测试迁移断言表达
- **THEN** 测试 MUST NOT 新增旧 role 字段、旧 binding 行为、旧 baseline catalog、旧 fake 或旧 helper 兼容断言
- **AND** 测试 MUST NOT 新增机械 `require.Fail`、`require.Failf`、`assert.Fail` 或 `assert.Failf` 替换来模拟历史手写失败判断

#### Scenario: 残留手写失败调用符合例外

- **WHEN** `rg "t\\.Fatalf|t\\.Fatal\\(|t\\.Errorf|t\\.Error\\(|Failf?\\(" user-service/internal/features/role user-service/internal/shared/rbacbaseline --glob '*_test.go'` 在迁移后仍有命中
- **THEN** 每个剩余命中 MUST 属于 `docs/TESTING.md` 允许的自定义测试控制流、特殊诊断输出或测试辅助工具不适合依赖 `testify` 的场景
- **AND** 实现任务记录 MUST 列明这些剩余命中及其保留原因
