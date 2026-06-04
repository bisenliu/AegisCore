## 1. Rename Bootstrap Providers

- [x] 1.1 将 `user-services/internal/bootstrap/postgres.go` 中的 `NewPostgresPools` 重命名为 `ProvidePostgresPools`，保持参数类型、返回类型和函数体行为不变。
- [x] 1.2 将 `user-services/internal/bootstrap/redis.go` 中的 `NewRedisClients` 重命名为 `ProvideRedisClients`，保持参数类型、返回类型和函数体行为不变。
- [x] 1.3 将 `user-services/internal/bootstrap/ent.go` 中的 `NewNamedClients` 重命名为 `ProvideEntClients`，保持参数类型、返回类型和函数体行为不变。
- [x] 1.4 更新 `user-services/internal/bootstrap/app.go` 中 Fx provider 列表，引用 `ProvidePostgresPools`、`ProvideRedisClients` 和 `ProvideEntClients`。

## 2. Update Tests

- [x] 2.1 更新 `user-services/internal/bootstrap/postgres_test.go` 中 PostgreSQL provider 引用和测试函数名。
- [x] 2.2 更新 `user-services/internal/bootstrap/postgres_test.go` 中 Ent clients provider 引用和测试函数名。
- [x] 2.3 更新 `user-services/internal/bootstrap/postgres_test.go` 中 Redis provider 引用、错误消息断言和测试函数名。

## 3. Preserve Contracts

- [x] 3.1 将 `user-services/internal/bootstrap/ent.go` 中的 `ClientParams` 重命名为 `NamedEntClientParams`，保持字段、Fx tag 和行为不变。
- [x] 3.2 将 `user-services/internal/bootstrap/ent.go` 中的 `NamedClients` 重命名为 `NamedEntClients`，保持字段、Fx tag 和行为不变。
- [x] 3.3 将 `user-services/internal/bootstrap/ent.go` 中的私有 helper `newClient` 重命名为 `newEntClient`，保持实现行为不变。
- [x] 3.4 更新 `user-services/internal/bootstrap/postgres_test.go` 中 Ent provider 测试引用的类型名。
- [x] 3.5 确认 `NamedPostgresParams`、`NamedRedisParams`、`NamedPostgresPools` 和 `NamedRedisClients` 类型名与结构保持不变。
- [x] 3.6 确认 `user_db`、`common_db`、`cache_redis` Fx name tags 和 `commoninfra.Name*` 资源名常量引用保持不变。
- [x] 3.7 确认不修改 controller/service/repository 构造函数、`NewApp`、`NewJWTService`、`NewGinEngine`、`NewHTTPServer`、配置结构、Ent schema 或 Atlas migration。

## 4. Verification

- [x] 4.1 在 `user-services` 模块运行 `gofmt -w internal/bootstrap/postgres.go internal/bootstrap/redis.go internal/bootstrap/ent.go internal/bootstrap/app.go internal/bootstrap/postgres_test.go`。
- [x] 4.2 在 `user-services` 模块运行 `go test ./...`，验证编译、bootstrap provider 测试和现有用户服务测试通过。
