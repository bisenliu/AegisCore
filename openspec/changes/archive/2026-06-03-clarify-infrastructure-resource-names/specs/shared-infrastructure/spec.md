## MODIFIED Requirements

### Requirement: Centralize runtime resource name constants
系统 SHALL 在 `common` 模块集中维护共享运行时资源名称常量，用于 Redis、PostgreSQL 和 Ent runtime dependency wiring。常量 MUST 组织在职责明确的资源名文件中，并通过中文注释说明其用于 datastore 和 Ent 的 Fx wiring。常量值 MUST 与现有命名实例契约保持一致。

#### Scenario: Provide user database name constant
- **WHEN** 用户服务声明或创建 `user_db` PostgreSQL pool 或 Ent client
- **THEN** 非 struct tag 的运行时资源名称引用 MUST 使用 `common` 中的 `user_db` 公共常量
- **THEN** 该常量值 MUST 保持为 `user_db`

#### Scenario: Provide common database name constant
- **WHEN** 用户服务声明或创建 `common_db` PostgreSQL pool 或 Ent client
- **THEN** 非 struct tag 的运行时资源名称引用 MUST 使用 `common` 中的 `common_db` 公共常量
- **THEN** 该常量值 MUST 保持为 `common_db`

#### Scenario: Provide cache redis name constant
- **WHEN** 用户服务声明或创建 `cache_redis` Redis client
- **THEN** 非 struct tag 的运行时资源名称引用 MUST 使用 `common` 中的 `cache_redis` 公共常量
- **THEN** 该常量值 MUST 保持为 `cache_redis`

#### Scenario: Keep resource names in an explicit file
- **WHEN** 维护者查看 `common/infrastructure` 中的运行时资源名常量
- **THEN** 常量 MUST 位于职责明确的资源名文件中
- **THEN** 常量组 MUST 使用中文注释说明其用于 datastore 和 Ent 的 Fx wiring
- **THEN** 实现 MUST NOT 为减少文件数量而将这些跨资源常量合并进配置加载实现文件

#### Scenario: Preserve Fx name tags
- **WHEN** Go struct tag 用于 Fx named injection
- **THEN** struct tag 中的 name 值 MUST 继续匹配 `user_db`、`common_db` 或 `cache_redis`
- **THEN** 实现 MUST NOT 为替换 tag 字面量而引入改变依赖图行为的大规模 wiring 重构

#### Scenario: Preserve named datastore configuration
- **WHEN** 运行时资源名称常量迁移完成
- **THEN** 配置路径 MUST 继续使用 `postgres.user_db`、`postgres.common_db` 和 `redis.cache_redis`
- **THEN** `common/config.Load` 的读取、覆盖和反序列化行为 MUST 保持不变
