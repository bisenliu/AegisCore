## ADDED Requirements

### Requirement: RBAC CLI 测试断言规范

RBAC CLI 测试 MUST 优先使用 `testify/require` 表达 seed、assign-super-admin、create-super-admin、password/env normalization、command construction、error handling 和 cleanup behavior 等语义化断言。只有当单个测试需要收集多个互相独立的命令属性失败，且后续检查不依赖前置检查成功时，测试 MAY 使用 `testify/assert`。

#### Scenario: 迁移 RBAC command 历史断言

- **WHEN** `user-service/cmd` 测试检查 `rbac seed`、`rbac assign-super-admin`、`rbac create-super-admin` 或相关 helper 的错误返回、输出文本、flag/env 归一化、password 输入、cleanup error 或 command metadata
- **THEN** 测试 MUST 使用 `require.NoError`、`require.Error`、`require.ErrorContains`、`require.Equal`、`require.NotNil`、`require.Len`、`require.Contains`、`require.Regexp` 或等价语义化断言表达预期
- **AND** 测试 MUST NOT 使用 `t.Fatalf`、`t.Fatal`、`t.Errorf`、`t.Error` 或 `Fail` 类调用表达已有语义化断言可以清晰覆盖的失败

#### Scenario: 不保留旧 RBAC CLI 兼容断言

- **WHEN** RBAC CLI 测试迁移断言表达
- **THEN** 测试 MUST NOT 新增旧 root command alias、旧 RBAC command path、旧 flag/env 名、旧 password handling 或旧 cleanup behavior 兼容断言
- **AND** 迁移 MUST NOT 改变 RBAC seed、超级管理员角色绑定、密码哈希、用户状态或权限目录生产语义

#### Scenario: RBAC CLI 残留手写失败调用符合例外

- **WHEN** `rg "t\\.Fatalf|t\\.Fatal\\(|t\\.Errorf|t\\.Error\\(|Fail(Now)?f?\\(" user-service/cmd --glob '*_test.go'` 在迁移后仍有命中
- **THEN** 每个剩余命中 MUST 属于 `docs/TESTING.md` 允许的自定义测试控制流、特殊诊断输出或测试辅助工具不适合依赖 `testify` 的场景
- **AND** 实现任务记录 MUST 列明这些剩余命中及其保留原因
