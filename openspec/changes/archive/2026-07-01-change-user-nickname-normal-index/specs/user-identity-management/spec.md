## MODIFIED Requirements

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
