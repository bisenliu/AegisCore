## ADDED Requirements

### Requirement: RBAC CLI 命令测试覆盖

`rbac-access-control` 的 user-service RBAC CLI 测试 MUST 直接覆盖当前 `user-service rbac` seed、assign-super-admin 和 create-super-admin 命令契约。测试 MUST 固定当前配置来源、参数归一化、依赖装配、错误传播和 cleanup 语义，并 MUST NOT 为旧命令名、旧 flag、旧环境变量、旧 root Makefile 无服务前缀入口或旧 bootstrap 行为新增兼容断言。

#### Scenario: seed 命令 runner 传递当前选项

- **WHEN** `runRBACSeedCommand` 使用当前配置路径和 seed options 执行
- **THEN** 测试 MUST 验证 runner 通过当前 RBAC seed service 接收 `reactivateSystem` 和 `syncSystemBindings`
- **AND** 测试 MUST 验证成功路径执行 cleanup

#### Scenario: assign-super-admin 命令 runner 绑定指定用户

- **WHEN** `runAssignSuperAdminCommand` 收到合法用户 UUID
- **THEN** 测试 MUST 验证 runner 将该 UUID 传递给当前超级管理员绑定流程
- **AND** 测试 MUST 覆盖绑定已存在和新增绑定两类当前输出语义

#### Scenario: create-super-admin 命令 runner 使用当前创建流程

- **WHEN** `runCreateSuperAdminCommand` 收到当前 create-super-admin options
- **THEN** 测试 MUST 验证 runner 使用当前配置路径初始化依赖并调用 `createSuperAdmin`
- **AND** 测试 MUST 验证输出中的 username 使用当前 username 归一化规则

#### Scenario: createSuperAdmin 覆盖用户存在性分支

- **WHEN** `createSuperAdmin` 处理不存在用户、已存在用户不重置密码或已存在用户重置密码
- **THEN** 测试 MUST 验证新建用户、角色绑定、密码 hash 和凭据更新按当前契约发生
- **AND** 测试 MUST 验证用户读取、创建、hash、凭据更新和角色绑定错误会 fail-fast 返回

#### Scenario: RBAC CLI 初始化和 cleanup 错误可见

- **WHEN** RBAC CLI 依赖初始化失败或命令执行后 cleanup 返回错误
- **THEN** 测试 MUST 验证命令返回明确错误
- **AND** 如果命令错误和 cleanup 错误同时存在，测试 MUST 验证两者都保留在返回错误中

### Requirement: RBAC CLI 参数归一化测试

`rbac-access-control` 的 create-super-admin 参数归一化测试 MUST 固定当前 username、nickname、password env、password value 和 reset password 语义。测试 MUST 只验证当前 `ADMIN_PASSWORD` 默认来源和当前 flag/env 契约，不得新增旧环境变量或旧默认值兼容路径。

#### Scenario: create-super-admin 使用默认 password env

- **WHEN** `passwordEnv` 为空且 `ADMIN_PASSWORD` 存在
- **THEN** `normalizeCreateSuperAdminOptions` MUST 使用 `ADMIN_PASSWORD`
- **AND** username MUST trim 后转小写，空 nickname MUST 回退为归一化 username

#### Scenario: create-super-admin 拒绝缺失必要输入

- **WHEN** password env 不存在、username 为空或 password value 为空
- **THEN** `normalizeCreateSuperAdminOptions` MUST 返回明确错误
- **AND** 测试 MUST 使用 `require.ErrorContains` 或等价语义化断言表达错误内容

#### Scenario: reset password 标志保持当前值

- **WHEN** create-super-admin options 启用 reset password
- **THEN** 归一化结果 MUST 保留 `resetPassword=true`
- **AND** 测试 MUST NOT 通过旧 flag 或旧环境变量表达重置密码预期
