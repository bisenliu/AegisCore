## MODIFIED Requirements

### Requirement: Maintain user indexes for status nickname and soft delete queries
系统 MUST 审查并更新用户表索引，使常用查询条件 `username`、`nickname`、`status` 和 `deleted_at IS NULL` 能与新的字段命名和软删除语义一致。`username` MUST 使用全表唯一约束，软删除后不得释放；`nickname` MUST 仅作为可重复展示名，不得建立唯一约束。

#### Scenario: Update indexes after field rename
- **Given** 用户表索引引用旧字段 `name` 或 `active`
- **When** migration 生成或人工审查 SQL
- **Then** 新索引 MUST 引用 `nickname` 或 `status`
- **Then** migration 完成后索引定义 MUST NOT 引用 `name` 或 `active`

#### Scenario: Preserve global username uniqueness with soft delete
- **Given** 用户表需要按用户名识别创建用户唯一账号名
- **When** migration 审查用户名唯一索引
- **Then** implementation MUST 使用全表 `UNIQUE(username)` 约束
- **Then** implementation MUST NOT 使用 `WHERE deleted_at IS NULL` 或等价条件的 partial unique index 释放已软删除用户名
- **Then** migration SQL 和实现说明 MUST 记录软删除后不释放 `username` 的全局唯一策略
- **Then** repository 的创建冲突处理 MUST 与全表唯一索引语义一致

#### Scenario: Prevent duplicate lowercase usernames before adding constraint
- **Given** 目标数据库中可能存在大小写不同但小写后相同的 `username`，或软删除记录与未删除记录使用相同 `username`
- **When** 开发者审查用户名全局唯一 migration
- **Then** migration review MUST 明确冲突检测或部署前数据清理策略
- **Then** migration MUST NOT 静默创建会导致唯一约束失败或账号归属不明确的数据状态

#### Scenario: Index active user lookup paths
- **Given** 查询、列表和登录默认只访问未软删除用户
- **When** migration 审查索引
- **Then** 系统 MUST 为 `deleted_at` 相关过滤保留可审查的索引策略
- **Then** 常用 `username`、`status` 或 `nickname` 查询 MUST 不依赖已删除的旧字段索引
