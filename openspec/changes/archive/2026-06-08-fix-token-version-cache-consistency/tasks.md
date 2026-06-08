## 1. Repository Contract

- [x] 1.1 调整 `repository.AuthSessionRepository` 的安全事件 token version cache 契约，使用户级吊销流程使用写入新版本语义而不是删除旧缓存语义。
- [x] 1.2 更新 Redis auth session repository 实现，复用现有 token version cache TTL 和 key builder，将指定用户 cache 覆盖为 PostgreSQL 返回的新 `token_version`。
- [x] 1.3 同步更新 service、bootstrap 和 repository 测试 stub，确保接口变更不破坏 Fx 构造和受保护路由测试。

## 2. Service Flow

- [x] 2.1 修改 `authSessionManager.RevokeAllUserSessions`，在 `IncrementTokenVersion` 成功后先写入新 token version cache，再删除全部 Redis Refresh Token 会话和用户会话索引。
- [x] 2.2 保持 `AuthService.ChangePassword` 和 `AuthService.LogoutAll` 只调用认证会话组件，不直接执行用户 repository 写入或 Redis 清理。
- [x] 2.3 确保 token version cache 写入失败时返回错误，并且不会将改密或退出全部设备报告为成功。
- [x] 2.4 确保全部 Refresh Token 会话删除失败时仍返回错误，但新 token version cache 已可使旧 token 因版本不一致被拒绝。

## 3. Tests

- [x] 3.1 添加或更新 service 层单元测试，验证 `RevokeAllUserSessions` 使用 DB 返回的新版本刷新 Redis cache。
- [x] 3.2 添加或更新 service 层单元测试，验证 token version cache 写入失败时流程中止且不报告成功。
- [x] 3.3 添加或更新 service 层单元测试，验证 Refresh Token 会话删除失败时已尝试写入新 token version cache。
- [x] 3.4 添加或更新 Redis repository 测试，验证用户安全事件可覆盖旧 token version cache 并保持 TTL 行为。
- [x] 3.5 添加或更新认证校验相关测试，验证旧 Access Token 在新 token version cache 命中时被拒绝。

## 4. Validation

- [x] 4.1 对修改的 Go 文件执行 `gofmt`。
- [x] 4.2 在 `user-services/` 执行 `go test ./...`。
- [x] 4.3 在 `common/` 执行 `go test ./...`，确认认证中间件契约未被破坏。
- [x] 4.4 确认无需运行 `go generate ./ent`、Atlas migration diff 或 migration validate，因为本变更不修改 Ent schema 或数据库结构。
