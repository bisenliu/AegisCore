## Context

本次重构聚焦 `user-session-control` 能力下的认证服务内部边界，主要代码位于 `user-services/internal/service/auth_service.go:38-207`，并关联 `auth_credentials.go`、`auth_tokens.go`、`auth_sessions.go` 和对应测试。

当前 `authService` 字段列表位于 `auth_service.go:38-45`，直接持有 `repo`、`jwt`、`config`、`credentials`、`tokens`、`sessions`。其中 `credentials`、`tokens`、`sessions` 已经是拆分后的 service 内子组件，但 `authService` 仍保留原始 `jwt` 字段，并在改密和退出全部设备中直接调用用户仓储。`auth_service.go:74-109` 的修改密码流程直接读取用户、校验状态、生成密码 hash、更新 credentials 并失效 token version；`auth_service.go:155-180` 的退出全部设备流程直接解析 UUID、递增数据库 `token_version`、清理 token version 缓存和删除全部 Redis 会话。

这种状态说明拆分只完成了“对象拆分”，还没有完成“职责迁移”。主服务仍然知道凭证更新的细节、用户状态恢复的细节和用户级会话吊销的底层管道。后续如果引入 MFA、登录风控、设备管理、审计日志或安全事件流水，最容易继续向 `authService` 增加 `repo` 写操作、Redis 清理、token version 策略分支和审计调用，导致认证入口从编排器退化为大型事务脚本。

本设计要求 `authService` 退化为纯工作流编排器。编排器只负责表达用例顺序、组合子组件结果和返回 DTO，不再直接拥有凭证写入、JWT 签发实现、Redis 会话细节或 token version 持久化细节。凭证所属写操作进入 `CredentialVerifier`，会话整体吊销进入 `AuthSessionManager`，token 签发和解析继续由 `AuthTokenIssuer` 承载。

## Goals / Non-Goals

**Goals:**

- 让 `CredentialVerifier` 自治负责修改密码时的凭证更新写操作，包括读取用户、校验用户仍可改密、生成新密码 hash、更新 `password_hash`、恢复 `status=100`，并把领域错误映射为现有响应语义。
- 让 `AuthSessionManager` 自治负责“吊销全部会话”的完整管道，包括 PostgreSQL `token_version` 原子递增、Redis token version 缓存清理、删除该用户全部 Refresh Token/设备会话记录。
- 让 `authService` 删除多余底层依赖，尤其是不再保存 `repository.UserRepository` 和原始 `*auth.JWTService`；如果刷新轮转配置仍需读取 `config`，应优先以更窄的策略值或 helper 方法隔离，而不是把整个配置继续作为任意底层依赖使用。
- 让修改密码和退出全部设备流程只保留高层调用顺序，不再直接操作仓储、JWT 或 Redis 细节。
- 保持现有 HTTP API、DTO、错误码、响应信封、token claims、Redis key 格式、session TTL、token version 失效语义不变。
- 形成更清晰的测试边界：`authService` 测编排，`CredentialVerifier` 测凭证读写和状态迁移，`AuthSessionManager` 测 token version 与 session 清理管道。

**Non-Goals:**

- 不新增 MFA、登录风控、设备管理、审计日志或安全事件表。
- 不修改 Ent schema、Atlas migration、Redis key 命名规则或认证中间件契约。
- 不改变 controller/service/repository 分层，不把 HTTP 解析或响应写入逻辑下沉到子组件。
- 不把 service 内组件迁移到 `common`；这些组件仍是用户服务认证会话的业务策略，不是跨服务共享基础能力。

## Decisions

### Decision 1: 子组件接管所属领域写操作，而不是只做 helper

`CredentialVerifier` 当前只做登录校验。修改密码流程中，用户状态校验、密码 hash 生成和凭证更新仍在 `authService` 内。这会让主服务同时知道登录认证、受限改密、用户状态恢复和 repository 更新契约。重构后，`CredentialVerifier` 应成为“凭证组件”，负责所有凭证相关读写策略。

建议接口：

```go
type CredentialVerifier interface {
    VerifyPassword(ctx context.Context, username string, plainPassword string) (*domain.User, error)
    ChangePassword(ctx context.Context, userID uuid.UUID, newPassword string) (*CredentialUpdateResult, error)
}

type CredentialUpdateResult struct {
    UserID       uuid.UUID
    TokenVersion int64
}
```

`ChangePassword` 的入参使用 `uuid.UUID`，因为 token 组件已经负责解析和校验外部 UUID 格式；子组件不需要重复处理字符串格式错误。返回值保留更新后的 `TokenVersion`，与 repository `UpdateCredentials` 返回结果对齐，为审计、日志或后续安全事件扩展保留明确事实来源。当前 `authService` 不一定使用该值，但测试可以断言它来自持久化结果而不是缓存。

