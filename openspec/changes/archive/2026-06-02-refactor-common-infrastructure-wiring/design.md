## Context

`common/infrastructure.Module` 当前只包装 `NewConfig` 和 `NewLogger`，但它作为共享 Fx module 出现在用户服务启动图中。与此同时，用户服务已经在自身 bootstrap module 中显式声明 `cache_redis`、`user_db`、`common_db` 和 Ent clients，说明运行时依赖的实际边界已经偏向服务侧手动装配。

`ProvideNamedRedis` 当前定义在 `common/infrastructure/postgres.go`，与 `NewRedisClient` 所在的 Redis 文件分离。该 helper 虽然行为正确，但文件组织会让 Redis provider 的维护入口不清晰。

受影响边界包括 `common/infrastructure` 的 provider 组织、`user-services/internal/bootstrap` 的 Fx 装配、共享基础设施测试，以及描述长期能力的 docs/OpenSpec 文档。不涉及 controller/service/repository 分层、HTTP API、Ent schema 或 Atlas migration。

## Goals / Non-Goals

**Goals:**

- 删除 `common/infrastructure.Module`，让服务侧在自己的启动装配中显式提供 `NewConfig` 和 `NewLogger`。
- 保持 `common` 继续提供共享配置、日志、Redis/PostgreSQL 单实例创建能力和 opt-in 命名 Fx helper。
- 将 `ProvideNamedRedis` 移到 Redis 相关文件中，保持 `ProvideNamedPostgres` 在 PostgreSQL 相关文件中。
- 更新测试、架构文档、能力地图和主规格，使长期契约与代码结构一致。
- 保持用户服务只连接自己声明的 `cache_redis`、`user_db` 和 `common_db`。

**Non-Goals:**

- 不新增认证、支付、健康聚合或新的业务 API。
- 不修改配置字段名、HTTP 响应契约、错误码、数据库 schema 或迁移流程。
- 不手写或重新生成 `user-services/ent/` 下的 Ent 生成代码。
- 不把共享基础设施实现复制到 `user-services`；服务侧只负责组合已有共享 provider。

## Decisions

1. 删除共享基础设施聚合 module，而不是保留空 module 或重命名 module。

理由：用户要求移除该文件，并且 module 只提供两个公共依赖。直接删除能避免调用方继续依赖隐式聚合入口。替代方案是保留 `Module` 但只作为兼容 alias；这会削弱服务侧手动注入的目标，因此不采用。

2. 用户服务在 `bootstrap.NewApp` 中显式提供公共依赖。

实现上由 `bootstrap.NewApp(configPath)` 继续 `fx.Supply(commoninfra.ConfigPath(configPath))`，然后通过 `fx.Provide(commoninfra.NewConfig, commoninfra.NewLogger)` 或等价顺序提供配置和 Zap logger，再引入用户服务自己的 bootstrap module。这样仍由 `common` 包实现配置和日志逻辑，服务侧只表达依赖顺序和选择。

3. Redis 命名 Fx helper 与 Redis runtime factory 放在同一职责文件。

`ProvideNamedRedis` 调用 `NewRedisClient` 并返回具名 `*redis.Client`，应放在 `redis.go` 或新的 Redis provider 文件中。`postgres.go` 只保留 PostgreSQL 相关 provider、factory 和 lifecycle。这样不改变导出 API，只改变文件组织。

4. 测试从“共享 module 不提供命名数据存储”迁移为“服务侧显式公共 provider 不产生额外数据存储”。

删除 `Module` 后，原有针对 `Module` 的测试不再成立。应保留和调整 `ProvideNamedPostgres`、`ProvideNamedRedis` 的 opt-in 行为测试，并增加或更新 bootstrap/Fx 测试来证明显式 `NewConfig`、`NewLogger` 不会自动创建 Redis/PostgreSQL 实例。

5. 文档和主规格同步更新长期契约。

`shared-infrastructure` 主规格需要修改 “Provide shared runtime dependencies through Fx” 要求，移除 `common/infrastructure.Module` 的强制表述。`http-service-runtime` 主规格需要补充用户服务启动图必须显式提供公共配置和日志依赖。

## Risks / Trade-offs

- [Risk] 其他内部服务或测试仍引用 `commoninfra.Module` 导致编译失败。→ 通过全仓搜索并迁移所有引用，运行 `go test ./...` 覆盖两个 Go module。
- [Risk] 删除 module 后 logger 生命周期同步被遗漏。→ 在服务侧显式提供 `commoninfra.NewLogger`，并通过测试验证 Fx stop 仍触发 logger lifecycle。
- [Risk] 移动 `ProvideNamedRedis` 时遗漏 `redis` import 调整。→ 保持导出函数签名不变，仅在文件间迁移实现并运行 `gofmt` 与测试。
- [Risk] 规格归档后丢失旧 requirement 的其他场景。→ delta spec 使用完整 MODIFIED requirement block，保留原有 Redis/PostgreSQL opt-in、未声明实例不连接、pay_db 不自动连接等场景。

## Migration Plan

1. 在 `user-services/internal/bootstrap/bootstrap.go` 中将 `commoninfra.Module` 替换为显式 `fx.Provide(commoninfra.NewConfig, commoninfra.NewLogger)`。
2. 将 `ProvideNamedRedis` 从 `postgres.go` 移到 Redis 相关文件，修正 import。
3. 删除 `common/infrastructure/module.go`。
4. 更新或删除引用 `commoninfra.Module` 的测试，并保留 opt-in 命名 datastore helper 覆盖。
5. 更新 `docs/ARCHITECTURE.md`、`docs/opsx/CAPABILITY_MAP.md` 和相关 OpenSpec 主规格 delta。
6. 运行 `gofmt`，分别在 `common/` 和 `user-services/` 执行 `go test ./...`。

Rollback 可通过恢复 `common/infrastructure.Module` 文件并将用户服务 bootstrap 装配改回引入该 module 完成；由于不涉及数据模型和外部 API，无需数据库或客户端迁移。

## Open Questions

无。
