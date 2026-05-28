## Context

当前 `common/config.Config` 将 Redis 建模为单个 `redis` 对象，将 PostgreSQL 建模为 `database.postgres` 共享连接参数加 `user_db_name`、`pay_db_name`、`common_db_name`。`common/infrastructure.Module` 固定提供 `*redis.Client`，而 PostgreSQL 已经通过 `common/infrastructure.NewPostgres` 保留基础创建代码，并由 `user-services/internal/bootstrap.NewPostgresPools` 选择 `user_db` 与 `common_db`。

本次变更要把 Redis 调整为与 PostgreSQL 相同的模式：`common` 只保留基础配置类型和 client 创建/lifecycle 代码，具体服务在自己的 Fx module 中选择要使用的命名实例。同时，PostgreSQL 配置也从共享字段加数据库名字段调整为命名实例对象，让每个实例可以覆盖连接池和 timeout 等参数。

## Goals / Non-Goals

**Goals:**

- 支持 YAML 中通过 anchor/merge 定义 Redis 与 PostgreSQL 基础模板，并在 `redis.<name>`、`postgre.<name>` 下声明命名实例。
- 让 `common/config` 加载和校验 Redis/PostgreSQL 命名实例 map，并保留 `AEGISCORE_` 环境变量覆盖能力。
- 让 `common/infrastructure` 提供 Redis 和 PostgreSQL 单实例创建函数，负责 open/ping/close lifecycle。
- 让 `user-services` 在自己的 module 中声明需要的 Redis/PostgreSQL 实例，避免 common module 固定连接所有配置项。
- 保持现有 controller/service/repository 分层和 HTTP 响应契约不变。

**Non-Goals:**

- 不新增支付服务运行时、支付 repository、支付 API 或支付 Ent client。
- 不新增认证、授权、健康检查聚合或业务缓存读写逻辑。
- 不修改 Ent schema，不手写 `user-services/ent/` 生成代码。

## Decisions

- 使用顶层 `redis` 与 `postgre` map 表示命名实例。这样 YAML 可以自然表达 `redis.cache_redis`、`redis.queue_redis`、`postgre.user_db`、`postgre.common_db`，也便于各服务按名称查找实例。备选方案是保留 `database.postgres` 并继续添加数据库名字段，但该方式无法让每个数据库独立覆盖连接池参数，也与 Redis 多实例需求不一致。
- 将 PostgreSQL 顶层 key 命名为 `postgre`，匹配用户期望的配置结构。代码内部类型仍可使用 `PostgresConfig` 等 Go 命名以保持语义清晰，但 mapstructure tag 必须对应 `postgre`。
- Redis provider 参考 PostgreSQL provider 设计：`common/infrastructure.NewRedisClient` 接收实例名，从 `config.Config` 中查找对应 Redis 配置并注册 lifecycle；服务模块再通过 `fx.Out` 暴露具名 client。备选方案是让 common module 提供默认 `cache_redis`，但这会再次把实例选择固定在共享模块中。
- 用户服务继续显式声明 `user_db` 和 `common_db` PostgreSQL 连接池；Redis 如果需要保留运行时 Redis 连接，则在用户服务 bootstrap 中声明 `cache_redis`，而不是由 common module 自动提供未命名 `*redis.Client`。
- DSN 仍由配置对象根据 PostgreSQL 实例字段生成，不在 YAML 中声明完整 DSN，避免重复和泄漏连接串拼接逻辑。

## Risks / Trade-offs

- 配置路径变更是破坏性变更 -> 更新示例配置、配置结构测试和启动路径，确保旧路径缺失时验证错误清晰。
- Viper 对 map、duration 与环境变量覆盖组合可能存在边界差异 -> 增加配置加载测试覆盖 YAML 命名实例和关键环境变量覆盖。
- 服务选择 Redis 实例后可能产生未使用连接 -> 仅在服务确实声明 provider 时连接；没有业务消费者时避免新增不必要声明。
- PostgreSQL 每个实例独立配置会重复 YAML 字段 -> 使用 YAML anchor/merge 作为配置文件约定，Go 代码只读取合并后的结果。

## Migration Plan

1. 更新 `user-services/configs/config.yaml`，将旧 `redis.*` 和 `database.postgres.*` 改为 `redis.<name>` 与 `postgre.<name>`。
2. 更新 `common/config` 类型、查找方法和校验逻辑，删除旧 `database.postgres.*` 路径依赖。
3. 更新 `common/infrastructure` Redis provider，使其按实例名创建 client；保留 PostgreSQL 单实例 provider 并适配新配置结构。
4. 更新 `user-services/internal/bootstrap`，由服务模块声明所需 PostgreSQL 和 Redis 实例。
5. 运行 `go test ./...` 于 `common/` 与 `user-services/` 验证配置加载、provider 和装配。

回滚时恢复旧配置文件结构、旧配置结构体和旧 provider 装配即可；不涉及数据库 schema 或数据迁移。

## Open Questions

- 用户服务当前没有业务代码直接消费 Redis；实现时需要确认是否仅保留 Redis 基础 provider，还是为了保持启动依赖显式声明 `cache_redis`。
