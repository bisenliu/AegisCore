# Design

## Overview

本变更把 user-service 的启动壳和服务级 provider 适配层分开：

- `internal/bootstrap`：只保留 Fx app 创建、顶层 module 组合和 HTTP server 生命周期。
- `internal/providers`：承载用户服务级 provider，包括 Gin、routes、JWT、PostgreSQL、Redis 和 Ent。
- `internal/router`：继续只负责 route 定义、系统路由、Swagger 路由和认证中间件挂载，不引入 Fx。
- `internal/features/*`：继续由各 feature 自己提供 app service、controller 和 infra adapter。

目标依赖方向：

```text
bootstrap
  -> providers
  -> router
  -> features/*/transport/http

features/*/module.go
  -> feature-local app/transport/infra
```

`bootstrap` 可以导入 `providers` 和 feature modules 来声明顶层 module，但不再实现具体 provider。`providers` 可以导入 `router`、feature HTTP controllers、common runtime/security 和 Ent。`router` 不反向导入 `providers` 或 `bootstrap`。

## Target Package Layout

```text
user-service/internal/bootstrap/
  app.go
  server.go
  server_test.go or http_test.go

user-service/internal/providers/
  fx.go
  auth.go
  ent.go
  ent_test.go
  gin.go
  gin_test.go
  postgres.go
  postgres_test.go
  redis.go
  redis_test.go
  routes.go
  routes_test.go
```

测试文件名称可以沿用现有命名，只要测试跟随被测职责迁移。`bootstrap` 中仅保留 HTTP server 生命周期相关测试；Gin、routes、auth、datastore 和 Ent provider 测试迁移到 `providers` 包。

## Providers Module

新增 `user-service/internal/providers/fx.go`：

```go
package providers

import "go.uber.org/fx"

var Module = fx.Module("user-service-providers",
    fx.Provide(
        ProvidePostgresPools,
        ProvideRedisClients,
        NewJWTService,
        ProvideEntClients,
        NewGinEngine,
    ),
    fx.Invoke(RegisterRoutes),
)
```

是否把 HTTP server 实例化 invoke 放入 `providers.Module` 之外：

- `NewHTTPServer` 仍属于 `bootstrap`，因为它管理进程级 HTTP server lifecycle。
- `func(*http.Server) {}` 仍由 `bootstrap.AppModule` 负责 invoke，确保 server 生命周期 hook 被注册。

`bootstrap.AppModule` 目标形态：

```go
var AppModule = fx.Module("aegiscore-user-services",
    commontz.Module,
    validation.Module,
    authfeature.Module,
    userfeature.Module,
    providers.Module,
    fx.Provide(NewHTTPServer),
    fx.Invoke(func(*http.Server) {}),
)
```

`bootstrap.NewApp` 继续供应 config path，并提供 shared runtime config/logger：

```go
fx.Supply(config.ConfigPath(configPath))
fx.Provide(config.NewConfig, logger.NewLogger)
AppModule
```

## File Moves

### Gin provider

Move:

- From `user-service/internal/bootstrap/gin.go`
- To `user-service/internal/providers/gin.go`

Keep:

- `GinParams` Fx input shape.
- `gin.SetMode(gin.ReleaseMode)`.
- `engine.SetTrustedProxies(params.Config.HTTP.TrustedProxies)` only when list is non-empty.
- Middleware order: trace ID, recovery, request logger, CORS.
- Error wrapping text for trusted proxy setup unless implementation already has a better existing convention.

Only package name and imports should change.

### Route registration provider

Move:

- From `user-service/internal/bootstrap/routes.go`
- To `user-service/internal/providers/routes.go`

Keep:

- `RegisterRouteParams` shape and optional `TokenVersions` dependency.
- Mapping from config/log/JWT/controllers into `router.RouteParams`.
- Public/protected route behavior delegated to `router.RegisterUserServiceHTTPRoutes`.

`providers/routes.go` remains an Fx adapter. Route path definitions stay in `internal/router` and `features/*/transport/http/routes.go`.

### Auth provider

Move:

- From `user-service/internal/bootstrap/auth.go`
- To `user-service/internal/providers/auth.go`

Keep:

- `NewJWTService(cfg *config.Config) *auth.JWTService`
- Construction from `cfg.Auth`.

This is service-level auth wiring, not auth feature business logic. Feature auth session behavior remains in `internal/features/auth`.

### PostgreSQL provider

Move:

- From `user-service/internal/bootstrap/postgres.go`
- To `user-service/internal/providers/postgres.go`

Keep:

- `NamedPostgresParams` Fx inputs.
- `NamedPostgresPools` Fx outputs.
- Use of `datastore.NewPostgresPools`.
- Requested resource names: `resources.NameUserDB`, `resources.NameCommonDB`.
- Fx output tags: `name:"user_db"` and `name:"common_db"`.

The provider must not initialize any future or unused configured database such as `pay_db`.

### Redis provider

Move:

- From `user-service/internal/bootstrap/redis.go`
- To `user-service/internal/providers/redis.go`

