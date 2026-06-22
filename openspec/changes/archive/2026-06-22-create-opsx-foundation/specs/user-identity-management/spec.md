## ADDED Requirements

### Requirement: 用户资料创建

系统 MUST 提供用户资料创建能力，支持用户名、昵称、密码和状态的校验、持久化与凭证初始化。

#### Scenario: 创建正常用户

- **WHEN** 调用用户创建能力并提供合法用户名、昵称、密码和正常状态
- **THEN** 系统 MUST 创建用户资料、初始化认证凭证，并返回可用于后续查询和授权绑定的用户 ID

#### Scenario: 用户名不合法

- **WHEN** 创建用户请求包含空用户名、格式不合法用户名或与现有用户冲突的用户名
- **THEN** 系统 MUST 拒绝创建并返回一致的业务错误

#### Scenario: 密码不满足策略

- **WHEN** 创建用户请求中的密码不满足认证密码策略
- **THEN** 系统 MUST 拒绝创建用户和凭证，且 MUST NOT 写入部分成功的数据

### Requirement: 用户资料查询

系统 MUST 提供按用户 ID 查询用户资料和分页列表能力，并保证查询结果使用共享分页和响应契约。

#### Scenario: 查询存在的用户

- **WHEN** 授权调用方按有效用户 ID 查询用户资料
- **THEN** 系统 MUST 返回该用户的 ID、用户名、昵称、状态和创建更新时间等公开资料字段

#### Scenario: 查询不存在的用户

- **WHEN** 调用方按不存在的用户 ID 查询用户资料
- **THEN** 系统 MUST 返回用户不存在错误，而不是返回空成功响应

#### Scenario: 分页列出用户

- **WHEN** 调用方按分页参数列出用户
- **THEN** 系统 MUST 返回用户列表和共享 pagination 信息，并对无效分页参数执行校验

### Requirement: 用户状态约束

系统 MUST 使用统一用户状态模型约束认证、资料查询和 RBAC 绑定行为。

#### Scenario: 正常状态用户参与业务流程

- **WHEN** 用户状态为正常
- **THEN** 系统 MUST 允许其在满足认证和授权条件时访问受保护资源

#### Scenario: 非正常状态用户登录或访问

- **WHEN** 用户状态不允许认证或访问受保护资源
- **THEN** 系统 MUST 拒绝相关认证或授权流程，并返回明确错误

#### Scenario: 状态值不受支持

- **WHEN** 代码或输入尝试使用未定义用户状态
- **THEN** 系统 MUST 通过 domain validation 或输入校验拒绝该状态

### Requirement: 用户 HTTP 边界

系统 MUST 通过 `user-service/internal/features/user/transport/http` 暴露用户资料能力，并受认证和 RBAC 授权保护。

#### Scenario: 未认证调用用户接口

- **WHEN** 未提供有效 bearer token 的调用方访问受保护用户接口
- **THEN** 系统 MUST 在进入用户业务处理前拒绝请求

#### Scenario: 已认证但无权限

- **WHEN** 调用方已认证但没有对应用户接口权限
- **THEN** 系统 MUST 通过 RBAC 授权中间件拒绝请求

#### Scenario: 已授权调用

- **WHEN** 调用方已认证且具备目标用户接口权限
- **THEN** 系统 MUST 执行用户业务流程并返回共享响应 envelope
