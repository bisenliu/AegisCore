## MODIFIED Requirements

### Requirement: Provide service-specific Ent clients

系统必须为用户服务基于共享 PostgreSQL 连接池创建 Ent clients。用户服务 Ent client provider 必须组织在 `user-services/internal/bootstrap` 包中，与服务侧 Redis/PostgreSQL runtime dependency wiring 保持同一启动装配边界。

#### Scenario: Create named Ent clients
- **Given** Fx 容器中存在具名 `user_db` 和 `common_db` PostgreSQL 连接池
- **When** `user-services/internal/bootstrap.NewNamedClients` 被调用
- **Then** 系统创建具名 `user_db` Ent client
- **Then** 系统创建具名 `common_db` Ent client
- **Then** Fx app 停止时关闭 Ent clients

#### Scenario: Repository receives user database client
- **Given** 用户 repository 需要访问用户数据
- **When** Fx 构造 `UserRepository`
- **Then** repository 接收具名 `user_db` Ent client，而不是直接打开数据库连接
