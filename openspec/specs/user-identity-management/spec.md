## Purpose

定义 user-service 的用户资料创建、查询、列表、状态约束和 HTTP 边界，保证用户身份数据可被认证和 RBAC 能力稳定复用。

## Requirements

### Requirement: 用户资料创建与状态约束

系统 MUST 校验用户名、昵称、密码和状态并原子创建用户资料与认证凭证。系统 MUST 使用 `shared/identity` 的统一状态模型约束资料、认证和 RBAC 流程，不得通过旧状态别名放宽约束。

#### Scenario: 创建用户成功或拒绝

- **WHEN** 已授权调用方通过 `POST /api/v1/users` 提交合法用户名、昵称、密码和受支持状态
- **THEN** 系统 MUST 原子创建用户资料和认证凭证，并通过共享成功 envelope 返回可用于查询和 RBAC 绑定的 UUID `user_id`
- **WHEN** 用户名为空或格式非法、状态不受支持，或密码不满足认证策略
- **THEN** 系统 MUST 拒绝创建并返回一致校验错误，MUST NOT 写入部分用户资料或凭证

#### Scenario: 用户名冲突

- **WHEN** 创建流程返回 `identity.ErrUserAlreadyExists`
- **THEN** 共享错误渲染 MUST 返回 `409 Conflict`、共享冲突业务 code 和当前用户已存在公开文案
- **AND** 该错误在包装前后 MUST 保持正确的 `errors.Is` 匹配语义

#### Scenario: 用户状态转换与拒绝

- **WHEN** 用户状态为 `UserStatusNormal`
- **THEN** 系统 MUST 在认证和授权条件满足时允许其访问受保护资源
- **WHEN** `UserStatusMustChangePassword` 用户完成受限改密流程
- **THEN** 系统 MUST 将状态转为 `UserStatusNormal`，转换前 MUST NOT 允许普通登录会话访问受保护资源
- **WHEN** 状态为 disabled 等不可用状态，或输入包含未定义状态值
- **THEN** 系统 MUST 拒绝相应认证、授权或资料写入流程

### Requirement: 用户资料查询、列表与 HTTP 契约

系统 MUST 仅通过 `/api/v1/users` 路由图提供用户创建、按 UUID 查询和分页列表接口，并在业务处理前执行 bearer token 认证与 RBAC 授权。稳定查询模式所需索引 MUST 由 Ent schema 和 Atlas migration 管理，且索引调整不得改变业务和错误语义。

#### Scenario: 查询用户资料

- **WHEN** 已授权调用方通过 `GET /api/v1/users/:id` 查询存在的用户
- **THEN** 系统 MUST 返回 UUID `user_id`、用户名、昵称、状态和创建更新时间等公开字段，MUST NOT 暴露内部数字数据库 ID
- **WHEN** 查询返回 `identity.ErrUserNotFound`
- **THEN** 共享错误渲染 MUST 返回 `404 Not Found`、共享未找到业务 code 和当前用户不存在公开文案，而不是空成功响应
- **AND** 该错误在包装前后 MUST 保持正确的 `errors.Is` 匹配语义

#### Scenario: 分页列表与查询索引

- **WHEN** 已授权调用方通过 `GET /api/v1/users` 提交有效分页、状态或 cursor 参数
- **THEN** 系统 MUST 按软删除过滤、状态过滤和 `user_id` keyset 顺序返回用户列表及共享 pagination 信息，并拒绝无效分页参数
- **AND** Ent schema 和 Atlas migration MUST 为列表过滤、`user_id` cursor 排序和昵称查询提供索引，昵称索引 MUST NOT 依赖 PostgreSQL GIN、`gin_trgm_ops` 或插件
- **AND** 索引调整 MUST NOT 改变用户名唯一性、软删除、状态、排序、响应字段或错误语义

#### Scenario: 路由保护与错误渲染

- **WHEN** 调用方访问用户接口
- **THEN** 未认证或无权限请求 MUST 在业务处理前被拒绝；已授权请求 MUST 执行对应流程并返回共享 response envelope
- **AND** 系统 MUST NOT 暴露 `/api/users`、`/v1/users` 或其他旧路径别名
- **WHEN** 用户 use case 返回业务错误
- **THEN** controller MUST 通过 `response.Fail(c, err)` 渲染，HTTP transport MUST NOT 维护用户专用 sentinel-to-HTTP mapper

### Requirement: 用户写接口请求体容量边界

系统 MUST 对 `/api/v1/users` 下需要 JSON 请求体的用户写接口执行请求体字节上限检查。超限请求 MUST 在输入 preparer、字段校验、授权后业务 use case 或持久化写入前被拒绝，并 MUST 返回 `413 Payload Too Large` 与统一错误 envelope。

#### Scenario: 创建用户拒绝超限请求体

- **WHEN** 已认证且已授权调用方向 `POST /api/v1/users` 提交超过配置上限的 JSON 请求体
- **THEN** 系统 MUST 返回 `413 Payload Too Large`
- **AND** 系统 MUST NOT 创建用户资料、认证凭证或任何部分持久化数据

#### Scenario: 用户写接口尾随 JSON 超限

- **WHEN** 创建用户请求体首个 JSON 文档合法，但其后追加的尾随 JSON 使总请求体超过配置上限
- **THEN** 系统 MUST 返回 `413 Payload Too Large`
- **AND** 用户 create use case MUST NOT 被调用

#### Scenario: 查询接口不引入请求体限制副作用

- **WHEN** 调用方访问 `GET /api/v1/users/:id` 或 `GET /api/v1/users` 并仅提交合法路径或 query 参数
- **THEN** 请求体容量边界 MUST NOT 改变既有查询、分页、认证、授权和错误渲染语义

### Requirement: 用户 feature 架构与依赖边界

系统 MUST 将用户资料能力按 feature-local 的 application、domain、transport 和 infrastructure 分层；application port MUST 由消费侧拥有，HTTP DTO 与输入处理 MUST 留在 transport，Ent 访问和 predicate 构造 MUST 留在 PostgreSQL adapter。生产依赖 API MUST framework-neutral，服务级 DI 和 named resource 选择仅存在于 composition 边界。

#### Scenario: 分层、输入与持久化边界

- **WHEN** 新增或维护用户写侧、查询、列表或 HTTP 输入处理
- **THEN** 写侧编排 MUST 位于 `application/command`，读侧编排 MUST 位于 `application/query`，application MUST 通过自身最小 port 访问基础设施，MUST NOT 导入 HTTP DTO 或 Ent predicate 包
- **AND** request/response DTO MUST 位于 `transport/http`，裁剪和归一化 MUST 在 feature-local input preparer 完成；preparer MUST NOT 查询 store、调用 use case、授权或写 HTTP 响应
- **AND** Ent 查询和 predicate 构造 MUST 留在 `infrastructure/postgres`

#### Scenario: Framework-neutral 构造、装配与日志

- **WHEN** 用户 feature 的生产 constructor 暴露依赖或组件记录正式业务日志
- **THEN** constructor MUST 使用显式普通 Go 参数和消费侧 application port，生产包 MUST NOT 导入 `go.uber.org/fx`、`go.uber.org/dig` 或声明其 DI marker
- **AND** named resource metadata MUST 留在 Fx composition 边界，不得泄漏到 feature constructor API
- **AND** logger MUST 显式注入或来自 request context，request logger MUST 保留 `request_id`、`trace_id` 和 `span_id`
- **AND** 缺省构造 MUST 使用局部 nop logger 或 fail-fast，MUST NOT 安装或读取可变进程级默认 logger
