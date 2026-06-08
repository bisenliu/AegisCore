## MODIFIED Requirements

### Requirement: User query data access uses repository abstraction with PostgreSQL implementation boundary
用户资料查询能力 SHALL 通过 service 消费侧声明的用户资料持久化端口读取用户资料，具体 Ent/PostgreSQL 查询实现 MUST 位于 `user-services/internal/repository/postgres` 包。根 `repository` 包 MUST NOT 定义用户资料查询 service 消费的接口或依赖 `repository/postgres`，查询 API 的路由、认证要求、错误映射和响应内容 MUST 保持不变。

#### Scenario: Query service depends on consumer-owned repository port
- **Given** 用户资料查询 service 需要按外部 UUID 读取用户
- **When** service 构造函数声明仓储依赖
- **Then** service MUST 依赖由 `user-services/internal/service` 声明的用户资料持久化端口
- **Then** service MUST NOT 依赖根 `repository` 包声明的用户资料接口
- **Then** service MUST NOT 直接依赖 `repository/postgres` 或 Ent 查询实现类型

#### Scenario: PostgreSQL implementation preserves query behavior
- **Given** `repository/postgres` 提供用户资料持久化端口的 Ent/PostgreSQL 实现
- **When** 调用方请求 `GET /api/v1/users/:user_id`
- **Then** 系统 MUST 继续只返回未软删除用户
- **Then** Ent not found MUST 继续转换为用户领域 `ErrUserNotFound`
- **Then** HTTP 响应路径、响应信封和公开字段 MUST 与迁移前保持一致

### Requirement: User profile query depends on profile repository interface

用户资料查询服务 SHALL 仅依赖由消费方声明的用户资料相关仓储接口读取用户资料。该接口 MUST 覆盖按外部用户 ID 查询和用户列表查询所需方法，MUST NOT 要求用户资料查询服务依赖认证凭证更新、按用户名认证读取或 token version 递增能力。查询 API 的认证要求、参数校验、错误映射、响应信封和公开字段 MUST 保持不变。

#### Scenario: Query service declares minimum repository dependency
- **Given** 用户资料查询 service 需要按外部 UUID 读取用户
- **When** service 构造函数声明仓储依赖
- **Then** service MUST 依赖由 `user-services/internal/service` 声明的用户资料仓储接口
- **Then** service MUST NOT 依赖包含认证凭证和 token version 操作的完整用户仓储大接口
- **Then** service MUST NOT 直接依赖 `repository/postgres` 或 Ent 查询实现类型

#### Scenario: Query API behavior remains compatible
- **Given** PostgreSQL 用户仓储实现通过用户资料仓储接口提供查询能力
- **When** 调用方请求 `GET /api/v1/users/:user_id`
- **Then** 系统 MUST 继续只返回未软删除用户
- **Then** Ent not found MUST 继续转换为用户领域 `ErrUserNotFound`
- **Then** HTTP 响应路径、响应信封、错误语义和公开字段 MUST 与迁移前保持一致

#### Scenario: Query service test fake stays focused
- **Given** 单元测试只验证按用户 ID 查询或用户列表查询逻辑
- **When** 测试构造用户资料查询 service 的仓储替身
- **Then** 测试替身 MUST 只需要实现用户资料查询相关方法
- **Then** 测试替身 MUST NOT 为认证凭证更新或 token version 递增提供无关空实现
