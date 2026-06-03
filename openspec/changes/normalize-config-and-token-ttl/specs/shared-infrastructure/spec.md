## ADDED Requirements

### Requirement: Use symmetric datastore collection names in config object

共享配置对象中 Redis 与 PostgreSQL 命名实例集合的 Go 字段命名 SHALL 保持对称。Redis 集合字段 MUST 命名为 `Redis`，PostgreSQL 集合字段 MUST 命名为 `Postgres`，且 PostgreSQL 字段 MUST 继续使用 `mapstructure:"postgres"` 映射外部配置。

#### Scenario: Load PostgreSQL named instances into Postgres field
- **GIVEN** YAML 配置包含 `postgres.user_db` 命名实例
- **WHEN** `common/config.Load` 被调用
- **THEN** 系统 MUST 将该实例反序列化到 `config.Config.Postgres["user_db"]`
- **THEN** 系统 MUST 保持配置路径为 `postgres.user_db`

#### Scenario: Override PostgreSQL named instances after field rename
- **GIVEN** YAML 配置包含 `postgres.user_db` 命名实例
- **GIVEN** 环境变量提供 `AEGISCORE_POSTGRES_USER_DB_PASSWORD`
- **WHEN** `common/config.Load` 被调用
- **THEN** 系统 MUST 将环境变量覆盖应用到 `config.Config.Postgres["user_db"].Password`
- **THEN** 系统 MUST NOT 要求调用方使用新的 YAML key 或新的环境变量前缀

#### Scenario: Runtime PostgreSQL helper uses renamed field
- **GIVEN** 配置对象包含 `config.Config.Postgres["user_db"]`
- **WHEN** 共享基础设施通过 PostgreSQL 命名实例 helper 创建 `user_db` 连接池
- **THEN** 系统 MUST 从 `Config.Postgres` 读取该实例配置
- **THEN** 连接池 driver、DSN、连接池参数和 ping timeout 行为 MUST 与重命名前保持一致
