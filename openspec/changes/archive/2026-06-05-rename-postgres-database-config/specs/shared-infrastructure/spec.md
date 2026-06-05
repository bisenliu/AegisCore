## MODIFIED Requirements

### Requirement: Shared infrastructure naming cleanup preserves runtime behavior
共享基础设施相关命名标准化 SHALL 只修改低风险内部名称、公共 Go API 名称或文档表达，不得改变配置加载、Zap 日志、trace-id 边界名称、Redis provider、PostgreSQL provider 或 Ent runtime client 的行为。

#### Scenario: Shared infrastructure names are reviewed
- **WHEN** 实现审查 `common/runtime/config`、`common/runtime/infrastructure`、`common/runtime/logger`、`common/http/middleware`、`common/validation` 和服务侧基础设施 wiring 的命名
- **THEN** 实现 MUST 区分公共 Go API、内部参数名、文档表达和外部配置契约

#### Scenario: Runtime contracts are preserved
- **WHEN** 共享基础设施相关名称被标准化
- **THEN** YAML key、`AEGISCORE_` 环境变量覆盖、Redis/PostgreSQL 命名实例、`X-Trace-ID` header 和日志 `trace-id` 字段 MUST 保持不变

#### Scenario: PostgreSQL runtime config API is renamed without behavior change
- **GIVEN** 调用方需要读取 `postgres.<name>` 命名实例对应的 PostgreSQL 运行时连接配置
- **WHEN** 调用方使用 `Config.PostgresDatabaseConfig(name)` 获取配置
- **THEN** 系统 MUST 返回 `PostgresDBConfig` 和存在性标记
- **THEN** 返回的 driver、DSN、连接池参数和 ping timeout MUST 与重命名前保持一致
- **THEN** 系统 MUST NOT 要求调用方修改 YAML key、`AEGISCORE_` 环境变量或 PostgreSQL 命名实例名称
