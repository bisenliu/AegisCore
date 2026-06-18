## Purpose

定义 user-service 的 RBAC 访问控制能力，覆盖权限目录、角色、角色权限、用户角色、Casbin 授权、系统 seed 和超级管理员引导。

## Requirements

### Requirement: 权限目录管理

系统 MUST 提供权限目录创建、更新、启停、查询、列表和路由差异分析能力，用于描述可授权的 HTTP 资源和动作。

#### Scenario: 创建权限

- **WHEN** 授权调用方提供合法权限标识、方法、路径和描述
- **THEN** 系统 MUST 创建权限记录，并使其可参与后续角色绑定和授权判断

#### Scenario: 权限输入非法

- **WHEN** 权限方法、路径、标识或描述不满足 domain validation
- **THEN** 系统 MUST 拒绝创建或更新，并返回一致的校验错误

#### Scenario: 路由差异分析

- **WHEN** 系统扫描已注册 HTTP 路由并与权限目录比较
- **THEN** 系统 MUST 能识别缺失、冗余或不一致的权限定义

### Requirement: 角色与权限绑定

系统 MUST 提供角色创建、更新、查询、列表和角色权限绑定能力，并保证绑定引用的权限存在且状态可用。

#### Scenario: 创建角色并绑定权限

- **WHEN** 授权调用方创建角色并指定合法权限集合
- **THEN** 系统 MUST 持久化角色、写入角色权限绑定，并使授权策略可同步使用

#### Scenario: 绑定不存在权限

- **WHEN** 角色绑定请求引用不存在或不可用的权限
- **THEN** 系统 MUST 拒绝绑定并保持已有角色权限关系不被破坏

#### Scenario: 查询角色列表

- **WHEN** 授权调用方分页查询角色
- **THEN** 系统 MUST 返回角色列表、权限摘要和共享 pagination 信息

### Requirement: 用户角色绑定

系统 MUST 支持将角色绑定到用户，并为授权判断提供用户有效权限查询能力。

#### Scenario: 绑定角色给用户

- **WHEN** 授权调用方把存在的角色绑定给存在用户
- **THEN** 系统 MUST 写入用户角色关系，并使该用户后续访问权限生效

#### Scenario: 用户或角色不存在

- **WHEN** 用户角色绑定请求引用不存在的用户或角色
- **THEN** 系统 MUST 拒绝绑定并返回明确错误

#### Scenario: 查询用户有效权限

- **WHEN** 系统或调用方查询某用户有效权限
- **THEN** 系统 MUST 聚合用户角色和角色权限，返回该用户当前可访问的权限集合

### Requirement: Casbin 授权保护

系统 MUST 使用 RBAC 授权中间件保护权限、角色和用户业务接口，并在认证通过后执行资源级授权判断。

#### Scenario: 授权通过

- **WHEN** 已认证用户拥有当前 HTTP 方法和路径对应权限
- **THEN** 系统 MUST 允许请求进入目标 controller

#### Scenario: 授权失败

- **WHEN** 已认证用户缺少当前 HTTP 方法和路径对应权限
- **THEN** 系统 MUST 拒绝请求并返回授权失败错误

#### Scenario: 权限策略更新

- **WHEN** 权限、角色或绑定发生变化
- **THEN** 系统 MUST 同步或刷新授权策略，避免旧策略长期影响授权判断

### Requirement: RBAC 系统数据引导

系统 MUST 提供 CLI 能力初始化系统角色、系统权限、系统绑定，并支持为用户分配或创建超级管理员。

#### Scenario: 初始化 RBAC 系统数据

- **WHEN** 运维执行 `aegiscore-user-services rbac seed`
- **THEN** 系统 MUST 创建或更新默认系统角色、权限和绑定，并输出插入、更新、绑定增删统计

#### Scenario: 创建超级管理员

- **WHEN** 运维执行 `create-super-admin` 并提供管理员密码环境变量
- **THEN** 系统 MUST 创建或复用管理员用户，按需更新密码，并绑定内置超级管理员角色

#### Scenario: 缺少管理员密码

- **WHEN** 创建超级管理员时缺少配置的密码环境变量或密码为空
- **THEN** 系统 MUST 拒绝执行并返回明确错误
