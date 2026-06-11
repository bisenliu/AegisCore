# Tasks

## Implementation

- [x] 阅读 `docs/ARCHITECTURE.md` 和当前 `common/runtime` 结构，确认 Fx provider 迁移只发生在 runtime owner 包内。
- [x] 将 `common/runtime/configfx/config.go` 迁移到 `common/runtime/config/fx.go`，包名改为 `config`，保留 `ConfigPath` 和 `NewConfig` 语义。
- [x] 将 `common/runtime/loggerfx/logger.go` 迁移到 `common/runtime/logger/fx.go`，包名改为 `logger`，保留 Fx lifecycle sync hook 和 stdout/stderr 兼容错误处理。
- [x] 检查 `common/runtime/datastore/redis.go` 中底层 Redis 构造函数命名，解决与迁移后 Fx provider `NewRedisClient` 的同包命名冲突。
- [x] 将 `common/runtime/datastorefx/redis.go` 迁移到 `common/runtime/datastore`，保留 `ProvideNamedRedis` 和 Fx 版 `NewRedisClient` 的参数、返回值、Ping/Close 行为和日志语义。
- [x] 将 `common/runtime/datastorefx/postgres.go` 迁移到 `common/runtime/datastore`，保留 `ProvideNamedPostgres`、`NewPostgres`、`NewPostgresPools`、错误聚合和生命周期行为。
- [x] 迁移 `common/runtime/datastorefx/datastorefx_test.go` 到 `common/runtime/datastore`，并更新包名、import 和测试调用。
- [x] 更新 `user-service/internal/bootstrap/app.go`，使用 `config.ConfigPath`、`config.NewConfig` 和 `logger.NewLogger`。
- [x] 更新 `user-service/internal/bootstrap/redis.go`，使用 `datastore.NewRedisClient`。
- [x] 更新 `user-service/internal/bootstrap/postgres.go`，使用 `datastore.NewPostgresPools`。
- [x] 更新 `user-service/internal/bootstrap/validation_test.go` 和其他测试中的旧 `configfx`、`loggerfx`、`datastorefx` import。
- [x] 删除旧 `common/runtime/configfx`、`common/runtime/loggerfx`、`common/runtime/datastorefx` 包中的 Go 源码，确保不再形成可导入包。
- [x] 更新 `AGENTS.md` 中共享 runtime Fx provider 的关键入口路径。
- [x] 更新 `docs/ARCHITECTURE.md` 和必要开发文档中对 `loggerfx`、`configfx`、`datastorefx` 的描述。
- [x] 运行 `gofmt -w` 格式化所有改动的 Go 文件。

## Verification

- [x] 运行 `rg -n "common/runtime/(configfx|loggerfx|datastorefx)|\\bconfigfx\\b|\\bloggerfx\\b|\\bdatastorefx\\b" common user-service AGENTS.md docs`，确认当前业务代码和长期规则文档不再引用旧 Fx 包。
- [x] 在 `common/` 运行 `go test ./...`。
- [x] 在 `user-service/` 运行 `go test ./...`。
- [x] 检查 Redis provider 仍读取 `redis.cache_redis`，Fx output name 仍为 `cache_redis`。
- [x] 检查 PostgreSQL provider 仍读取 `postgres.user_db` 和 `postgres.common_db`，Fx output names 仍为 `user_db`、`common_db`。
- [x] 检查日志 provider 仍在 Fx stop 阶段调用 `Sync`，且仍忽略 stdout/stderr 不支持 fsync 的平台错误。
- [x] 检查 `git diff -- common user-service AGENTS.md docs`，确认没有配置结构、业务逻辑、HTTP API、Ent schema、migration 或 generated code 变更。

## Review Notes

- [x] 确认没有新增 `openspec/` 或 `docs/opsx/`。
- [x] 确认没有把服务特定 helper 放入 `common/runtime`。
- [x] 确认没有新增横向 `internal/controller`、`internal/service`、`internal/repository`、`internal/api` 或 `internal/domain` 包。
- [x] 确认旧 Fx 包没有通过兼容 shim 延续为业务可用入口。
- [x] 确认本变更可以一次性迁移，实施后不需要新旧 Fx 包并存。
