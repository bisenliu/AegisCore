## Why

`common/runtime/datastore` 当前将 PostgreSQL 基础构造、批量资源 ownership 和 Fx lifecycle 绑定在同一 API 中。普通 Go 调用方必须提供 `fx.Lifecycle` 和整份 `PostgresConfigs`，单资源构造也通过 map result 的批量入口间接完成，弱化了服务 composition 对真实资源的显式选择。

本变更以不兼容方式建立框架无关的单资源 PostgreSQL constructor 和具名 Ping/Close 契约，使 user-service 显式选择并构造 `primary_db`，为未来按真实需求增加其他资源提供清晰前置。

## What Changes

- **BREAKING** 删除核心 `datastore.NewPostgresPools`、批量 map result 和接收 `fx.Lifecycle`、`PostgresConfigs` 的旧 `datastore.NewPostgres`。
- 新增只接收资源名称和单份 `PostgresConfig` 的框架无关 `datastore.OpenPostgres`，直接返回单个 `*sql.DB`，并提供具名启动 `Ping` 和 `Close` 契约。
- 将 PostgreSQL Fx lifecycle adapter 与框架无关 constructor 分离，并作为 `postgres_fx.go` 共置在 `common/runtime/datastore`；Redis Fx adapter 保留相同的包内共置风格。
- 保留 `PostgresConfigs` 作为服务配置 map，但 common constructor 不遍历该 map。
- 将 user-service PostgreSQL composition 改为显式 `NewPrimaryDB`，由服务选择 `primary_db` 的单份配置。
- 将 Redis Fx constructor 改为接收单份 `RedisConfig`，并由 user-service 的 `NewCacheRedis` 显式选择 `cache_redis`。

## Capabilities

### Modified Capabilities

- `shared-platform-primitives`: 将共享 PostgreSQL Fx provider 行为改为框架无关的单资源 constructor、Ping、Close、失败回滚和 Fx adapter 边界契约。

## Impact

- Go API：影响 `common/runtime/datastore` 的 PostgreSQL 导出 API，并迁移 datastore Fx adapter 的包路径。
- user-service：影响 PostgreSQL 和 Redis provider composition 及直接测试，不改变 `primary_db` 资源名称或 Fx result name。
- 配置：保留 `PostgresConfigs`、现有字段、默认值和校验，不改变 YAML 或环境变量契约。
- 数据库与 API：不影响 Ent schema、Atlas migration、真实数据库内容、HTTP API 或 OpenAPI 生成物。
