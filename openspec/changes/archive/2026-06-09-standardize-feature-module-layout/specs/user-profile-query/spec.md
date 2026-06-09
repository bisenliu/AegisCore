## MODIFIED Requirements

### Requirement: User query data access uses repository abstraction with PostgreSQL implementation boundary
用户资料查询能力 SHALL 通过 app service 消费侧声明的用户资料持久化端口读取用户资料，具体 Ent/PostgreSQL 查询实现 MUST 位于 `user-services/internal/features/user/infra/postgres` 包。实现 MUST NOT 新增根 `repository` 包定义用户资料查询 app service 消费的接口或依赖具体 infra 实现，查询 API 的路由、认证要求、错误映射和响应内容 MUST 保持不变。

#### Scenario: Query service depends on consumer-owned repository port
- **Given** 用户资料查询 service 需要按外部 UUID 读取用户
- **When** service 构造函数声明仓储依赖
- **Then** service MUST 依赖由 `user-services/internal/features/user/app` 声明的用户资料持久化端口
- **Then** service MUST NOT 依赖根 `repository` 包声明的用户资料接口
- **Then** service MUST NOT 直接依赖 `features/user/infra/postgres` 或 Ent 查询实现类型

#### Scenario: PostgreSQL implementation preserves query behavior
- **Given** `features/user/infra/postgres` 提供用户资料持久化端口的 Ent/PostgreSQL 实现
- **When** 调用方请求 `GET /api/v1/users/:user_id`
- **Then** 系统 MUST 继续只返回未软删除用户
- **Then** Ent not found MUST 继续转换为用户领域 `ErrUserNotFound`
- **Then** HTTP 响应路径、响应信封和公开字段 MUST 与迁移前保持一致

### Requirement: Query service uses domain user model
用户资料查询能力 SHALL 通过 `userapp.UserProfileStore` 获取用户领域实体，并将领域实体映射为查询响应 DTO。Service 层 MUST NOT 直接依赖 Ent 用户模型或 Ent 查询实现类型，查询 API 的认证要求、参数校验、错误映射和响应内容 MUST 保持不变。

#### Scenario: Query maps domain user to response
- **Given** PostgreSQL repository 读取到未软删除用户并返回用户领域实体
- **When** `UserService.GetUserByID` 处理查询结果
- **Then** Service MUST 将用户领域实体映射为 `userapi.UserResponse`
- **Then** 响应 MUST 继续包含 `user_id`、`nickname`、`username`、`status`、`created_at` 和 `updated_at`
- **Then** 响应 MUST NOT 包含 `password_hash`、内部 `id` 或 `deleted_at`

#### Scenario: Query service remains independent of Ent
- **Given** 用户资料查询 Service 编译
- **When** 检查 Service 层依赖
- **Then** `user-services/internal/features/user/app` MUST NOT 为用户资料查询导入 `github.com/aegiscore/user-services/ent`
- **Then** Ent 查询和 Ent 到 Domain 映射 MUST 位于 `user-services/internal/features/user/infra/postgres`

#### Scenario: Query not found mapping remains unchanged
- **Given** PostgreSQL repository 未找到未软删除用户
- **When** Store 返回 `userdomain.ErrUserNotFound`
- **Then** Service MUST 继续将该领域错误映射为用户不存在响应
- **Then** HTTP 404 响应信封和公开错误消息 MUST 与现有查询能力保持一致

### Requirement: User profile query depends on profile repository interface

用户资料查询服务 SHALL 仅依赖由消费方声明的用户资料相关仓储接口读取用户资料。该接口 MUST 覆盖按外部用户 ID 查询和用户列表查询所需方法，MUST NOT 要求用户资料查询服务依赖认证凭证更新、按用户名认证读取或 token version 递增能力。查询 API 的认证要求、参数校验、错误映射、响应信封和公开字段 MUST 保持不变。

#### Scenario: Query service declares minimum repository dependency
- **Given** 用户资料查询 service 需要按外部 UUID 读取用户
- **When** service 构造函数声明仓储依赖
- **Then** service MUST 依赖由 `user-services/internal/features/user/app` 声明的用户资料仓储接口
- **Then** service MUST NOT 依赖包含认证凭证和 token version 操作的完整用户仓储大接口
- **Then** service MUST NOT 直接依赖 `features/user/infra/postgres` 或 Ent 查询实现类型

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
