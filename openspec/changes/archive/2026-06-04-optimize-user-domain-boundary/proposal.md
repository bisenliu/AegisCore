## Why

`user-services` 的用户领域层目前过薄，`repository.UserRepository` 直接返回 Ent 生成模型，导致 `UserService` 和 `AuthService` 依赖持久化结构并分散承载用户状态规则。随着用户服务后续扩展为更大的服务，需要尽早收敛用户领域模型边界，避免认证、资料、会话等业务规则继续和 Ent schema 耦合。

## What Changes

- 引入轻量 `domain.User` 实体，承载用户对外身份、资料字段、认证凭据摘要、状态、token version 和时间戳等用户聚合核心数据。
- 将用户状态规则从 Service 中的字段判断收敛到 `domain.User` 或 `domain.UserStatus` 方法，例如可登录、是否需要改密、是否允许完成改密状态流转。
- 修改根 `repository.UserRepository` 抽象，使用户读取、创建和列表方法返回 `domain.User`，不再向 Service 泄漏 `ent.User`。
- 将 Ent 到 Domain 的转换限制在 `internal/repository/postgres` 实现内，保持 Ent/PostgreSQL 作为持久化细节。
- 更新 `UserService` 和 `AuthService` 去除 Ent import，继续负责应用编排、密码 hash/verify、JWT、Redis session 和响应错误映射。
- 保持现有 HTTP API、响应信封、错误码、数据库 schema、Atlas migration 和 Ent 生成代码兼容。
- 非目标：不引入复杂聚合体系、领域服务拆分、事务框架、数据库 schema 变更或新的用户业务能力。

## Capabilities

### New Capabilities

- `user-domain-boundary`: 定义用户领域模型、Repository 抽象与 Ent/PostgreSQL 持久化模型之间的边界契约。

### Modified Capabilities

- `user-profile-query`: 查询用户资料时 Service 必须依赖领域用户模型，不得直接依赖 Ent 用户模型。
- `user-profile-create`: 创建用户后 Repository 必须返回领域用户模型，Service 保持创建编排和冲突错误映射。
- `user-list-query`: 列表查询必须通过领域用户模型向 Service 返回用户资料集合，不泄漏 Ent 列表模型。
- `user-session-control`: 登录、改密、退出全部设备等认证会话流程必须通过领域用户模型读取用户状态、密码哈希和 token version。

## Impact

- 影响代码：`user-services/internal/domain/`、`user-services/internal/repository/user_repository.go`、`user-services/internal/repository/postgres/user_repository.go`、`user-services/internal/service/user_service.go`、`user-services/internal/service/auth_service.go` 及相关测试。
- API 兼容性：不改变 `GET /api/v1/users/:user_id`、`POST /api/v1/users`、用户列表和认证会话相关 HTTP 路径、响应 JSON 字段、响应信封或错误码。
- 数据兼容性：不修改 Ent schema、PostgreSQL 表结构、Atlas migration 或 Redis key 格式。
- 依赖影响：不新增外部依赖；Ent 继续作为 PostgreSQL repository 的内部持久化实现。
