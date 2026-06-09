## MODIFIED Requirements

### Requirement: User creation data access uses repository abstraction with PostgreSQL implementation boundary
用户创建能力 SHALL 通过 app service 消费侧声明的用户资料持久化端口创建用户，具体 Ent/PostgreSQL 写入和查询实现 MUST 位于 `user-services/internal/features/user/infra/postgres` 包。用户创建 service 的创建输入模型 MUST 由消费侧声明，根 `repository` 包 MUST NOT 定义用户资料创建 service 消费的接口或输入模型。

#### Scenario: Create flow remains layered
- **Given** 用户创建 controller 已完成请求绑定和校验
- **When** service 编排用户创建
- **Then** service MUST 通过 `user-services/internal/features/user/app` 声明的用户资料持久化端口调用创建
- **Then** service MUST NOT 调用 `ExistsByUsername` 或等价用户名存在性预查
- **Then** service MUST NOT 直接调用 Ent client 或 `features/user/infra/postgres` 私有实现类型

#### Scenario: Create API remains compatible after implementation split
- **Given** `features/user/infra/postgres` 承载 Ent/PostgreSQL 用户创建实现
- **When** 调用方提交有效用户创建请求
- **Then** 系统 MUST 保持现有成功响应信封和用户响应字段
- **Then** 用户名冲突、校验失败和持久化错误的公开错误语义 MUST 与迁移前保持一致

### Requirement: User creation depends on profile repository interface

用户资料创建服务 SHALL 仅依赖由消费方声明的用户资料相关仓储接口创建用户资料。该接口 MUST 覆盖创建用户、按外部用户 ID 查询和用户列表查询等用户资料服务实际消费的方法，MUST NOT 要求创建服务依赖认证凭证更新、按用户名认证读取或 token version 递增能力。创建 API 的请求校验、密码 hash、用户名唯一性冲突映射、响应信封和公开字段 MUST 保持不变。

#### Scenario: Create service declares minimum repository dependency
- **Given** 用户创建 service 需要持久化新用户记录
- **When** service 构造函数声明仓储依赖
- **Then** service MUST 依赖由 `user-services/internal/features/user/app` 声明的用户资料仓储接口
- **Then** service MUST NOT 依赖包含认证凭证和 token version 操作的完整用户仓储大接口
- **Then** service MUST NOT 直接调用 Ent client 或 `features/user/infra/postgres` 私有实现类型

#### Scenario: Create API behavior remains compatible
- **Given** PostgreSQL 用户仓储实现通过用户资料仓储接口提供创建能力
- **When** 调用方提交有效用户创建请求
- **Then** 系统 MUST 继续创建用户并返回现有成功响应信封和用户响应字段
- **Then** 用户名冲突、校验失败和持久化错误的公开错误语义 MUST 与迁移前保持一致
- **Then** 创建响应 MUST NOT 包含 `password`、`password_hash`、`token_version`、内部 `id` 或 `deleted_at`

#### Scenario: Create service test fake stays focused
- **Given** 单元测试只验证用户创建流程
- **When** 测试构造用户创建 service 的仓储替身
- **Then** 测试替身 MUST 只需要实现用户资料创建和资料读取相关方法
- **Then** 测试替身 MUST NOT 为认证凭证更新、按用户名认证读取或 token version 递增提供无关空实现
