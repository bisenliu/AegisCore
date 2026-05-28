## MODIFIED Requirements

### Requirement: Load configuration from YAML and environment

系统必须从 YAML 配置文件加载运行时配置，并支持 `AEGISCORE_` 前缀的环境变量覆盖。主要运行时配置必须由 YAML 或环境变量显式提供，系统不得通过代码默认值补齐缺失的主要配置。PostgreSQL 配置必须使用共享连接参数和数据库名称字段，而不是按数据库重复声明完整 DSN。

#### Scenario: Load explicit config file
- **Given** 调用方提供配置文件路径
- **When** `common/config.Load` 被调用
- **Then** 系统读取该 YAML 文件
- **Then** 系统将 YAML 与 `AEGISCORE_` 环境变量覆盖反序列化为 `config.Config`
- **Then** 系统验证配置结构

#### Scenario: Override config with environment variable
- **Given** 环境变量使用 `AEGISCORE_` 前缀
- **When** 环境变量 key 对应配置路径中的 `.` 或 `-`
- **Then** 系统将其映射为 `_` 并覆盖配置值

#### Scenario: Missing primary configuration fails validation
- **Given** YAML 和环境变量未显式提供 app、HTTP、log、Redis 或 PostgreSQL 的主要运行时配置字段
- **When** `common/config.Load` 被调用
- **Then** 配置加载必须返回错误，指出缺失或无效的配置字段
- **Then** 服务启动必须在依赖初始化前失败

#### Scenario: Load PostgreSQL shared connection fields
- **Given** YAML 配置包含 `database.postgres.host`、`port`、`username`、`password`、`user_db_name`、`pay_db_name` 和 `common_db_name`
- **When** `common/config.Load` 被调用
- **Then** 系统必须加载这些 PostgreSQL 字段到配置对象
- **Then** PostgreSQL driver、连接池大小和 ping timeout 等运行时参数必须来自 YAML 或 `AEGISCORE_` 环境变量覆盖

#### Scenario: Missing PostgreSQL required field fails validation
- **Given** PostgreSQL 配置缺少 `host`、`port`、`username`、`user_db_name` 或 `common_db_name` 中任一必填字段
- **When** 系统验证配置结构
- **Then** 配置加载必须返回错误，指出对应 `database.postgres.<field>` 必填或无效