Keep:

- `NamedRedisParams` Fx inputs.
- `NamedRedisClients` Fx output.
- Use of `datastore.NewRedisClient`.
- Requested resource name: `resources.NameCacheRedis`.
- Fx output tag: `name:"cache_redis"`.

The provider must not initialize unrelated Redis instances that may appear in config.

### Ent provider

Move:

- From `user-service/internal/bootstrap/ent.go`
- To `user-service/internal/providers/ent.go`

Keep:

- `NamedEntClientParams` and `NamedEntClients`.
- Fx input tags for `user_db` and `common_db` SQL pools.
- Fx output tags for `user_db` and `common_db` Ent clients.
- `entsql.OpenDB(dialect.Postgres, db)`.
- `nonClosingEntDriver` to prevent Ent client close from closing datastore-owned SQL pools.
- Fx `OnStop` hook that closes both Ent clients.
- `errors.Join` based named close error aggregation.

No Ent schema, generated code or migration changes are needed.

## Bootstrap Package

After migration, `bootstrap` should contain:

- `app.go`: `NewApp` and `AppModule`.
- `server.go`: HTTP server construction, lifecycle hook, serve loop and shutdown helpers.
- Tests that exercise `NewApp`, `AppModule` smoke behavior, and HTTP server lifecycle.

`bootstrap` should not contain:

- Gin engine construction.
- Route registration adapter.
- JWT service construction.
- Redis/PostgreSQL provider output structs.
- Ent client provider and non-closing Ent driver.

`bootstrap` may still import `net/http` because the HTTP server is intentionally owned there.

## Router Package

`internal/router` should remain focused on HTTP route definition:

- `router.go`: Gin engine/system route registration entrypoint and feature route grouping.
- `system.go` or `health.go`: health route handler.
- `swagger.go`: Swagger route behavior.

If the existing system route file is named `system.go`, do not rename it only to match a desired tree unless there is a clear implementation reason. Avoid churn that does not help the provider split.

## Tests

Provider tests should move with the provider code:

- `bootstrap/ent_test.go` -> `providers/ent_test.go`
- `bootstrap/postgres_test.go` -> `providers/postgres_test.go`
- `bootstrap/validation_test.go` provider validation cases -> `providers/validation_test.go` or split into focused test files.
- Gin or route tests currently in `bootstrap/http_test.go` should be split: provider-specific assertions move to `providers`; HTTP server lifecycle assertions remain in `bootstrap`.

Expected test updates:

- Package declarations change from `package bootstrap` or `bootstrap_test` to `package providers` or `providers_test` as appropriate.
- Tests that need unexported helpers such as `closeEntClients` can use `package providers`.
- Tests that verify public provider behavior can use `package providers_test`.
- Import paths updated from `internal/bootstrap` to `internal/providers` where tests are external package tests.

Do not weaken existing lifecycle/error coverage during the move.

## Documentation Updates

Update long-lived docs:

- `AGENTS.md`
  - Add provider entry points under Key Entry Points.
  - Change bootstrap description so it owns app/server lifecycle, not service infrastructure provider implementations.
  - Add `user-service/internal/providers/` to Repository Shape or rules.
- `docs/ARCHITECTURE.md`
  - Update Runtime Flow step that currently says `bootstrap.AppModule` explicitly provides Redis/PostgreSQL/Ent/Gin/routes.
  - Update HTTP Request Flow middleware and route assembly locations from `bootstrap` to `providers` where appropriate.
  - Update Infrastructure section so Ent clients are provided by `internal/providers/ent.go`.
  - Add a short boundary statement for `internal/providers`.
- `docs/DEVELOPMENT.md`
  - Update coding conventions to mention service-level provider code belongs in `user-service/internal/providers`.

Do not reintroduce OpenSpec/OPSX references or artifacts.

## Compatibility

This is an internal package move inside `user-service`; no external API compatibility shim is needed.

Because these packages live under `internal/`, consumers outside the module cannot import them. In-repo references should migrate directly to `providers`. Keeping forwarding functions in `bootstrap` would preserve the old boundary and weaken the acceptance criteria, so the implementation should remove the migrated provider files from bootstrap rather than leaving aliases behind.

## Verification Strategy

After implementation, run:

```bash
rg -n "func NewGinEngine|func RegisterRoutes|func NewJWTService|func ProvidePostgresPools|func ProvideRedisClients|func ProvideEntClients|type Named(Postgres|Redis|Ent)" user-service/internal/bootstrap
```

This should return no provider implementations from `bootstrap`.

Run:

```bash
rg -n "package providers" user-service/internal/providers
rg -n "providers\\.Module|internal/providers" user-service/internal docs AGENTS.md
```

These should show the new provider package and updated references.

Then run:

```bash
cd common && go test ./...
cd ../user-service && go test ./...
```

If failures appear, inspect import cycles first. The intended dependency path is `bootstrap -> providers -> router/features`, with no `providers -> bootstrap` import.
