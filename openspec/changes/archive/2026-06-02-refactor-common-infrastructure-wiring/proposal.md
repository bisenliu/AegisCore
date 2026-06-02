## Why

当前 `common/infrastructure.Module` 隐式提供公共配置和日志依赖，而服务侧已经需要显式声明自身的 PostgreSQL、Redis 和 Ent 运行时依赖。将公共依赖也改为服务侧手动依次注入，可以让服务启动装配更直观，并避免共享模块把不同基础设施 provider 组织在一起。

`ProvideNamedRedis` 目前放在 `postgres.go` 中，和文件职责不一致。调整 provider 所属文件可以降低维护时的误读风险。

## What Changes

- **BREAKING** 移除 `common/infrastructure.Module`，服务启动装配不得再依赖共享 Fx module。
- 用户服务在 `bootstrap.NewApp` 中显式提供 `commoninfra.NewConfig` 和 `commoninfra.NewLogger`，并继续显式声明自身需要的 Redis/PostgreSQL/Ent 依赖。
- 将 `ProvideNamedRedis` 从 `common/infrastructure/postgres.go` 移到 Redis 相关文件中；`ProvideNamedPostgres` 保留在 PostgreSQL 相关文件中。
- 更新相关测试、架构文档、能力地图和 OpenSpec 主规格，反映公共依赖由服务侧手动注入。
- 保持现有运行时配置键、HTTP API、响应信封、错误码和数据模型不变。

## Capabilities

### New Capabilities

无。

### Modified Capabilities

- `shared-infrastructure`: 移除通过 `common/infrastructure.Module` 提供配置和 Zap logger 的要求，改为要求服务侧显式注入公共配置、日志和具名数据存储依赖；同时保持具名 Redis/PostgreSQL provider 按需声明的约束。
- `http-service-runtime`: 用户服务启动装配从引入共享基础设施 module 改为手动依次提供公共配置、日志以及服务声明的运行时依赖。

## Impact

- 影响代码：`common/infrastructure/module.go`、`common/infrastructure/postgres.go`、`common/infrastructure/redis.go`、`user-services/internal/bootstrap/bootstrap.go`、相关测试。
- 影响文档：`docs/ARCHITECTURE.md`、`docs/opsx/CAPABILITY_MAP.md`。
- 兼容性：对 HTTP API、错误码、配置字段、数据库 schema 和外部数据模型无影响。
- 依赖注入兼容性：依赖 `commoninfra.Module` 的内部服务装配需要迁移为显式 `fx.Provide(commoninfra.NewConfig, commoninfra.NewLogger)` 或等价写法。
