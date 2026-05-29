## MODIFIED Requirements

### Requirement: Load configuration from YAML and environment

系统必须从 YAML 配置文件加载运行时配置，并支持 `AEGISCORE_` 前缀的环境变量覆盖。配置加载必须将 YAML 与环境变量覆盖反序列化为 `config.Config`，但不得执行 required/optional、字段存在性或基础取值范围校验。Redis 与 PostgreSQL 配置必须支持按名称声明多个实例；PostgreSQL 配置必须按实例声明连接参数和数据库名称字段，而不是通过共享字段加固定数据库名字段或完整 DSN 表达。

#### Scenario: Load explicit config file
- **Given** 调用方提供配置文件路径
- **When** `common/config.Load` 被调用
- **Then** 系统读取该 YAML 文件
- **Then** 系统将 YAML 与 `AEGISCORE_` 环境变量覆盖反序列化为 `config.Config`
- **Then** 系统不得执行 required/optional、字段存在性或基础范围校验

#### Scenario: Override config with environment variable
- **Given** 环境变量使用 `AEGISCORE_` 前缀
- **When** 环境变量 key 对应配置路径中的 `.` 或 `-`
- **Then** 系统将其映射为 `_` 并覆盖配置值

#### Scenario: Missing primary configuration is not rejected by config loader
- **Given** YAML 和环境变量未显式提供 app、HTTP、log、Redis 命名实例或 PostgreSQL 命名实例的主要运行时配置字段
- **When** `common/config.Load` 被调用
- **Then** 配置加载不得仅因为字段缺失、为空或为零值而返回校验错误
- **Then** 后续服务启动或依赖初始化可以因实际无法启动或无法连接而失败

#### Scenario: Invalid basic values are not rejected by config loader
- **Given** YAML 配置包含零值端口、负数 Redis DB、零值连接池大小或零值 timeout
- **When** `common/config.Load` 反序列化配置
- **Then** 配置加载不得执行范围校验
- **Then** 相关错误或默认行为必须由后续运行时初始化或依赖库处理

#### Scenario: Load Redis named instances
- **Given** YAML 配置包含 `redis.cache_redis` 和 `redis.queue_redis` 命名实例
- **When** `common/config.Load` 被调用
- **Then** 系统必须加载每个 Redis 实例的 addr、username、password、db 和 timeout 字段到配置对象
- **Then** 每个 Redis 实例必须能独立覆盖 db 和 timeout 等运行时参数

#### Scenario: Load PostgreSQL named instances
- **Given** YAML 配置包含 `postgre.user_db`、`postgre.pay_db` 和 `postgre.common_db` 命名实例
- **When** `common/config.Load` 被调用
- **Then** 系统必须加载每个 PostgreSQL 实例的 host、port、username、password、db_name、driver、sslmode、连接池和 ping timeout 字段到配置对象
- **Then** 每个 PostgreSQL 实例必须能独立覆盖连接池和 timeout 等运行时参数

#### Scenario: Core dependency startup failure remains fatal
- **Given** 配置加载成功
- **Given** Redis、`user_db`、`common_db` 或 HTTP server 在启动时不可用或配置无法被底层依赖接受
- **When** 服务启动初始化共享基础设施或 HTTP server
- **Then** 对应初始化必须返回错误并终止服务启动
- **Then** 错误必须保留失败的依赖名称、类型或底层错误上下文
