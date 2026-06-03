## ADDED Requirements

### Requirement: Runtime composes concrete repository implementations at the bootstrap boundary
HTTP 服务运行时 SHALL 在 `user-services/internal/bootstrap` 组合根中装配具体 repository 实现。用户服务启动时 MUST 通过 `repository/postgres` provider 提供 `repository.UserRepository`，并通过 `repository/redis` provider 提供 `repository.AuthSessionRepository`，同时保持现有 `user_db` Ent client、`cache_redis` Redis client 和 auth 配置依赖不变。

#### Scenario: Bootstrap provides PostgreSQL user repository
- **Given** Fx app 装配用户服务依赖
- **WHEN** bootstrap 创建用户仓储 provider
- **THEN** bootstrap MUST 使用 `postgres.NewUserRepository`
- **THEN** provider MUST 注入具名 `user_db` Ent client
- **THEN** 下游 service MUST 接收 `repository.UserRepository` 抽象

#### Scenario: Bootstrap provides Redis auth session repository
- **Given** Fx app 装配用户服务依赖
- **WHEN** bootstrap 创建认证会话仓储 provider
- **THEN** bootstrap MUST 使用 `redis.NewAuthSessionRepository`
- **THEN** provider MUST 注入具名 `cache_redis` Redis client、`repository.UserRepository` 和 auth 配置
- **THEN** 下游 auth service 和认证中间件 MUST 接收 `repository.AuthSessionRepository` 抽象

#### Scenario: Startup dependencies remain unchanged
- **Given** 用户服务通过 CLI 启动
- **WHEN** Fx app 初始化 runtime 依赖
- **THEN** 系统 MUST 继续只初始化自身声明的 `cache_redis`、`user_db` 和 `common_db` 运行时依赖
- **THEN** 系统 MUST NOT 因 repository 实现分包新增 Redis、PostgreSQL、Ent client 或 HTTP 路由依赖