`credentialVerifier.ChangePassword` 应接管以下逻辑：

- `repo.GetByUserID(ctx, userID)` 读取未软删除用户。
- 将 `domain.ErrUserNotFound` 映射为 `response.NotFoundError(messages.UserNotFound)`。
- 调用 `user.CanChangePassword()` 校验状态仍为必须改密；不通过时沿用现有 `response.TokenInvalidError(messages.MissingSession)` 语义。
- 调用 `password.Hash(newPassword)` 生成 Argon2id hash。
- 调用 `repo.UpdateCredentials(ctx, repository.UpdateCredentialsInput{UserID: userID, PasswordHash: passwordHash, Status: domain.UserStatusNormal})` 持久化新凭证、恢复状态并递增 `token_version`。
- 不记录明文密码、完整 hash、salt 或 hash 参数。

备选方案是新增独立 `CredentialUpdater` 接口，保留 `CredentialVerifier` 只做校验。该方案命名更精确，但会增加一个只服务认证流程的组件，短期让 `authService` 注入更多接口，违背“收敛主服务字段”的目标。因此本次选择扩展 `CredentialVerifier` 为凭证组件，后续如果凭证能力继续扩大，再按真实复杂度拆分为 `CredentialService` 或 `CredentialManager`。

### Decision 2: `AuthSessionManager` 接管用户级安全吊销管道

退出全部设备和改密后的凭据失效都属于用户级安全事件。当前 `authService` 在 `LogoutAll` 中直接调用 `repo.IncrementTokenVersion`，再调用 `sessions.InvalidateUserTokenVersion` 和 `sessions.DeleteAllUserSessions`。这使主服务知道数据库版本递增和 Redis 清理的顺序，也要求主服务持有用户 repository。

建议接口：

```go
type AuthSessionManager interface {
    CreateTokenSession(ctx context.Context, userID string, sessionID string, tokenVersion int64, refreshTTL time.Duration) error
    ValidatePasswordChangeClaims(ctx context.Context, claims *auth.Claims) error
    ValidateRefreshSession(ctx context.Context, claims *auth.Claims) (repository.AuthSession, int64, error)
    DeleteSession(ctx context.Context, userID string, sessionID string) error
    RevokeAllUserSessions(ctx context.Context, userID uuid.UUID) (*SessionRevocationResult, error)
}

type SessionRevocationResult struct {
    UserID       uuid.UUID
    TokenVersion int64
}
```

`RevokeAllUserSessions` 应封装完整顺序：

1. 调用 `users.IncrementTokenVersion(ctx, userID)` 在 PostgreSQL 中原子递增目标用户 `token_version`，并获取新版本。
2. 调用 `sessions.InvalidateUserTokenVersion(ctx, userID.String())` 清理 Redis token version 缓存。
3. 调用 `sessions.DeleteAllUserSessions(ctx, userID.String())` 删除该用户全部 Refresh Token 会话记录和用户活跃会话索引。
4. 返回 `SessionRevocationResult`。

该组件需要同时持有 `repository.AuthSessionRepository` 和 `repository.UserRepository`：

```go
type authSessionManager struct {
    users    repository.UserRepository
    sessions repository.AuthSessionRepository
}

func newAuthSessionManager(users repository.UserRepository, sessions repository.AuthSessionRepository) AuthSessionManager {
    return &authSessionManager{users: users, sessions: sessions}
}
```

备选方案是把 `IncrementTokenVersion` 移到 `AuthSessionRepository`，让会话 repository 同时写 PostgreSQL 和 Redis。该方案会污染 repository 边界：`AuthSessionRepository` 目前抽象的是 token version 缓存、Refresh Token 会话和索引，具体 Redis 实现在 `repository/redis` 包；它不应拥有用户 PostgreSQL 写操作。因此本次选择在 service 内 `AuthSessionManager` 组合两个 repository，repository 继续只负责数据访问。

### Decision 3: `authService` 只保留编排依赖

重构后的 `authService` 应删除字段：

- `repo repository.UserRepository`：凭证更新和 token version 递增分别进入 `CredentialVerifier` 与 `AuthSessionManager`。
- `jwt *auth.JWTService`：JWT 签发和解析已完全由 `AuthTokenIssuer` 封装。

`config *config.Config` 不应再作为通用底层依赖被 `authService` 任意使用。当前 `Refresh` 只读取 `s.config.Auth.RefreshTokenRotation`。可以用两种方式降低依赖面：

- 最小变更：保留 `config` 字段，但只用于读取 refresh rotation 策略；后续再抽成 `refreshTokenRotation bool`。
- 推荐变更：在构造函数中提取 `refreshTokenRotation bool`，`authService` 字段只保存该布尔策略，完整配置只传入 `AuthTokenIssuer`。

推荐结构：

