## ADDED Requirements

### Requirement: User creation data access uses repository abstraction with PostgreSQL implementation boundary
用户创建能力 SHALL 通过根 `repository.UserRepository` 抽象创建用户并检查用户名唯一性，具体 Ent/PostgreSQL 写入和查询实现 MUST 位于 `user-services/internal/repository/postgres` 包。根 `repository` 包 MUST 保留创建输入类型并 MUST NOT 依赖具体实现包。

#### Scenario: Create flow remains layered
- **Given** 用户创建 controller 已完成请求绑定和校验
- **WHEN** service 编排用户创建
- **THEN** service MUST 通过 `repository.UserRepository` 调用创建和唯一性检查
- **THEN** service MUST NOT 直接调用 Ent client 或 `repository/postgres` 私有实现类型

#### Scenario: Create API remains compatible after implementation split
- **Given** `repository/postgres` 承载 Ent/PostgreSQL 用户创建实现
- **WHEN** 调用方提交有效用户创建请求
- **THEN** 系统 MUST 保持现有成功响应信封和用户响应字段
- **THEN** 用户名冲突、校验失败和持久化错误的公开错误语义 MUST 与迁移前保持一致
