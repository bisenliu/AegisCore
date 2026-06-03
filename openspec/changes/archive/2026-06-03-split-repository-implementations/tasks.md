## 1. Redis Auth Session Repository

- [x] 1.1 新建 `user-services/internal/repository/auth_session_repository.go`，定义 `AuthSession`、`AuthSessionRepository`、`ErrAuthSessionNotFound` 和 `ErrTokenVersionMismatch`。
- [x] 1.2 新建 `user-services/internal/repository/redis/auth_session_repository.go`，从 `service/session_store.go` 迁移 Redis session store 实现、key helper、TTL fallback 和 token version cache 逻辑。
- [x] 1.3 将 Redis 实现类型和 provider 重命名为 `authSessionRepository`、`AuthSessionRepositoryParams` 和 `NewAuthSessionRepository`，并返回 `repository.AuthSessionRepository`。
- [x] 1.4 删除 `user-services/internal/service/session_store.go`，确认 service 层不再定义或实现 session store。

## 2. Auth Service And Runtime References

- [x] 2.1 更新 `user-services/internal/service/auth_service.go`，将 `SessionStore`、`Session` 和 session 错误替换为 `repository.AuthSessionRepository`、`repository.AuthSession` 和 repository 错误。
- [x] 2.2 更新 `user-services/internal/bootstrap/bootstrap.go`，将 session provider 从 `service.NewSessionStore` 替换为 `redis.NewAuthSessionRepository`，并更新 `BootstrapParams.SessionStore` 类型。
- [x] 2.3 更新 `user-services/internal/bootstrap/http_test.go` 中的认证会话 stub 类型和 import，保持认证中间件 token version validator 测试语义不变。
- [x] 2.4 更新 `user-services/internal/service/auth_service_test.go` 中的 session stub、错误和 helper 签名，确保登录、刷新、退出和改密测试继续覆盖 repository 抽象。

## 3. PostgreSQL User Repository

- [x] 3.1 新建 `user-services/internal/repository/postgres/user_repository.go`，从根 `repository/user_repository.go` 迁移 Ent/PostgreSQL `UserRepository` 实现、Fx params、provider、查询方法和 predicate helper。
- [x] 3.2 精简 `user-services/internal/repository/user_repository.go`，只保留 `UserRepository` 接口和 `CreateUserInput`、`UpdateCredentialsInput`、`ListUsersInput` 等输入类型。
- [x] 3.3 更新 `user-services/internal/bootstrap/bootstrap.go`，将用户仓储 provider 从 `repository.NewUserRepository` 替换为 `postgres.NewUserRepository`。
- [x] 3.4 确认根 `repository` 包不 import `repository/postgres` 或 `repository/redis`，service 层不 import 具体实现包。

## 4. Repository Tests Migration

- [x] 4.1 将 `user-services/internal/service/session_store_test.go` 迁移为 `user-services/internal/repository/redis/auth_session_repository_test.go`，更新 package、类型名和 repository import。
- [x] 4.2 将 `user-services/internal/repository/user_repository_test.go` 迁移为 `user-services/internal/repository/postgres/user_repository_test.go`，更新 package 和必要的根 repository 类型引用。
- [x] 4.3 删除迁移后的旧测试文件，确认没有测试仍引用 `service.Session`、`service.SessionStore`、`ErrSessionNotFound` 或 `NewSessionStore`。

## 5. Verification

- [x] 5.1 对修改的 Go 文件运行 `gofmt`。
- [x] 5.2 在 `user-services/` 执行 `go test ./...`，修复编译、import、Fx 装配或测试失败。
- [x] 5.3 检查本次变更未修改 Ent schema、`user-services/ent/` 生成代码、Atlas migration、HTTP 路由、配置样例或响应契约。