```go
type authService struct {
    credentials          CredentialVerifier
    tokens               AuthTokenIssuer
    sessions             AuthSessionManager
    refreshTokenRotation bool
}
```

构造函数仍使用现有 Fx 入参，避免外部依赖注入大改：

```go
type AuthServiceParams struct {
    fx.In

    Repo     repository.UserRepository
    Sessions repository.AuthSessionRepository
    JWT      *auth.JWTService
    Config   *config.Config
}

func NewAuthService(params AuthServiceParams) AuthService {
    return &authService{
        credentials:          newCredentialVerifier(params.Repo),
        tokens:               newAuthTokenIssuer(params.JWT, params.Config),
        sessions:             newAuthSessionManager(params.Repo, params.Sessions),
        refreshTokenRotation: params.Config.Auth.RefreshTokenRotation,
    }
}
```

这样底层依赖仍由 Fx 提供给构造函数，但只在创建子组件时注入，不再作为 `authService` 的可访问状态存在。

### Decision 4: 修改密码流程只表达高层顺序

推荐编排顺序：

```go
func (s *authService) ChangePassword(ctx context.Context, req dto.ChangePasswordRequest) (*dto.ChangePasswordResponse, error) {
    userID, err := s.verifyPasswordChangeToken(ctx, req.Token)
    if err != nil {
        return nil, err
    }
    if _, err := s.credentials.ChangePassword(ctx, userID, req.NewPassword); err != nil {
        return nil, err
    }
    if _, err := s.sessions.RevokeAllUserSessions(ctx, userID); err != nil {
        return nil, err
    }
    return &dto.ChangePasswordResponse{Changed: true}, nil
}
```

职责归属：

- `AuthTokenIssuer.ParsePasswordChangeToken` 解析改密 token、校验 token 类型并得到 claims 与 `uuid.UUID`。
- `AuthSessionManager.ValidatePasswordChangeClaims` 校验服务端当前 token version 与 claims 一致。
- `CredentialVerifier.ChangePassword` 读取用户状态、生成 hash、更新 credentials、恢复状态。
- `AuthSessionManager.RevokeAllUserSessions` 执行用户级 session 吊销，使改密前后所有旧 token 和 refresh session 失效。
- `authService` 只组合这些步骤并返回 DTO。

注意：现有 `UpdateCredentials` 规格要求凭证更新递增 PostgreSQL `token_version`。如果改密后再调用 `RevokeAllUserSessions`，会再递增一次 `token_version`。从安全角度这是可接受且幂等偏保守，但会改变“改密只递增一次”的内部计数观察。为了避免不必要的双递增，有两个可选策略：

- 策略 A：保留 `CredentialVerifier.ChangePassword` 通过 `UpdateCredentials` 递增版本，然后 `AuthSessionManager` 提供 `ClearAllUserSessions(ctx, userID uuid.UUID)` 只清 Redis 缓存和会话，不再递增数据库。该策略不满足本次“AuthSessionManager 自治负责吊销全部会话相关写操作，包括原子递增 token_version”的目标。
- 策略 B：接受修改密码流程中凭证更新递增一次、会话吊销再递增一次，并在规格和测试中只断言旧 token/version 失效，不依赖精确递增次数。该策略更符合“所有用户级吊销统一走 RevokeAllUserSessions”的自治目标。

本次建议采用策略 B，但实现时必须检查现有测试是否断言精确 `token_version` 数值。如果测试只关注旧凭据失效和成功响应，则无需兼容旧内部计数；如果存在精确计数断言，应改为断言版本大于旧值和缓存/会话已删除。

### Decision 5: 退出全部设备流程只表达认证上下文到吊销调用

推荐编排顺序：

```go
func (s *authService) LogoutAll(ctx context.Context) (*dto.LogoutResponse, error) {
    userID, _, err := authenticatedSession(ctx)
    if err != nil {
        logger.Warn(ctx, "logout all missing authenticated session", zap.Error(err))
        return nil, err
    }
    parsedUserID, err := uuid.Parse(userID)
    if err != nil {
        logger.Warn(ctx, "logout all user id invalid", zap.String("user_id", userID))
        return nil, response.UnauthenticatedError(messages.MissingSession)
    }
    if _, err := s.sessions.RevokeAllUserSessions(ctx, parsedUserID); err != nil {
        return nil, err
    }
    return &dto.LogoutResponse{LoggedOut: true}, nil
}
```

职责归属：

- `authenticatedSession` 继续作为认证上下文边界，校验 `user_id` 和 `session_id` 存在且 `user_id` 是 UUID。
- `authService` 可以保留 `uuid.Parse`，因为这是认证上下文输入到子组件强类型参数的边界转换，不是底层写操作。
- `AuthSessionManager.RevokeAllUserSessions` 负责所有 DB/Redis 写入和错误映射。

### Decision 6: 错误分类和日志跟随职责迁移

