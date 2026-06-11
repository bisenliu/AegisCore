# Split service providers from bootstrap

## What

将 `user-service/internal/bootstrap` 中的服务级基础设施 provider 拆到新的 `user-service/internal/providers` 包，使 bootstrap 只负责 Fx app 创建、`AppModule` 总装和 HTTP server 生命周期。

目标结构：

```text
user-service/internal/
  bootstrap/
    app.go
    server.go
  providers/
    auth.go
    ent.go
    gin.go
    postgres.go
    redis.go
    routes.go
  features/
    user/
    auth/
  router/
    router.go
    system.go
    swagger.go
```

本变更迁移以下 provider：

- Gin engine provider。
- 用户服务 HTTP route registration provider。
- JWT service provider。
- PostgreSQL named pool provider。
- Redis named client provider。
- Ent named client provider。

`bootstrap.AppModule` 改为通过 `providers.Module` 或 `providers.New...` 组装服务级基础设施；feature modules 仍由 `features/user` 和 `features/auth` 自己提供。`router` 包继续只负责 HTTP route 定义、系统路由和 Swagger 路由，不承载 Fx provider。

## Why

当前 `bootstrap` 同时包含 Fx app 入口、HTTP server 生命周期，以及 Gin、routes、JWT、PostgreSQL、Redis、Ent provider 实现细节。这个包已经承担了两类责任：

- 启动壳：创建 `fx.New`、声明顶层 `AppModule`、管理 HTTP server 生命周期。
- 服务级 provider：把用户服务需要的运行时基础设施适配成 Fx provider。

随着 feature-first 分层稳定下来，继续把 provider 细节放在 bootstrap 会让启动壳成为服务级基础设施的兜底目录。拆出 `internal/providers` 后，bootstrap 可以保持很薄，只表达“服务由哪些模块组成”；provider 包则集中承载“用户服务如何把共享 runtime、common security、Ent、Gin 和 router 适配为 Fx 依赖”。

这样后续新增服务级 provider 时有明确落点，也能避免把 provider 实现误放进 `router`、feature module 或 `common`。

## Scope

包括：

- 新增 `user-service/internal/providers` 包。
- 将 `bootstrap/gin.go` 迁移为 `providers/gin.go`，保留 trusted proxies、中间件顺序和 Gin release mode 行为。
- 将 `bootstrap/routes.go` 迁移为 `providers/routes.go`，保留 `/healthz`、Swagger、`/api/v1`、认证中间件和 feature route registration 的外部行为。
- 将 `bootstrap/auth.go` 迁移为 `providers/auth.go`，保留 `auth.NewJWTService(cfg.Auth)` 语义。
- 将 `bootstrap/postgres.go` 迁移为 `providers/postgres.go`，保留 `postgres.user_db`、`postgres.common_db` 配置读取和 `user_db`、`common_db` Fx output names。
- 将 `bootstrap/redis.go` 迁移为 `providers/redis.go`，保留 `redis.cache_redis` 配置读取和 `cache_redis` Fx output name。
- 将 `bootstrap/ent.go` 迁移为 `providers/ent.go`，保留 Ent client 包装、non-closing driver、关闭 hook 和具名关闭错误。
- 在 `providers` 中提供集中组装入口，例如 `providers.Module`，供 `bootstrap.AppModule` 引入。
- 更新 `user-service/internal/bootstrap` 与相关测试中的 package/import/call site。
- 迁移或拆分 bootstrap 测试，使 provider 行为测试落到 `internal/providers`，HTTP server 生命周期测试留在 `internal/bootstrap`。
- 更新 `AGENTS.md`、`docs/ARCHITECTURE.md` 和 `docs/DEVELOPMENT.md` 中关于 bootstrap、providers、router 和关键入口的说明。

不包括：

- 不移动 feature 内部业务代码。
- 不改变 HTTP API、route path、响应信封、认证中间件语义或 Swagger 路由行为。
- 不改变 YAML 配置 key、`AEGISCORE_` 环境变量覆盖规则、Redis/PostgreSQL named resource。
- 不改变 Ent schema、Ent generated code 或 Atlas migration。
- 不引入 tracing、metrics、transaction、eventbus 的真实实现。
- 不将服务特定 provider 上移到 `common`。
- 不新增 `openspec/` 或 `docs/opsx/` 工件。

## Acceptance Criteria

- `user-service/internal/bootstrap` 中只保留 `app.go`、`server.go` 以及必要的 bootstrap 测试。
- `user-service/internal/providers` 中集中提供 Gin、routes、JWT、PostgreSQL、Redis 和 Ent provider。
- `bootstrap.AppModule` 通过 `providers.Module` 或显式 provider 引用完成服务级基础设施组装，不直接包含数据库、Redis、Ent、Gin provider 实现细节。
- `router` 包不新增 Fx provider，只保留 HTTP route 定义和系统路由能力。
- Feature modules 仍由 `features/user` 与 `features/auth` 自己提供，不被迁移到 `providers`。
- Redis provider 仍读取 `redis.cache_redis`，Fx output name 仍为 `cache_redis`。
- PostgreSQL provider 仍读取 `postgres.user_db`、`postgres.common_db`，Fx output names 仍为 `user_db`、`common_db`。
- Ent clients 仍使用 SQL pool 支撑，且不会重复关闭由 datastore lifecycle 管理的 SQL pool。
- 文档同步反映 `bootstrap`、`providers`、`router` 的新职责边界。
- `common/` 下 `go test ./...` 通过。
- `user-service/` 下 `go test ./...` 通过。
