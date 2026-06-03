## ADDED Requirements

### Requirement: User query data access uses repository abstraction with PostgreSQL implementation boundary
用户资料查询能力 SHALL 通过根 `repository.UserRepository` 抽象读取用户资料，具体 Ent/PostgreSQL 查询实现 MUST 位于 `user-services/internal/repository/postgres` 包。根 `repository` 包 MUST NOT 依赖 `repository/postgres`，查询 API 的路由、认证要求、错误映射和响应内容 MUST 保持不变。

#### Scenario: Query service depends on repository abstraction
- **Given** 用户资料查询 service 需要按外部 UUID 读取用户
- **WHEN** service 调用数据访问层
- **THEN** service MUST 依赖 `repository.UserRepository`
- **THEN** service MUST NOT 直接依赖 `repository/postgres` 或 Ent 查询实现类型

#### Scenario: PostgreSQL implementation preserves query behavior
- **Given** `repository/postgres` 提供 `UserRepository` 的 Ent/PostgreSQL 实现
- **WHEN** 调用方请求 `GET /api/v1/users/:user_id`
- **THEN** 系统 MUST 继续只返回未软删除用户
- **THEN** Ent not found MUST 继续转换为用户领域 `ErrUserNotFound`
- **THEN** HTTP 响应路径、响应信封和公开字段 MUST 与迁移前保持一致
