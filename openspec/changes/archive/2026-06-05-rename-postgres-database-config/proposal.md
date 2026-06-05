## Why

`common/runtime/config` 中 PostgreSQL 运行时数据库配置的导出名称同时使用 `PostgresDatabaseConfig` 和 `PostgresDatabase`，与现有 `RedisConfig`、`PostgresConfig` 命名风格不一致，容易让调用方误解类型与 accessor 的职责。现在进行仅命名层面的重构，可以在不改变配置加载、DSN 生成或运行时行为的前提下提升共享基础设施 API 的一致性。

## What Changes

- 将 `common/runtime/config.Config` 返回的 PostgreSQL 数据库运行时配置类型从 `PostgresDatabaseConfig` 重命名为 `PostgresDBConfig`。
- 将 `Config.PostgresDatabase(name string)` accessor 重命名为 `Config.PostgresDatabaseConfig(name string)`，返回 `(PostgresDBConfig, bool)`。
- 同步更新项目内所有代码、注释、文档和测试中的旧类型名与旧方法名引用。
- 保持 PostgreSQL 命名实例、DSN 生成、连接池字段、配置文件结构和环境变量覆盖行为不变。

## Capabilities

### New Capabilities

无。

### Modified Capabilities

- `shared-infrastructure`: 调整 common 配置包中 PostgreSQL 运行时数据库配置的 Go API 命名，保持共享基础设施行为不变。

## Impact

- 主要影响代码：`common/runtime/config/config.go`、`common/runtime/datastorefx/`、`user-services/internal/bootstrap/` 以及所有测试或文档中的旧符号引用。
- Go API 影响：项目内调用方需要使用 `PostgresDBConfig` 和 `PostgresDatabaseConfig(name)`；这是导出符号重命名，未计划保留旧名称兼容层。
- 外部可观察行为：HTTP API、错误码、配置 YAML key、环境变量、数据库 schema 和 migration 均不改变。
- 依赖影响：不新增第三方依赖，不修改 Ent 生成代码或 Atlas migration。
