## Purpose

定义 user-service 的用户资料创建、查询、列表、状态约束和 HTTP 边界，保证用户身份数据可被认证与 RBAC 能力稳定复用。

## Requirements

### Requirement: 用户资料创建与状态约束

系统 MUST 校验用户名、昵称、密码和状态，并原子创建用户资料与认证凭证。用户、认证和 RBAC 流程 MUST 复用 `internal/shared/identity` 的用户状态与账号生命周期错误。

#### Scenario: 创建用户成功或拒绝

- **WHEN** 已授权调用方通过 `POST /api/v1/users` 提交合法资料、密码和受支持状态
- **THEN** 系统 MUST 原子创建用户资料与认证凭证，并通过共享成功 envelope 返回 UUID `user_id`
- **WHEN** 用户名、状态或密码策略校验失败
- **THEN** 系统 MUST 返回一致校验错误，MUST NOT 写入部分资料或凭证
- **WHEN** 创建流程返回 `identity.ErrUserAlreadyExists`
- **THEN** 系统 MUST 返回 `409 Conflict` 与共享冲突 code，并保留包装前后的 `errors.Is` 语义

#### Scenario: 用户状态约束

- **WHEN** 用户状态为 `UserStatusNormal`
- **THEN** 系统 MUST 在认证和授权条件满足时允许访问受保护资源
- **WHEN** 用户状态为 `UserStatusMustChangePassword`
- **THEN** 系统 MUST 只允许受限改密流程；成功改密后状态 MUST 转为 `UserStatusNormal`
- **WHEN** 用户为 disabled、已删除或状态值未定义
- **THEN** 系统 MUST 拒绝相应认证、授权或资料写入流程

### Requirement: 用户资料查询、列表与 HTTP 边界

系统 MUST 仅通过 `/api/v1/users` 提供创建、按 UUID 查询和分页列表，并在业务处理前执行认证与 RBAC 授权。写接口 MUST 执行请求体容量检查；所有响应和错误 MUST 使用共享 envelope。

#### Scenario: 查询与不存在错误

- **WHEN** 已授权调用方通过 `GET /api/v1/users/:user_id` 查询存在用户
- **THEN** 系统 MUST 返回 UUID `user_id`、用户名、昵称、状态和时间字段，MUST NOT 暴露内部数字 ID
- **WHEN** 查询返回 `identity.ErrUserNotFound`
- **THEN** 系统 MUST 返回 `404 Not Found` 与共享未找到 code，MUST NOT 返回空成功响应

#### Scenario: 分页列表与查询索引

- **WHEN** 已授权调用方通过 `GET /api/v1/users` 提交有效分页、状态或 cursor 参数
- **THEN** 系统 MUST 按软删除过滤、状态过滤和 `user_id` keyset 顺序返回列表与共享 pagination，并拒绝无效参数
- **AND** Ent schema 与 Atlas migration MUST 为稳定过滤和排序提供索引；昵称 substring 查询 MUST 使用 PostgreSQL `pg_trgm` 的 GIN `gin_trgm_ops` 索引
- **AND** 索引调整 MUST NOT 改变用户名唯一性、软删除、状态、排序、响应或错误语义

#### Scenario: 路由保护与错误渲染

- **WHEN** 调用方访问用户接口
- **THEN** 未认证或无权限请求 MUST 在 use case 前被拒绝，已授权请求 MUST 返回共享 response envelope
- **AND** 系统 MUST NOT 暴露 `/api/users`、`/v1/users` 或其他旧路径别名
- **WHEN** 用户 use case 返回业务错误
- **THEN** controller MUST 通过 `response.Fail(c, err)` 渲染，MUST NOT 维护 feature 专用错误 mapper

#### Scenario: 写请求体容量边界

- **WHEN** `POST /api/v1/users` 的固定长度、chunked 或含尾随 JSON 的总请求体超过配置上限
- **THEN** 系统 MUST 在 input preparer、use case 和持久化前返回 `413 Payload Too Large`
- **AND** 系统 MUST NOT 创建用户资料、凭证或其他部分数据
- **WHEN** 调用方通过 GET 接口仅提交合法路径或 query 参数
- **THEN** 请求体容量边界 MUST NOT 改变查询、分页、认证、授权或错误语义

### Requirement: 用户 feature 架构与依赖边界

用户能力 MUST 按 feature-local 的 domain、application、transport 和 infrastructure 分层。application port MUST 由消费侧拥有；HTTP DTO 与输入处理 MUST 留在 transport；Ent 访问和 predicate 构造 MUST 留在 PostgreSQL adapter；服务级 DI 与 named resource 选择 MUST 只存在于 composition 边界。

#### Scenario: 分层、输入与持久化边界

- **WHEN** 新增或维护用户写侧、查询、列表或 HTTP 输入处理
- **THEN** 写侧编排 MUST 位于 `application/command`，读侧编排 MUST 位于 `application/query`，application MUST 只通过自身最小 port 访问基础设施
- **AND** request/response DTO 与 input preparer MUST 位于 `transport/http`；preparer MUST NOT 查询 store、调用 use case、执行授权或写 HTTP 响应
- **AND** application MUST NOT 导入 HTTP DTO 或 Ent predicate 包，Ent 查询与 predicate 构造 MUST 留在 `infrastructure/postgres`

#### Scenario: Framework-neutral 构造与日志

- **WHEN** 生产 constructor 暴露依赖或组件记录业务日志
- **THEN** constructor MUST 使用普通 Go 参数与消费侧 port，生产包 MUST NOT 导入 Fx、Dig 或声明 DI marker
- **AND** named resource metadata MUST 留在 composition，logger MUST 显式注入或来自 request context
- **AND** 日志 message MUST 使用英文、字段名 MUST 使用稳定 `snake_case`，MUST NOT 记录密码哈希、token、DSN、SQL 参数或完整 Redis key
