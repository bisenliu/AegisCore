## Context

`user-services` 当前已经有 controller、service、repository 分层，但用户 repository 抽象返回 `*ent.User`，导致 `UserService` 和 `AuthService` 直接依赖 Ent 生成模型。用户领域层目前主要包含 `UserStatus` 和少量领域错误，用户状态规则、认证资料读取、DTO 映射和持久化模型字段访问散落在 Service 中。

本变更面向用户服务后续扩展，目标是在不改变 API、数据库 schema、Redis key 和运行时配置的前提下，建立用户领域模型和持久化模型之间的稳定边界。Ent 继续作为 PostgreSQL repository 的内部实现；Service 继续负责编排密码 hash/verify、JWT 签发校验、Redis session、日志和响应错误映射。

受影响包：

- `user-services/internal/domain`: 新增用户实体与状态规则方法。
- `user-services/internal/repository`: 调整用户 repository 抽象返回领域模型。
- `user-services/internal/repository/postgres`: 保持 Ent 查询写入实现，新增 Ent 到 Domain mapper。
- `user-services/internal/service`: 去除 Ent import，改为使用 `domain.User`。
- `user-services/internal/*_test.go`: 更新测试 stub 和断言数据构造。

## Goals / Non-Goals

**Goals:**

- 引入轻量 `domain.User`，表达用户资料、认证凭据摘要、状态和 token version。
- 让根 `repository.UserRepository` 面向领域模型返回数据，避免 Ent 泄漏到 Service。
- 将 Ent 到 Domain 的转换限制在 `internal/repository/postgres`。
- 让 `UserService` 和 `AuthService` 基于领域方法判断可登录、是否需要改密、是否允许完成改密。
- 保持 controller/service/repository 分层职责不变。
- 保持现有 HTTP 响应契约、错误码、Swagger 可见字段、数据库 schema 和 Redis 行为不变。

**Non-Goals:**

- 不新增用户业务功能，例如用户更新、禁用、删除、权限控制或风控锁定。
- 不修改 Ent schema、Atlas migration、PostgreSQL 表结构或 Redis key 格式。
- 不把密码 hash/verify、JWT、Redis session、`common/response` 错误映射移入 domain。
- 不引入复杂聚合框架、领域事件、Unit of Work、事务管理器或独立领域服务。
- 不手写 `user-services/ent/` 下的生成代码。

## Decisions

### 1. 引入轻量 `domain.User`，而不是完整 DDD 聚合体系

`domain.User` 包含当前 Service 已经依赖的用户核心字段：内部 ID、外部 `user_id`、`nickname`、`username`、`password_hash`、`status`、`token_version`、`created_at`、`updated_at`。字段命名使用 Go 领域模型表达，持久化字段映射由 repository/postgres 负责。

领域方法优先覆盖当前已有规则：

- `CanLogin()` 判断 `status=100` 是否允许普通登录。
- `RequiresPasswordChange()` 判断 `status=300` 是否需要签发受限改密凭据。
- `CanChangePassword()` 或等价方法判断当前用户是否允许通过改密流程转为正常状态。
- 可选的 `ChangePassword(passwordHash string)` 表达状态从必须改密转为正常并更新凭据摘要的领域状态变化。

选择该方案是因为当前业务规则规模还不需要拆出多个值对象或领域服务，但已经需要一个稳定实体来避免 Service 继续依赖 Ent 字段。

替代方案：保持只有 `UserStatus` 方法。该方案改动更小，但无法解决 Repository 返回 Ent 和 Service 依赖 Ent 的核心边界问题。

### 2. Repository 抽象返回 `domain.User`，Postgres 实现内部使用 Ent

根 `repository.UserRepository` 的用户数据方法改为返回领域模型：

- `Create(ctx, input) (*domain.User, error)`
- `GetByUsername(ctx, username) (*domain.User, error)`
- `GetByUserID(ctx, userID) (*domain.User, error)`
- `ListUsers(ctx, input) ([]domain.User, int, error)` 或 `[]*domain.User`

