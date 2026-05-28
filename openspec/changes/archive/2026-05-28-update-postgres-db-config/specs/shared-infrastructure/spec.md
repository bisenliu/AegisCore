## MODIFIED Requirements

### Requirement: Load configuration from YAML and environment

系统必须从 YAML 配置文件加载运行时配置，并支持 `AEGISCORE_` 前缀的环境变量覆盖。PostgreSQL 配置必须使用共享连接参数和数据库名称字段，而不是按数据库重复声明完整 DSN。

#### Scenario: Load explicit config file
- **Given** 调用方提供配置文件路径
- **When** `common/config.Load` 被调用
- **Then** 系统读取该 YAML 文件
- **Then** 系统应用默认值并反序列化为 `config.Config`
- **Then** 系统验证配置结构

#### Scenario: Override config with environment variable
- **Given** 环境变量使用 `AEGISCORE_` 前缀
- **When** 环境变量 key 对应配置路径中的 `.` 或 `-`
- **Then** 系统将其映射为 `_` 并覆盖配置值

#### Scenario: Load PostgreSQL shared connection fields
- **Given** YAML 配置包含 `database.postgres.host`、`port`、`username`、`password`、`user_db_name`、`pay_db_name` 和 `common_db_name`
- **When** `common/config.Load` 被调用
- **Then** 系统必须加载这些 PostgreSQL 字段到配置对象
- **Then** 系统必须为 PostgreSQL driver、连接池大小和 ping timeout 应用默认值

#### Scenario: Missing PostgreSQL required field fails validation
- **Given** PostgreSQL 配置缺少 `host`、`port`、`username`、`user_db_name` 或 `common_db_name` 中任一必填字段
- **When** 系统验证配置结构
- **Then** 配置加载必须返回错误，指出对应 `database.postgres.<field>` 必填或无效

### Requirement: Provide shared runtime dependencies through Fx

系统必须通过 `common/infrastructure.Module` 提供配置、日志、Redis 和 PostgreSQL 连接池。PostgreSQL 连接池必须基于共享连接参数和目标数据库名称构造连接，而不是读取每个数据库的独立 DSN。

#### Scenario: Provide common dependencies
- **Given** Fx app 包含 `common/infrastructure.Module`
- **When** Fx 解析依赖
- **Then** module 提供 `*config.Config`、`*slog.Logger`、`*redis.Client` 和具名 PostgreSQL 连接池

#### Scenario: Register PostgreSQL lifecycle
- **Given** 配置中存在 PostgreSQL 共享连接参数以及 `user_db_name` 和 `common_db_name`
- **When** Fx app 启动
- **Then** 系统创建两个具名 `*sql.DB` 连接池：`user_db` 和 `common_db`
- **Then** `user_db` 连接池必须连接到 `user_db_name` 指定的数据库
- **Then** `common_db` 连接池必须连接到 `common_db_name` 指定的数据库
- **Then** 启动时 ping 每个数据库
- **Then** 停止时关闭每个数据库连接池

#### Scenario: Pay database name is configurable without adding payment runtime dependency
- **Given** 配置中存在 `pay_db_name`
- **When** `common/infrastructure.Module` 提供当前用户服务运行时依赖
- **Then** 系统必须允许配置对象读取 `pay_db_name`
- **Then** 系统不得仅因为存在 `pay_db_name` 而创建支付业务 API、支付 repository 或支付 Ent client

#### Scenario: Register Redis lifecycle
- **Given** 配置中存在 Redis 地址和超时设置
- **When** Fx app 启动
- **Then** 系统创建 Redis client 并执行 ping
- **Then** Fx app 停止时关闭 Redis client
