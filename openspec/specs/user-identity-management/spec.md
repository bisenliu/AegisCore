## Purpose

定义 user-service 的用户资料创建、查询、列表、状态约束和用户 HTTP 边界，保证用户身份数据可被认证和 RBAC 能力稳定复用。
## Requirements
### Requirement: 用户资料创建

系统 MUST 提供用户资料创建能力，支持用户名、昵称、密码和状态的校验、持久化与凭证初始化。

#### Scenario: 创建正常用户

- **WHEN** 调用用户创建能力并提供合法用户名、昵称、密码和正常状态
- **THEN** 系统 MUST 创建用户资料、初始化认证凭证，并返回可用于后续查询和授权绑定的 UUID `user_id`

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
- **THEN** 系统 MUST 返回该用户的 UUID `user_id`、用户名、昵称、状态和创建更新时间等公开资料字段，并 MUST NOT 暴露内部数字数据库 ID

#### Scenario: 查询不存在的用户

- **WHEN** 调用方按不存在的用户 ID 查询用户资料
- **THEN** 系统 MUST 返回用户不存在错误，而不是返回空成功响应

#### Scenario: 分页列出用户

- **WHEN** 调用方按分页参数列出用户
- **THEN** 系统 MUST 返回用户列表和共享 pagination 信息，并对无效分页参数执行校验

### Requirement: 用户查询索引支撑

系统 MUST 为用户资料查询和列表能力维护与稳定查询模式匹配的数据库索引，并通过 Ent schema 和 Atlas migration 交付可审查的结构变更。

#### Scenario: 用户列表分页索引

- **WHEN** 调用方按软删除状态、用户状态或用户 ID cursor 分页列出用户
- **THEN** 数据库 schema MUST 提供支持未软删除过滤、状态过滤和 `user_id` keyset 排序的索引

#### Scenario: 用户昵称普通索引

- **WHEN** 用户资料 schema 定义 `nickname` 字段索引
- **THEN** 数据库 schema MUST 为 `users.nickname` 提供 Ent 可生成的普通索引
- **AND** 系统 MUST NOT 依赖 PostgreSQL GIN、`gin_trgm_ops` 或插件相关索引作为昵称字段的持久化要求

#### Scenario: 用户身份索引不改变业务语义

- **WHEN** 用户查询索引发生调整
- **THEN** 用户创建、用户名唯一性、软删除过滤、用户状态约束、HTTP 响应字段和错误语义 MUST 保持不变

### Requirement: 用户 feature 分层

系统 MUST 将用户资料能力按 feature-local 分层组织，保证用户 use case、协议 DTO 和 Ent adapter 归属清晰。

#### Scenario: 新增用户写侧用例

- **WHEN** 新增创建、更新或其他用户写侧操作
- **THEN** 用例 MUST 位于 `user-service/internal/features/user/application/command`，并通过用户 feature application 拥有的端口访问基础设施

#### Scenario: 新增用户读侧用例

- **WHEN** 新增用户查询或列表读侧操作
- **THEN** 查询实现 MUST 位于 `user-service/internal/features/user/application/query`，并消费用户 feature application 拥有的端口

#### Scenario: 用户 HTTP DTO 和输入准备

- **WHEN** 用户 HTTP controller 处理 path、query 或 body 字段
- **THEN** HTTP request/response DTO MUST 位于 `transport/http`，协议无关输入辅助 SHOULD 位于 `application/validators`，Ent 访问 MUST 留在 `infrastructure/postgres`

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

### Requirement: 用户资料路由注册测试覆盖
系统 MUST 使用 router 包测试覆盖用户资料路由在 user-service 聚合路由中的注册结果，确保用户接口只存在于当前 `/api/v1/users` 路由图并受当前认证和 RBAC 授权中间件链保护。

#### Scenario: 用户资料路由注册
- **WHEN** `registerV1Routes` 注册当前 `/api/v1` 路由组
- **THEN** 测试 MUST 验证用户创建、用户详情和用户列表路由注册在 `/api/v1/users` 下
- **AND** 测试 MUST 验证这些用户资料路由进入当前认证和 RBAC 授权中间件链
- **AND** 测试 MUST NOT 为 `/api/users`、`/v1/users` 或其他旧用户路径保留兼容断言

