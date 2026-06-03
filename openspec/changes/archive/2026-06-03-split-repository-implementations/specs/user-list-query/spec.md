## ADDED Requirements

### Requirement: User list query uses repository abstraction with PostgreSQL implementation boundary
用户列表查询能力 SHALL 通过根 `repository.UserRepository` 抽象执行分页、过滤和排序相关数据访问，具体 Ent/PostgreSQL predicate 与查询实现 MUST 位于 `user-services/internal/repository/postgres` 包。根 `repository` 包 MUST 保留列表查询输入类型并 MUST NOT 依赖具体实现包。

#### Scenario: List service remains decoupled from Ent query implementation
- **Given** 用户列表 service 需要查询用户列表
- **WHEN** service 调用数据访问层
- **THEN** service MUST 通过 `repository.UserRepository.ListUsers` 提交列表查询输入
- **THEN** service MUST NOT 直接引用 Ent predicate helper 或 `repository/postgres` 私有实现类型

#### Scenario: List query behavior remains compatible
- **Given** `repository/postgres` 承载 Ent/PostgreSQL 列表查询实现
- **WHEN** 调用方请求用户列表 API
- **THEN** 系统 MUST 继续按现有分页、过滤、排序和未软删除条件返回用户列表
- **THEN** HTTP 响应信封、公开字段和错误语义 MUST 与迁移前保持一致