错误映射应跟随负责该操作的组件迁移：

- `CredentialVerifier.ChangePassword` 负责把用户不存在映射为 not found，把状态不允许改密映射为 token invalid，把 hash/update 错误映射为现有内部错误语义。
- `AuthSessionManager.RevokeAllUserSessions` 负责把 `domain.ErrUserNotFound` 映射为 `response.NotFoundError(messages.UserNotFound)`，把数据库递增、缓存失效、会话删除错误映射为 `response.FromError`。
- `authService` 只记录流程入口或认证上下文缺失等编排级日志，不记录底层数据访问失败。

日志埋点应避免敏感信息，仅使用 `user_id`、`session_id`、`token_version`、操作名和错误，不记录 token 原文、新密码、密码 hash。

### Decision 7: 事务边界保持 repository 原子操作，不引入跨 Redis/DB 分布式事务

`IncrementTokenVersion` 和 `UpdateCredentials` 应保持单条数据库更新或 repository 内原子操作。Redis 缓存与会话清理在数据库更新成功后执行，延续现有“先持久化真实版本，再清缓存和会话”的安全顺序。PostgreSQL 与 Redis 之间不引入分布式事务。

如果 Redis 清理失败，数据库 `token_version` 已递增，旧 Access Token 会在后续 token version 校验中因缓存被删除失败而存在短暂风险：如果旧版本缓存仍在 Redis，认证中间件可能暂时读到旧版本。实现上应保证 `InvalidateUserTokenVersion` 错误被返回并记录，调用方得到失败响应；后续可通过缩短 cache TTL 或重试机制缓解。本次不新增后台补偿任务。

## Risks / Trade-offs

- [Risk] 职责迁移后出现循环依赖，例如 `AuthSessionManager` 依赖 `AuthService` 或 token 组件。→ Mitigation：子组件只依赖 repository、config/JWT 等底层抽象，不依赖 `AuthService`；`authService` 单向依赖接口。
- [Risk] 修改密码流程可能因 `UpdateCredentials` 与 `RevokeAllUserSessions` 都递增 `token_version` 而改变内部版本增量。→ Mitigation：测试不依赖精确增量，断言旧 token 失效、缓存删除和会话删除；如业务必须单次递增，再拆出只清 session 的方法并明确规格例外。
- [Risk] Redis 清理失败导致 PostgreSQL 已更新但缓存或 session 残留。→ Mitigation：保持 DB 先更新、Redis 后清理，错误上抛并记录；保留 token version cache TTL 作为最终一致兜底。
- [Risk] 错误映射迁移导致 HTTP 响应码或业务码变化。→ Mitigation：迁移前后用现有 service/controller 测试覆盖 invalid token、user not found、status rejected、repository error、session delete error。
- [Risk] 构造函数调整影响 Fx 注入。→ Mitigation：保留 `AuthServiceParams` 对外形状，只调整 `NewAuthService` 内部组装和私有构造函数签名。
- [Risk] 单测 mock 范围变化。→ Mitigation：增加组件级 fake repository 测试，authService 测试改为 fake 子组件或通过现有构造验证外部行为。

## Migration Plan

1. 扩展接口和私有结果类型：新增 `CredentialUpdateResult`、`SessionRevocationResult`，为 `CredentialVerifier` 和 `AuthSessionManager` 增加自治方法。
2. 在 `credentialVerifier` 中迁移 `auth_service.go:79-104` 的用户读取、状态校验、密码 hash 和 `UpdateCredentials` 逻辑，保留现有错误映射和日志语义。
3. 在 `authSessionManager` 中注入 `repository.UserRepository`，迁移 `auth_service.go:166-179` 的 `IncrementTokenVersion`、`InvalidateUserTokenVersion`、`DeleteAllUserSessions` 管道。
4. 精简 `authService` 字段，删除 `repo` 和 `jwt`，将 `config` 收敛为 `refreshTokenRotation bool`，构造函数只在创建子组件时使用底层依赖。
5. 重写 `ChangePassword` 和 `LogoutAll`，只保留高层编排顺序。
6. 更新测试：先覆盖子组件自治，再调整 authService 编排和回归测试。
7. 分别在 `user-services/` 运行 `go test ./...`，必要时在 `common/` 运行 `go test ./...` 确认工作区兼容。

Rollback 策略：本变更不改 schema、配置或外部 API，可通过恢复 service 层代码和测试回滚；不会产生数据库迁移回滚问题。

## Open Questions

- 修改密码后是否允许 `token_version` 递增两次。设计建议接受“版本只需单调增加”的安全语义，但实现前应检查现有测试和调用方是否依赖精确增量。
- 是否将 `CredentialVerifier` 重命名为 `CredentialManager` 或 `CredentialService`。为降低改动范围，本次建议暂不重命名接口；后续如果凭证能力继续扩展，再单独提出命名重构。