### Requirement: 用户资料 E2E 响应断言规范
系统 MUST 使用语义化断言覆盖 user-service E2E HTTP flow 中的用户创建、用户详情查询和用户状态流转响应。断言迁移 MUST 保持用户资料业务语义、测试数据构造和公开响应字段不变。

#### Scenario: 创建用户响应断言
- **WHEN** E2E flow 调用 `POST /api/v1/users` 创建用户
- **THEN** 测试 MUST 使用 `require.NotEmpty`、`require.Equal` 或必要 `assert` 验证 `user_id`、`username` 和当前成功 response envelope
- **AND** 测试 MUST NOT 接受旧用户响应字段、空 `user_id` 或旧创建状态兼容断言

#### Scenario: 查询用户响应断言
- **WHEN** E2E flow 调用 `GET /api/v1/users/:id` 查询目标用户
- **THEN** 测试 MUST 使用语义化断言验证 `user_id`、`username`、`status` 和当前成功 response envelope
- **AND** 测试 MUST NOT 改变公开资料字段、内部 ID 隐藏语义或用户不存在错误语义

#### Scenario: 强制改密后用户状态断言
- **WHEN** E2E flow 创建强制改密用户、完成改密并再次查询该用户
- **THEN** 测试 MUST 使用语义化断言验证用户状态从 `UserStatusMustChangePassword` 流转到 `UserStatusNormal`
- **AND** 迁移 MUST NOT 修改用户状态常量、账号生命周期判断、测试密码或 seed 数据构造

### Requirement: 用户身份错误应用错误渲染

系统 MUST 将用户资料能力中的用户已存在和用户不存在错误表达为可由共享 response helper 直接渲染的应用错误，并保持用户 HTTP 边界无专用 sentinel-to-HTTP 兼容映射。

#### Scenario: 用户已存在渲染为冲突响应

- **WHEN** 用户创建流程返回 `identity.ErrUserAlreadyExists`
- **THEN** 用户 HTTP 边界 MUST 通过 `response.Fail(c, err)` 渲染失败响应
- **AND** 响应 MUST 为 `409 Conflict` 和共享冲突业务 code
- **AND** 响应 message MUST 使用当前用户已存在公开文案

#### Scenario: 用户不存在渲染为未找到响应

- **WHEN** 用户详情查询流程返回 `identity.ErrUserNotFound`
- **THEN** 用户 HTTP 边界 MUST 通过 `response.Fail(c, err)` 渲染失败响应
- **AND** 响应 MUST 为 `404 Not Found` 和共享未找到业务 code
- **AND** 响应 message MUST 使用当前用户不存在公开文案

#### Scenario: 用户业务错误保留 errors.Is 语义

- **WHEN** 用户 feature 或测试需要判断用户已存在或用户不存在错误
- **THEN** `errors.Is(err, identity.ErrUserAlreadyExists)` 与 `errors.Is(err, identity.ErrUserNotFound)` MUST 继续返回正确结果
- **AND** 系统 MUST NOT 为用户 HTTP transport 保留 `toUserHTTPError` 或等价兼容函数

### Requirement: 用户 HTTP controller 统一错误出口

用户 HTTP controller MUST 对业务 service 返回错误使用共享 `response.Fail` 入口，避免在 transport 层重复维护用户身份错误到 HTTP 响应的映射。

#### Scenario: 用户创建业务错误

- **WHEN** `CreateUser` controller 调用用户创建 use case 返回错误
- **THEN** controller MUST 直接调用 `response.Fail(c, err)`
- **AND** controller MUST NOT 先调用用户专用错误 mapper

#### Scenario: 用户详情查询业务错误

- **WHEN** `GetByUserID` controller 调用用户查询 use case 返回错误
- **THEN** controller MUST 直接调用 `response.Fail(c, err)`
- **AND** controller MUST NOT 先调用用户专用错误 mapper

#### Scenario: 用户列表查询业务错误

- **WHEN** `ListUsers` controller 调用用户列表 use case 返回错误
- **THEN** controller MUST 直接调用 `response.Fail(c, err)`
- **AND** controller MUST NOT 先调用用户专用错误 mapper

