## Purpose

定义 user-service 的用户资料创建、查询、列表、状态约束和 HTTP 边界，保证用户身份数据可被认证和 RBAC 能力稳定复用。

## Requirements

### Requirement: 用户资料创建

系统 MUST 提供用户资料创建能力，对用户名、昵称、密码和状态进行校验，并以一致方式持久化用户资料和初始化认证凭证。

#### Scenario: 创建用户成功

- **WHEN** 已授权调用方通过 `POST /api/v1/users` 提交合法用户名、昵称、密码和受支持状态
- **THEN** 系统 MUST 原子创建用户资料和认证凭证
- **AND** 响应 MUST 使用共享成功 envelope，并返回可用于查询和 RBAC 绑定的 UUID `user_id`

#### Scenario: 创建参数无效

- **WHEN** 用户名为空、格式不合法，状态不受支持，或密码不满足认证密码策略
- **THEN** 系统 MUST 拒绝创建并返回一致的校验错误
- **AND** 系统 MUST NOT 写入部分成功的用户资料或凭证

#### Scenario: 用户名已存在

- **WHEN** 创建流程发现用户名与现有用户冲突并返回 `identity.ErrUserAlreadyExists`
- **THEN** 共享错误渲染 MUST 返回 `409 Conflict`、共享冲突业务 code 和当前用户已存在公开文案
- **AND** 该错误在包装前后 MUST 保持正确的 `errors.Is` 匹配语义

### Requirement: 用户资料查询与索引

系统 MUST 提供按 UUID 用户 ID 查询公开资料和分页列出用户的能力，并为稳定查询模式维护由 Ent schema 和 Atlas migration 管理的数据库索引。

#### Scenario: 查询存在的用户

- **WHEN** 已授权调用方通过 `GET /api/v1/users/:id` 查询存在的用户
- **THEN** 系统 MUST 返回 UUID `user_id`、用户名、昵称、状态和创建更新时间等公开资料字段
- **AND** 系统 MUST NOT 暴露内部数字数据库 ID

#### Scenario: 查询不存在的用户

- **WHEN** 查询流程未找到目标用户并返回 `identity.ErrUserNotFound`
- **THEN** 共享错误渲染 MUST 返回 `404 Not Found`、共享未找到业务 code 和当前用户不存在公开文案，而不是空成功响应
- **AND** 该错误在包装前后 MUST 保持正确的 `errors.Is` 匹配语义

#### Scenario: 分页列出用户

- **WHEN** 已授权调用方通过 `GET /api/v1/users` 提交有效分页、状态或 cursor 参数
- **THEN** 系统 MUST 按软删除过滤、状态过滤和 `user_id` keyset 顺序返回用户列表及共享 pagination 信息
- **AND** 系统 MUST 拒绝无效分页参数

#### Scenario: 查询索引支撑

- **WHEN** 数据库执行用户列表过滤、`user_id` cursor 排序或昵称查询
- **THEN** schema MUST 提供与未软删除过滤、状态过滤、`user_id` keyset 排序及 `nickname` 普通索引相匹配的索引
- **AND** 昵称索引 MUST NOT 依赖 PostgreSQL GIN、`gin_trgm_ops` 或插件
- **AND** 索引调整 MUST NOT 改变用户名唯一性、软删除、状态、响应字段或错误语义

### Requirement: 用户状态约束

系统 MUST 使用 `shared/identity` 维护的统一用户状态模型约束资料、认证和 RBAC 相关流程。

#### Scenario: 正常状态用户参与业务流程

- **WHEN** 用户状态为 `UserStatusNormal`
- **THEN** 系统 MUST 在认证和授权条件满足时允许其访问受保护资源

#### Scenario: 强制改密状态流转

- **WHEN** 状态为 `UserStatusMustChangePassword` 的用户完成受限改密流程
- **THEN** 系统 MUST 将用户状态转换为 `UserStatusNormal`
- **AND** 状态转换前系统 MUST NOT 允许该用户以普通登录会话访问受保护资源

#### Scenario: 不可用或未知状态

- **WHEN** 用户状态为 disabled 等不可用状态，或输入包含未定义状态值
- **THEN** 系统 MUST 拒绝相应认证、授权或资料写入流程
- **AND** 系统 MUST NOT 通过旧状态别名放宽状态约束