`GetTokenVersion`、`IncrementTokenVersion`、`UpdateCredentials` 可保持现有标量返回契约，因为这些方法表达的是持久化更新结果和认证会话需要的版本值，不要求返回完整用户实体。

`internal/repository/postgres` 保持导入 Ent，并新增 mapper 将 `*ent.User` 转换为 `domain.User`。Ent not found 和 constraint error 继续转换为 `domain.ErrUserNotFound`、`domain.ErrUserAlreadyExists`，repository 不构造 `common/response` 应用错误。

替代方案：在 Service 中做 Ent 到 Domain mapper。该方案仍要求 Service import Ent，不满足边界目标。

### 3. Service 保留应用编排职责，不下沉基础设施逻辑

`UserService` 继续负责创建用户流程中的用户名唯一性检查、密码 hash、UUIDv7 生成、调用 repository 创建、领域错误到统一响应错误的映射和 DTO 输出。

`AuthService` 继续负责登录、改密、刷新、退出当前设备和退出全部设备编排，包括密码校验、JWT 签发解析、Redis session 操作、token version 校验和响应错误映射。它只将用户状态判断改为调用 `domain.User` 方法。

这样可以保持已有规格中对 Service 的安全边界要求：Validation 层不执行认证业务校验，Domain 不依赖 JWT/Redis/HTTP response。

替代方案：把密码验证、token 策略或 session 创建移入 Domain。该方案会让领域层依赖基础设施和响应契约，不适合当前架构。

### 4. API 和数据模型保持兼容

本变更只调整 Go 内部分层边界，不改变 controller 入参、DTO、响应 envelope、错误码、日志字段、配置项、Ent schema、migration 或 Redis key。Swagger 可见字段保持不变，用户响应仍不得包含 `password_hash`、`deleted_at` 或内部 `id`。

## Risks / Trade-offs

- [Risk] 增加 Ent 到 Domain 映射可能遗漏字段。→ Mitigation: mapper 覆盖 Service 当前依赖字段，并通过 service/repository 测试验证创建、查询、列表、登录和改密流程。
- [Risk] `ChangePassword()` 如果同时在 domain 和 repository update input 中表达 token version 递增，可能造成语义重复。→ Mitigation: 当前 repository 仍负责数据库原子递增并返回版本；domain 方法仅表达实体状态规则，不作为持久化计数来源，除非后续引入更完整的写模型。
- [Risk] 一次修改 Repository 返回类型会影响较多测试 stub。→ Mitigation: 分批更新编译错误，优先修复 root repository interface、postgres mapper、service tests。
- [Risk] 领域模型暴露 `PasswordHash` 可能被误用于响应。→ Mitigation: DTO mapper 继续只输出公开字段，规格明确响应不得包含密码或密码哈希。
- [Risk] 列表返回 `[]domain.User` 与单条返回 `*domain.User` 选择不一致。→ Mitigation: 实现时优先选择减少 nil 处理和拷贝成本的形式，并在 repository interface 中保持一致文档化。

## Migration Plan

1. 新增 `internal/domain/user.go`，实现 `domain.User` 和用户状态规则方法。
2. 修改 `internal/repository/user_repository.go` 返回领域模型。
3. 在 `internal/repository/postgres/user_repository.go` 添加 Ent 到 Domain mapper，并更新 Create/Get/List 返回值。
4. 更新 `UserService` 和 `AuthService` 移除 Ent import，使用领域用户模型和方法。
5. 更新单元测试中的 repository stub 和用户构造数据。
6. 运行 `go test ./...` 分别验证 `common` 和 `user-services` 模块。

Rollback 策略：该变更不包含数据迁移。如实现出现问题，可回退 Go 代码到 Ent 返回模型；数据库和 Redis 数据无需回滚。

## Open Questions

- `ListUsers` 返回 `[]domain.User` 还是 `[]*domain.User` 由实现时按现有代码简洁性决定，但必须保证 Service 不依赖 Ent。
- `domain.User.ChangePassword()` 是否在首轮实现中落地为状态转换方法，还是先只提供 `CanChangePassword()`，取决于避免和 repository 原子更新契约重复的实现复杂度。
