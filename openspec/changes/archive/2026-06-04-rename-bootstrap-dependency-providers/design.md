## Context

用户服务 bootstrap 层负责将共享配置、日志、Redis、PostgreSQL、Ent client、JWT、HTTP server、controller、service 和 repository 装配进 Fx graph。其中 `NewPostgresPools`、`NewRedisClients` 和 `NewNamedClients` 均位于 `user-services/internal/bootstrap`，并通过 `fx.In`/`fx.Out` 提供服务运行时所需的具名依赖集合。

这些函数不同于普通业务构造函数：它们不是 controller/service/repository constructor，而是在服务启动边界声明 `user_db`、`common_db`、`cache_redis` 和 Ent runtime clients。将它们命名为 `Provide*` 能让 `app.go` 中的 Fx provider 列表更清楚地区分“运行时依赖 provider”和“普通对象构造器”。

## Goals / Non-Goals

**Goals:**

- 将 `NewPostgresPools` 重命名为 `ProvidePostgresPools`。
- 将 `NewRedisClients` 重命名为 `ProvideRedisClients`。
- 将 `NewNamedClients` 重命名为 `ProvideEntClients`。
- 更新 `UserServiceModule` 的 Fx provider 列表和 bootstrap 测试引用。
- 保持 `user_db`、`common_db`、`cache_redis` 的 Fx named injection、配置路径、连接创建数量和 lifecycle 行为不变。

**Non-Goals:**

- 不把所有 `fx.Provide(...)` 中的 `New*` 都改为 `Provide*`。
- 不重命名 `NewApp`、`NewJWTService`、`NewGinEngine`、`NewHTTPServer`。
- 不重命名 controller、service、repository 层 constructor。
- 不修改 `common/infrastructure.NewPostgres`、`common/infrastructure.NewRedisClient`、`ProvideNamedPostgres` 或 `ProvideNamedRedis`。
- 不修改 Ent schema、Atlas migration、运行时配置结构、HTTP API 或响应契约。

## Decisions

- 使用 `ProvidePostgresPools`、`ProvideRedisClients`、`ProvideEntClients` 命名 bootstrap runtime dependency providers。
  - 理由：这些函数返回 `fx.Out` 依赖集合，并用于服务启动边界声明运行时依赖，`Provide` 比 `New` 更准确。
  - 替代方案保留 `New*` 被拒绝，因为 `app.go` 中无法直观看出这些函数是 named runtime dependency providers。

- 使用 `ProvideEntClients` 而不是 `ProvideNamedClients`。
  - 理由：`EntClients` 明确资源类型，`NamedClients` 过于泛化；named injection 已由 `fx.Out` struct tags 表达，函数名不需要重复 `Named`。
  - 替代方案 `ProvideNamedEntClients` 被拒绝，因为名称更长且未提供必要额外信息。

- Ent client provider 的参数、输出和私有 helper 同步补充 Ent 语义。
  - 理由：`ClientParams`、`NamedClients` 和 `newClient` 在 `ent.go` 中仍然偏泛化；改为 `NamedEntClientParams`、`NamedEntClients` 和 `newEntClient` 能与 `ProvideEntClients` 形成一致语义闭环。
  - 替代方案保留旧类型名被拒绝，因为 provider 函数已明确为 Ent clients，参数和输出类型继续使用泛化 `Client` 会降低可读性。
  - 替代方案同步重命名 PostgreSQL/Redis 类型被拒绝，因为 `NamedPostgresParams`、`NamedPostgresPools`、`NamedRedisParams` 和 `NamedRedisClients` 已经准确表达资源类型。

## Risks / Trade-offs

- [Risk] 测试或 Fx 装配仍引用旧函数名导致编译失败。→ Mitigation: 同步更新 `app.go` 和 bootstrap 测试中所有旧函数名引用，并运行 `go test ./...`。
- [Risk] 命名清理被误解为运行时依赖行为变化。→ Mitigation: delta spec 明确配置路径、Fx name tags、连接数量和 lifecycle 行为必须保持不变。
- [Risk] 改动范围扩散到业务层 constructor。→ Mitigation: 任务清单明确只修改 bootstrap 中返回 `fx.Out` 的 runtime dependency providers。

## Migration Plan

- 重命名三个 bootstrap provider 函数及其引用。
- 补充重命名 Ent client provider 的参数类型、输出类型和私有 helper。
- 更新相关测试函数名和 `fx.Provide(...)` 引用。
- 运行 gofmt。
- 在 `user-services` 模块运行 `go test ./...`。
- 回滚策略：如发现不兼容，可恢复旧函数名和引用；由于不涉及外部配置、数据库或 HTTP API，无需数据迁移。

## Open Questions

- 无。
