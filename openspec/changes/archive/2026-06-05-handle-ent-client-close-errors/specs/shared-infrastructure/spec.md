## MODIFIED Requirements

### Requirement: Provide service-specific Ent clients

系统必须为用户服务基于共享 PostgreSQL 连接池创建 Ent clients。用户服务 Ent client provider 必须组织在 `user-services/internal/bootstrap` 包中，与服务侧 Redis/PostgreSQL runtime dependency wiring 保持同一启动装配边界。Fx app 停止时必须尝试关闭所有已创建的具名 Ent clients；任一 Ent client close 失败时，停止错误必须保留失败 client 的具名上下文。当多个 Ent client close 同时失败时，停止错误必须同时保留每个失败 client 的底层错误，不得因先返回第一个错误而丢失后续错误。

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

#### Scenario: Preserve named Ent close errors
- **Given** 用户服务已经创建具名 `user_db` 和 `common_db` Ent clients
- **Given** 两个 Ent clients 在 Fx app 停止时 close 均返回错误
- **When** Ent clients 停止 lifecycle 执行
- **Then** 系统必须尝试关闭两个 Ent clients
- **Then** 返回的停止错误必须包含 `user_db` close 失败的具名上下文和底层错误
- **Then** 返回的停止错误必须包含 `common_db` close 失败的具名上下文和底层错误
