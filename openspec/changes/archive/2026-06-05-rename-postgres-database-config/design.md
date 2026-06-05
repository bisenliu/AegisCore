## Context

`shared-infrastructure` 负责 `common/runtime/config` 配置对象、具名 Redis/PostgreSQL 配置读取，以及 `common/runtime/datastorefx` 中单实例连接池 provider 的装配。当前 PostgreSQL 运行时连接配置导出为 `PostgresDatabaseConfig`，并通过 `Config.PostgresDatabase(name string)` 获取；这两个名称与 `PostgresConfig`、`RedisConfig` 等配置对象命名不够对称，也没有明确表达 accessor 返回的是数据库运行时配置。

本变更是跨 `common` 与 `user-services` 引用点的 Go API 命名重构，但不改变 controller/service/repository 分层，不改变配置文件结构，不改变 Fx provider 创建连接池的行为。

## Goals / Non-Goals

**Goals:**

- 将 `PostgresDatabaseConfig` 重命名为 `PostgresDBConfig`。
- 将 `Config.PostgresDatabase(name string)` 重命名为 `Config.PostgresDatabaseConfig(name string)`，并返回 `(PostgresDBConfig, bool)`。
- 更新项目内所有旧符号引用，包括代码、测试、注释和文档。
- 保持 YAML key、`AEGISCORE_` 环境变量、DSN 组装、连接池参数和 ping timeout 行为不变。
- 通过 `common` 与 `user-services` 模块测试确认编译通过。

**Non-Goals:**

- 不新增旧符号 alias 或兼容 wrapper；本仓库内调用方统一迁移到新名称。
- 不修改 `PostgresConfig` 字段、配置 tag、环境变量映射或配置加载校验策略。
- 不修改 datastore provider lifecycle、Ent client wiring、HTTP API、数据库 schema 或 Atlas migration。
- 不手写或重新生成 `user-services/ent/` 下的 Ent 生成代码。

## Decisions

- 直接重命名导出符号，不保留兼容层。理由：该变更面向当前 workspace 内部代码，旧名称会继续制造歧义；保留 alias 会扩大 API 表面积。备选方案是保留 `type PostgresDatabaseConfig = PostgresDBConfig` 和旧方法 wrapper，但这会削弱命名清理目标。
- 只修改 `common/runtime/config` 的类型和 accessor，再按编译错误或全局搜索更新调用方。理由：最小化行为变更风险，并确保 `common/runtime/datastorefx` 仍从同一配置对象读取同一字段。备选方案是顺带重命名 `PostgresConfig` 或 YAML 字段，但这会触及外部配置契约，超出目标。
- 使用现有测试命令验证两个 Go module。理由：符号重命名可能同时影响共享包和用户服务 bootstrap；分别运行 `go test ./...` 能覆盖编译和现有行为测试。备选方案是只运行 `common` 测试，但可能漏掉 `user-services` 中对新 accessor 的引用。

## Risks / Trade-offs

- 导出 Go API 重命名会破坏尚未迁移的外部消费者。缓解：本次范围限定为仓库内引用；如未来存在外部 module 依赖，需要由对应版本策略处理。
- 全局引用更新遗漏会导致编译失败。缓解：使用搜索覆盖旧类型名和旧方法名，并运行 `common`、`user-services` 全量测试。
- 命名重构误触配置契约会导致部署配置不兼容。缓解：不得修改 `mapstructure` tag、YAML key、环境变量前缀、DSN 组装逻辑和 provider lifecycle。
