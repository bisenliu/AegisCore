## 1. 凭证组件自治

- [x] 1.1 在 `auth_credentials.go` 中新增 `CredentialUpdateResult`，并为 `CredentialVerifier` 增加 `ChangePassword(ctx context.Context, userID uuid.UUID, newPassword string)` 方法。
- [x] 1.2 将 `auth_service.go` 修改密码流程中的用户读取、`CanChangePassword` 状态校验、`password.Hash` 和 `repo.UpdateCredentials` 逻辑迁移到 `credentialVerifier.ChangePassword`。
- [x] 1.3 在 `credentialVerifier.ChangePassword` 中保持现有错误映射和安全日志语义：user not found 映射 not found，状态拒绝映射 token invalid，hash/update 错误映射内部错误，日志不包含明文密码或完整 hash。
- [x] 1.4 为 `CredentialVerifier` 增加或调整单元测试，覆盖成功改密、用户不存在、状态不允许改密、hash/update repository 错误和不泄露敏感信息的日志输入边界。

## 2. 会话组件自治

- [x] 2.1 在 `auth_sessions.go` 中新增 `SessionRevocationResult`，并为 `AuthSessionManager` 增加 `RevokeAllUserSessions(ctx context.Context, userID uuid.UUID)` 方法。
- [x] 2.2 调整 `authSessionManager` 构造函数和字段，使其同时接收 `repository.UserRepository` 与 `repository.AuthSessionRepository`，但不依赖 Redis 具体实现或 Ent 生成模型。
- [x] 2.3 将 `auth_service.go` 退出全部设备流程中的 `repo.IncrementTokenVersion`、`InvalidateUserTokenVersion`、`DeleteAllUserSessions` 管道迁移到 `authSessionManager.RevokeAllUserSessions`。
- [x] 2.4 在 `authSessionManager.RevokeAllUserSessions` 中保持 DB 先更新、Redis 后清理的顺序，并保持 `domain.ErrUserNotFound`、Redis 清理失败和 repository 内部错误的现有响应映射。
- [x] 2.5 为 `AuthSessionManager` 增加或调整单元测试，覆盖成功吊销、用户不存在、token version 递增失败、token version 缓存清理失败和全部会话删除失败。

## 3. AuthService 编排收敛

- [x] 3.1 精简 `authService` 结构体字段，删除 `repo` 和原始 `jwt` 字段，将完整 `config` 字段收敛为 `refreshTokenRotation bool` 或等价高层策略字段。
- [x] 3.2 调整 `NewAuthService`，保留现有 Fx 入参形状，但只在构造 `CredentialVerifier`、`AuthTokenIssuer` 和 `AuthSessionManager` 时注入底层依赖，不再把底层 repository/JWT 保存到 `authService`。
- [x] 3.3 重写 `ChangePassword`，使流程只执行改密 token 解析与版本校验、调用 `credentials.ChangePassword`、调用 `sessions.RevokeAllUserSessions` 和返回成功响应。
- [x] 3.4 重写 `LogoutAll`，使流程只执行认证上下文提取、UUID 转换、调用 `sessions.RevokeAllUserSessions` 和返回成功响应。
- [x] 3.5 确认 `Login`、`Refresh`、`Logout` 和 `issueTokenPair` 外部行为保持不变，尤其是 Refresh Token rotation、session 创建和当前设备退出语义。

## 4. 回归测试与验证

- [x] 4.1 调整 `auth_service_test.go`，让 authService 测试聚焦编排顺序、错误透传、DTO 响应和外部行为，不再断言主服务直接调用底层 repository。
- [x] 4.2 调整 `auth_components_test.go`，补齐凭证组件和会话组件自治后的组件级测试覆盖。
- [x] 4.3 检查是否存在对 `token_version` 精确递增次数的测试断言；如存在，改为断言版本单调增加、旧 token 失效和 Redis 会话清理结果。
- [x] 4.4 在 `user-services/` 运行 `gofmt` 覆盖被修改的 Go 文件。
- [x] 4.5 在 `user-services/` 运行 `go test ./...`，修复编译、静态或单元测试失败。
- [x] 4.6 如公共模块或工作区依赖受影响，在 `common/` 运行 `go test ./...` 确认没有跨模块回归。

## 5. 规格一致性检查

- [x] 5.1 对照 `openspec/changes/refactor-auth-service-orchestration/specs/user-session-control/spec.md`，确认实现满足 AuthService 不直接持有原始 JWT service 和用户 repository 写操作的要求。
- [x] 5.2 确认未修改 HTTP API、DTO、响应信封、Redis key 格式、Ent schema 或 Atlas migration。
- [x] 5.3 运行 `openspec status --change "refactor-auth-service-orchestration"` 确认变更工件和任务状态可用于 `/opsx:apply`。
