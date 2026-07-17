## 1. Constructor API 清理

- [x] 1.1 修改 `user-service/internal/features/auth/infrastructure/postgres/credential_store.go`，移除 `go.uber.org/fx` import、`CredentialStoreParams` 的 `fx.In` 和 `name:"primary_db"` tag，并改为普通参数或无 DI tag Options 构造 `CredentialStore`。
- [x] 1.2 修改 `user-service/internal/features/auth/infrastructure/redis/session_store.go`，移除 `go.uber.org/fx` import 和 `SessionStoreParams` 的 DI metadata，改为接收 Redis client、`KeyCatalog`、token version cache TTL、`PurgeTaskPool` 和 metrics 的窄构造输入。
- [x] 1.3 修改 `user-service/internal/features/auth/transport/http/controller.go`，移除 `go.uber.org/fx` import 和 `AuthControllerParams` 的 DI metadata，改为普通参数或无 DI tag Options 构造 `AuthController`。

## 2. Fx Composition 适配

- [x] 2.1 修改 `user-service/internal/features/auth/fx.go`，在 feature composition 边界新增私有 Fx provider 输入结构，用 named `primary_db`、`cache_redis` 和 `auth_session_purge_pool` 适配新的普通 constructor。
- [x] 2.2 在 composition 层从 `*serviceconfig.Config` 提取 `App.Name` 与 `Auth.TokenVersionCacheTTL`，创建 `authredis.KeyCatalog` 并传入 `SessionStore`，不得把完整 config 继续传入 Redis adapter。
- [x] 2.3 保持 `SessionPurgePool`、token-version localcache lifecycle、auth feature Fx module 和现有 port `fx.As` 暴露不变。

## 3. 测试与调用点迁移

- [x] 3.1 更新 auth PostgreSQL credential store 测试，使用新的普通 constructor，并确认凭据查询、token version 和条件更新行为不变。
- [x] 3.2 更新 auth Redis session store 测试和 test helper，使用新的窄构造输入，并覆盖现有 session、password-change session、token version cache 和 purge 行为不变。
- [x] 3.3 更新 auth HTTP controller、providers 和 router 相关测试调用点，使用新的 controller constructor。
- [x] 3.4 使用 `rg -n 'CredentialStoreParams|SessionStoreParams|AuthControllerParams' user-service/internal` 检查旧参数类型无残留；如采用同名无 DI tag Options，确认只在允许的 feature-local 普通构造 API 中出现且无 DI tag。

## 4. 验证

- [x] 4.1 运行 `cd user-service && go test ./internal/features/auth/infrastructure/... ./internal/features/auth/transport/http/... -count=1` 并通过。
- [x] 4.2 运行 `rg -n 'go\.uber\.org/(fx|dig)|fx\.(In|Out)' user-service/internal/features/auth/infrastructure/postgres/credential_store.go user-service/internal/features/auth/infrastructure/redis/session_store.go user-service/internal/features/auth/transport/http/controller.go` 并确认无输出。
- [x] 4.3 运行 `openspec validate remove-fx-from-auth-adapters` 并通过。
- [x] 4.4 运行 `make user-service-architecture-lint` 并通过。
- [x] 4.5 暂存本次预期代码、规格和文档变更后，运行 `make lint` 并通过。
- [x] 4.6 保持本次预期变更已暂存，运行 `make verify` 并通过。
