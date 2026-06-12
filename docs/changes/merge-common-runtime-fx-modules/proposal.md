# Merge common runtime Fx modules

## What

将 `common/runtime` 下独立的 Fx provider 包合并回对应 runtime 能力包，使共享运行时结构更贴近最终目录模型：

- `common/runtime/configfx` 合并到 `common/runtime/config/fx.go`。
- `common/runtime/loggerfx` 合并到 `common/runtime/logger/fx.go`。
- `common/runtime/datastorefx` 合并到 `common/runtime/datastore/fx.go` 或同包内按资源拆分的 Fx 文件。
- 更新 `user-service/internal/bootstrap` 和相关测试中的 import 与调用点。
- 一次性完成迁移，避免业务代码同时使用新旧 Fx 包。

本变更只调整共享 runtime Fx provider 的包归属和调用路径，不改变配置结构、Redis/PostgreSQL named resource 语义、连接生命周期行为、日志行为或 HTTP/业务行为。

## Why

当前 `configfx`、`loggerfx`、`datastorefx` 以横向 `*fx` 包承载 Fx 组装函数，和 `common/runtime/config`、`common/runtime/logger`、`common/runtime/datastore` 的能力所有权分离。随着仓库规则明确 `common/runtime` 按能力分类组织，Fx provider 继续散落在独立包中会让运行时结构显得像第二套分层，也会增加后续服务接入共享 runtime 能力时的查找成本。

将 Fx provider 放回对应 owner 包，可以让每个 runtime 子包同时拥有底层构造函数与 Fx 适配入口：

- 配置加载和 `ConfigPath` 位于 `runtime/config`。
- zap logger 构造和 Fx lifecycle sync hook 位于 `runtime/logger`。
- Redis/PostgreSQL client/pool 构造和 Fx named provider helper 位于 `runtime/datastore`。

这样后续服务只需要依赖能力包本身，不再依赖额外 `*fx` 包。

## Scope

包括：

- 在 `common/runtime/config` 中新增或迁移 Fx provider 文件，保留 `ConfigPath` 和 `NewConfig` 等对外入口。
- 在 `common/runtime/logger` 中新增或迁移 Fx provider 文件，保留 `NewLogger` 的 Fx lifecycle 行为。
- 在 `common/runtime/datastore` 中新增或迁移 Redis/PostgreSQL Fx provider，保留 `ProvideNamedRedis`、`NewRedisClient`、`ProvideNamedPostgres`、`NewPostgres`、`NewPostgresPools` 的语义。
- 更新 `user-service/internal/bootstrap/app.go`、`redis.go`、`postgres.go` 和测试中对旧 Fx 包的引用。
- 迁移 `common/runtime/datastorefx` 相关测试到 `common/runtime/datastore` 包，并保持覆盖范围。
- 更新 `AGENTS.md`、`docs/ARCHITECTURE.md` 以及必要开发文档中仍指向旧 Fx 包的说明。
- 删除空的旧 `common/runtime/configfx`、`common/runtime/loggerfx`、`common/runtime/datastorefx` 包目录中的 Go 源码，确保业务代码不再引用旧包。

不包括：

- 不改变 YAML 配置结构或 `AEGISCORE_` 环境变量覆盖规则。
- 不改变 Redis/PostgreSQL named resource key，例如 `redis.cache_redis`、`postgres.user_db`、`postgres.common_db`。
- 不改变 Fx module 拓扑、feature module 组装方式或 HTTP route registration。
- 不改变 datastore 底层连接参数、Ping/Close 生命周期、错误信息语义或日志字段。
- 不移动 `common/runtime/resources`、`common/runtime/timezone` 或 `common/runtime/observability`。
- 不新增 `openspec/` 或 `docs/opsx/` 工件。

## Acceptance Criteria

- `common/runtime/config/fx.go` 提供配置相关 Fx 入口，业务代码不再导入 `common/runtime/configfx`。
- `common/runtime/logger/fx.go` 提供日志相关 Fx 入口，业务代码不再导入 `common/runtime/loggerfx`。
- `common/runtime/datastore` 提供 Redis/PostgreSQL 相关 Fx 入口，业务代码不再导入 `common/runtime/datastorefx`。
- `user-service/internal/bootstrap` 使用新的 `config`、`logger`、`datastore` 包入口完成 Fx 组装。
- Redis/PostgreSQL named resource 语义保持不变，`cache_redis`、`user_db`、`common_db` 的 Fx name 和配置 key 不变。
- 旧 `configfx`、`loggerfx`、`datastorefx` 包不再被 `common/` 或 `user-service/` 业务代码引用。
- `common/` 下 `go test ./...` 通过。
- `user-service/` 下 `go test ./...` 通过。
