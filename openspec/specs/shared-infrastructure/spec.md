# shared-infrastructure

## Purpose

共享基础设施能力为服务提供一致的配置加载、结构化日志、Redis client、PostgreSQL 连接池和 Ent client 装配，减少各服务重复实现运行时基础能力。

## Requirements

### Requirement: Load configuration from YAML and environment

系统必须从 YAML 配置文件加载运行时配置，并支持 `AEGISCORE_` 前缀的环境变量覆盖。主要运行时配置必须由 YAML 或环境变量显式提供，系统不得通过代码默认值补齐缺失的主要配置。配置结构体必须声明必填/可选和基础取值范围校验规则，配置加载必须优先通过结构体验证执行字段校验。PostgreSQL 配置必须使用共享连接参数和数据库名称字段，而不是按数据库重复声明完整 DSN。

#### Scenario: Load explicit config file
- **Given** 调用方提供配置文件路径
- **When** `common/config.Load` 被调用
- **Then** 系统读取该 YAML 文件
- **Then** 系统将 YAML 与 `AEGISCORE_` 环境变量覆盖反序列化为 `config.Config`
- **Then** 系统通过配置结构体上的校验规则验证配置结构

#### Scenario: Override config with environment variable
- **Given** 环境变量使用 `AEGISCORE_` 前缀
- **When** 环境变量 key 对应配置路径中的 `.` 或 `-`
- **Then** 系统将其映射为 `_` 并覆盖配置值

#### Scenario: Missing primary configuration fails validation
- **Given** YAML 和环境变量未显式提供 app、HTTP、log、Redis 或 PostgreSQL 的主要运行时配置字段
- **When** `common/config.Load` 被调用
- **Then** 配置加载必须返回错误，指出缺失或无效的配置字段
- **Then** 服务启动必须在依赖初始化前失败

#### Scenario: Validate required and optional fields from config structs
- **Given** 配置结构体字段声明了必填、可选和基础范围校验规则
- **When** `common/config.Load` 反序列化配置后执行校验
- **Then** 必填字段缺失或为空时配置加载必须失败
- **Then** 可选字段为空或省略时不得仅因为空而失败
- **Then** 端口、连接池大小和 timeout 等字段超出基础范围时配置加载必须失败

#### Scenario: Load PostgreSQL shared connection fields
- **Given** YAML 配置包含 `database.postgres.host`、`port`、`username`、`password`、`user_db_name`、`pay_db_name` 和 `common_db_name`
- **When** `common/config.Load` 被调用
- **Then** 系统必须加载这些 PostgreSQL 字段到配置对象
- **Then** PostgreSQL driver、连接池大小和 ping timeout 等运行时参数必须来自 YAML 或 `AEGISCORE_` 环境变量覆盖

#### Scenario: Missing PostgreSQL required field fails validation
- **Given** PostgreSQL 配置缺少 `host`、`port`、`username`、`user_db_name` 或 `common_db_name` 中任一必填字段
- **When** 系统验证配置结构
- **Then** 配置加载必须返回错误，指出对应 `database.postgres.<field>` 必填或无效

### Requirement: Provide shared runtime dependencies through Fx

系统必须通过 `common/infrastructure.Module` 提供配置、日志和 Redis client。PostgreSQL 共享基础能力必须支持按调用方指定的单个数据库配置创建具名连接池，并注册启动 ping 与停止 close 生命周期；具体服务必须在自己的 Fx module 中声明需要哪些具名 PostgreSQL 连接池。PostgreSQL 连接池必须基于共享连接参数和目标数据库名称构造连接，而不是读取每个数据库的独立 DSN。

#### Scenario: Provide common dependencies
- **Given** Fx app 包含 `common/infrastructure.Module`
- **When** Fx 解析依赖
- **Then** module 提供 `*config.Config`、`*slog.Logger` 和 `*redis.Client`
- **Then** module 不得固定提供所有已知业务数据库的具名 PostgreSQL 连接池

#### Scenario: Register one PostgreSQL lifecycle
- **Given** 调用方为一个目标数据库提供逻辑注入名和 PostgreSQL 数据库配置
- **When** PostgreSQL 单库 provider 被调用
- **Then** 系统创建一个具名 `*sql.DB` 连接池
- **Then** 该连接池必须连接到调用方指定的目标数据库
- **Then** Fx app 启动时 ping 该数据库
- **Then** Fx app 停止时关闭该数据库连接池

#### Scenario: User service declares required PostgreSQL pools
- **Given** 用户服务运行时需要 `user_db` 和 `common_db`
- **When** Fx app 包含用户服务模块
- **Then** 用户服务模块必须声明并提供具名 `user_db` 和 `common_db` PostgreSQL 连接池
- **Then** `user_db` 连接池必须连接到 `user_db_name` 指定的数据库
- **Then** `common_db` 连接池必须连接到 `common_db_name` 指定的数据库

#### Scenario: Service does not connect unused PostgreSQL pools
- **Given** 某个服务只声明需要 `common_db`
- **When** Fx app 启动该服务
- **Then** 系统必须只创建该服务声明的 PostgreSQL 连接池
- **Then** 系统不得因为其他配置字段存在而连接未声明的业务数据库

#### Scenario: Pay database name is configurable without adding payment runtime dependency
- **Given** 配置中存在 `pay_db_name`
- **When** `common/infrastructure.Module` 提供当前用户服务运行时依赖
- **Then** 系统必须允许配置对象读取 `pay_db_name`
- **Then** 系统不得仅因为存在 `pay_db_name` 而创建支付数据库连接池、支付业务 API、支付 repository 或支付 Ent client

#### Scenario: Register Redis lifecycle
- **Given** 配置中存在 Redis 地址和超时设置
- **When** Fx app 启动
- **Then** 系统创建 Redis client 并执行 ping
- **Then** Fx app 停止时关闭 Redis client

### Requirement: Provide service-specific Ent clients

系统必须为用户服务基于共享 PostgreSQL 连接池创建 Ent clients。

#### Scenario: Create named Ent clients
- **Given** Fx 容器中存在具名 `user_db` 和 `common_db` PostgreSQL 连接池
- **When** `user-services/internal/entclient.NewClients` 被调用
- **Then** 系统创建具名 `user_db` Ent client
- **Then** 系统创建具名 `common_db` Ent client
- **Then** Fx app 停止时关闭 Ent clients

#### Scenario: Repository receives user database client
- **Given** 用户 repository 需要访问用户数据
- **When** Fx 构造 `UserRepository`
- **Then** repository 接收具名 `user_db` Ent client，而不是直接打开数据库连接
