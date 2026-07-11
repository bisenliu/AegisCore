## ADDED Requirements

### Requirement: RBAC CLI runner 测试局部依赖注入

`rbac-access-control` 的 user-service RBAC CLI command 测试 MUST 通过 root command 的本地依赖注入覆盖 `rbac seed`、`rbac assign-super-admin` 和 `rbac create-super-admin` runner。正式代码 MUST NOT 为这些 RBAC runner 暴露 package-level 可变函数变量，且本次装配重构 MUST 保持 RBAC seed、超级管理员分配、超级管理员创建、cleanup 和错误传播的生产语义不变。

#### Scenario: seed command 通过本地 runner 捕获选项

- **WHEN** `user-service/cmd` 测试执行 `rbac seed` 并传入 `--reactivate-system` 或 `--sync-system-bindings`
- **THEN** 测试 MUST 通过当前 root command 实例的本地 runner 替身断言配置路径和 seed options
- **AND** 测试 MUST NOT 通过赋值 package-level `runRBACSeed` 或等价可变全局函数变量注入 runner

#### Scenario: assign-super-admin command 通过本地 runner 校验 UUID

- **WHEN** `user-service/cmd` 测试执行 `rbac assign-super-admin --user-id <uuid>`
- **THEN** 测试 MUST 通过当前 root command 实例的本地 runner 替身断言配置路径和用户 UUID
- **AND** 非法 UUID 输入 MUST 在调用 runner 前被拒绝，且不得调用本地 runner 替身

#### Scenario: create-super-admin command 通过本地 runner 捕获参数

- **WHEN** `user-service/cmd` 测试执行 `rbac create-super-admin` 并传入默认值或显式 flag
- **THEN** 测试 MUST 通过当前 root command 实例的本地 runner 替身断言 username、nickname、password env 和 reset password 选项
- **AND** 测试 MUST NOT 通过保存和恢复 package-level `runCreateSuperAdmin` 或等价可变全局函数变量表达替身

#### Scenario: RBAC CLI 生产语义保持不变

- **WHEN** 运维执行 `rbac seed`、`rbac assign-super-admin` 或 `rbac create-super-admin`
- **THEN** 系统 MUST 继续调用现有真实 RBAC runner，并保持 RBAC 系统数据初始化、超级管理员绑定、密码处理、输出文本、cleanup 和错误传播语义不变
