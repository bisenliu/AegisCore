## 1. PostgreSQL 更新实现

- [x] 1.1 在 `user-service/internal/features/auth/infrastructure/postgres/credential_store.go` 中将 `IncrementTokenVersion` 改为 Ent 事务内更新并返回 `token_version`，移除成功路径上的 `GetTokenVersion` 调用。
- [x] 1.2 将 `UpdateCredentials` 改为 Ent 条件更新并在同一事务内返回新 `token_version`，保持用户不存在返回 `identity.ErrUserNotFound`、状态或版本条件不匹配返回 `authdomain.ErrTokenInvalid`。
- [x] 1.3 确认新实现不新增兼容分支、不修改 Ent schema、不新增 Atlas migration、不将 auth PostgreSQL SQL helper 上移到 `common` 或 `internal/shared`。

## 2. 撤销编排校验

- [x] 2.1 核对 `sessions.RevokeAllUserSessions` 在拿到新 `token_version` 后继续调用 `RevokeUserSessionsAtVersion`，并保持投影失败返回 `authdomain.ErrSessionRevocationIncomplete` 的上层语义。
- [x] 2.2 核对强制改密用例在 `UpdateCredentials` 成功返回新版本后继续执行本地缓存失效、Redis 投影刷新和 refresh session 撤销，失败时不返回普通成功结果。

## 3. 测试覆盖

- [x] 3.1 更新 `user-service/internal/features/auth/infrastructure/postgres/credential_store_test.go`，覆盖 `IncrementTokenVersion` 成功返回递增版本且不依赖提交后第二次 `SELECT`。
- [x] 3.2 更新 `credential_store_test.go`，覆盖 `UpdateCredentials` 成功返回新版本、用户不存在、状态条件不匹配和 token version 条件不匹配。
- [x] 3.3 补充 auth application 层测试，覆盖 PostgreSQL 更新成功后进入撤销编排，以及 Redis/session 投影失败时返回 `authdomain.ErrSessionRevocationIncomplete`。
- [x] 3.4 增加测试覆盖和实现约束，证明成功路径不存在提交后的 `GetTokenVersion` 读取；只能观察到未更新失败或已更新并拿到版本后进入撤销编排。

## 4. 验证与收尾

- [x] 4.1 运行相关 Go 测试：`go test ./user-service/internal/features/auth/...`。
- [x] 4.2 运行架构规格校验：`make user-service-architecture-lint`。
- [x] 4.3 检查不需要 OpenAPI、Ent 或 Atlas 生成物；如出现相关 diff，确认不是本次预期变更并处理 drift。
- [x] 4.4 将本次预期代码、测试和 OpenSpec 变更加到暂存区。
- [x] 4.5 运行 `make lint` 并确认通过。
- [x] 4.6 运行 `make verify` 并确认通过。
