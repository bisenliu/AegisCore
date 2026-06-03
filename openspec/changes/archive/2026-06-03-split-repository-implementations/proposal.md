## Why

当前 `user-services` 的 repository 边界混合了接口抽象、Ent/PostgreSQL 实现和 Redis session 存储实现，导致 service 层与具体存储细节的分离不够清晰。现在需要按存储类型拆分实现包，让根 `repository` 包只表达领域数据访问契约，便于后续维护、测试和替换实现。

## What Changes

- 将 `UserRepository` 接口和 input 类型保留在 `user-services/internal/repository/user_repository.go`，移除其中的 Ent/PostgreSQL 具体实现。
- 新增 `user-services/internal/repository/postgres/user_repository.go`，承载当前基于 Ent 的 `UserRepository` PostgreSQL 实现与 Fx provider。
- 新增 `user-services/internal/repository/auth_session_repository.go`，将认证会话抽象从 service 层迁移为 `AuthSessionRepository`、`AuthSession` 和 repository 级错误。
- 新增 `user-services/internal/repository/redis/auth_session_repository.go`，承载当前 Redis 会话存储实现与 Fx provider。
- 删除 `user-services/internal/service/session_store.go`，并同步更新 auth service、bootstrap 和测试引用新的 repository 抽象。
- 迁移相关测试到 `repository/postgres` 和 `repository/redis` 包，保持服务测试通过 stub 依赖根 repository 接口。
- 非目标：本次不将 `UserRepository` 返回值从 `*ent.User` 改为 domain DTO，也不调整 HTTP API、错误码、配置格式、Ent schema 或 Atlas migration。

## Capabilities

### New Capabilities


### Modified Capabilities
- `user-profile-query`: 用户资料查询继续依赖 `UserRepository` 抽象，但其 PostgreSQL/Ent 实现迁移到 `repository/postgres` 包。
- `user-profile-create`: 用户创建与凭证更新相关的数据访问继续使用 `UserRepository` 抽象，但其 PostgreSQL/Ent 实现迁移到 `repository/postgres` 包。
- `user-list-query`: 用户列表查询继续使用同一 `UserRepository` 抽象，但具体查询实现迁移到 `repository/postgres` 包。
- `user-session-control`: 认证会话存储从 service 层 `SessionStore` 迁移为 repository 层 `AuthSessionRepository`，Redis 实现迁移到 `repository/redis` 包。
- `http-service-runtime`: Fx 装配改为提供 `postgres.NewUserRepository` 和 `redis.NewAuthSessionRepository`，保持 `user_db` 与 `cache_redis` 具名依赖不变。

## Impact

- 影响代码：`user-services/internal/repository/`、`user-services/internal/service/auth_service.go`、`user-services/internal/bootstrap/bootstrap.go` 及相关测试。
- API 兼容性：不改变 HTTP 路由、请求/响应格式、错误码或认证 token 语义。
- 配置兼容性：继续使用 `postgres.user_db`、`redis.cache_redis` 和现有 auth TTL 配置，不新增配置项。
- 数据模型兼容性：不修改 Ent schema、生成代码或 Atlas migration。
- 依赖方向：`service -> repository`，`repository/postgres -> repository + ent`，`repository/redis -> repository + redis client`，根 `repository` 包不得依赖具体实现包。