### Requirement: 用户 HTTP 边界

系统 MUST 仅通过 `/api/v1/users` 路由图暴露用户创建、详情和列表接口，MUST 在业务处理前执行 bearer token 认证和 RBAC 授权，并 MUST 使用共享响应 envelope 和错误出口。

#### Scenario: 未认证调用

- **WHEN** 调用方未提供有效 bearer token 访问用户接口
- **THEN** 系统 MUST 在进入用户业务处理前拒绝请求

#### Scenario: 已认证但无权限

- **WHEN** 调用方已认证但没有目标用户接口权限
- **THEN** RBAC 授权中间件 MUST 拒绝请求

#### Scenario: 已授权调用

- **WHEN** 调用方已认证且具备目标用户接口权限
- **THEN** 系统 MUST 执行对应用户业务流程并返回共享响应 envelope
- **AND** 系统 MUST NOT 暴露 `/api/users`、`/v1/users` 或其他旧路径别名

#### Scenario: 业务错误统一渲染

- **WHEN** 用户创建、详情或列表 use case 返回业务错误
- **THEN** controller MUST 通过共享 `response.Fail(c, err)` 渲染响应
- **AND** HTTP transport MUST NOT 维护用户专用 sentinel-to-HTTP mapper

### Requirement: 用户 feature 边界与日志

系统 MUST 将用户资料能力按 feature-local 的 application、domain、transport 和 infrastructure 边界组织；application port MUST 由消费侧拥有，HTTP DTO MUST 留在 transport，Ent 访问 MUST 留在 PostgreSQL adapter。用户 feature 的 domain、application、infrastructure 和 transport 生产包 MUST 使用 framework-neutral constructor 与消费侧 port 注入，MUST NOT 导入 Fx/Dig 或携带 Fx/Dig metadata；正式业务路径的日志 MUST 来自 constructor 注入的 logger 或 request context，MUST NOT 依赖可变的 package-level 默认 logger。

#### Scenario: 用户写侧和读侧用例

- **WHEN** 新增或维护用户写侧、查询或列表行为
- **THEN** 写侧编排 MUST 位于 `application/command`，读侧编排 MUST 位于 `application/query`
- **AND** application MUST 通过自身拥有的最小 port 访问基础设施，MUST NOT 导入 HTTP DTO 或 Ent predicate 包

#### Scenario: HTTP 输入和持久化边界

- **WHEN** controller 处理 path、query 或 body 字段
- **THEN** request/response DTO MUST 位于 `transport/http`，输入裁剪和归一化 MUST 在 feature-local input preparer 完成
- **AND** input preparer MUST NOT 查询 store、调用 use case、执行授权或写 HTTP 响应
- **AND** Ent 查询和 predicate 构造 MUST 留在 `infrastructure/postgres`

#### Scenario: framework-neutral 基础设施构造

- **WHEN** 用户 feature 的基础设施 adapter 暴露生产 constructor
- **THEN** constructor MUST 使用显式普通 Go 参数表达依赖，并由消费侧通过 application port 注入
- **AND** domain、application、infrastructure 和 transport 生产包 MUST NOT 导入 `go.uber.org/fx`、`go.uber.org/dig` 或声明 `fx.In`、`fx.Out`、`dig.In`、`dig.Out`
- **AND** 基础设施 constructor MUST NOT 通过 `name:"primary_db"` 等 Fx/Dig struct tag 暴露服务级命名资源 metadata

#### Scenario: Fx composition 层适配

- **WHEN** 服务运行时通过 Fx 组装用户资料 feature
- **THEN** Fx module MAY 在 composition 文件中适配服务级命名资源并提供用户 application port
- **AND** 该适配 MUST NOT 要求 PostgreSQL adapter 保留 Fx/Dig 专用参数结构、兼容 wrapper 或旧 constructor

#### Scenario: 显式日志依赖

- **WHEN** 用户 application、HTTP 边界或关键 PostgreSQL adapter 记录正式业务日志
- **THEN** logger MUST 由 constructor 显式注入或从当前 request context 获取
- **AND** request logger MUST 保留可用的 `request_id`、`trace_id` 和 `span_id`
- **AND** 缺省构造 MUST 使用局部 nop logger 或 fail-fast，MUST NOT 安装或读取可变进程级默认 logger
