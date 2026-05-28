# shared-infrastructure

## Purpose

共享基础设施能力为服务提供一致的配置加载、结构化日志、Redis client、PostgreSQL 连接池和 Ent client 装配，减少各服务重复实现运行时基础能力。

## Requirements

### Requirement: Load configuration from YAML and environment

系统必须从 YAML 配置文件加载运行时配置，并支持 `AEGISCORE_` 前缀的环境变量覆盖。主要运行时配置必须由 YAML 或环境变量显式提供，系统不得通过代码默认值补齐缺失的主要配置。配置结构体必须声明必填/可选和基础取值范围校验规则，配置加载必须优先通过结构体验证执行字段校验。Redis 与 PostgreSQL 配置必须支持按名称声明多个实例；PostgreSQL 配置必须按实例声明连接参数和数据库名称字段，而不是通过共享字段加固定数据库名字段或完整 DSN 表达。

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
- **Given** YAML 和环境变量未显式提供 app、HTTP、log、Redis 命名实例或 PostgreSQL 命名实例的主要运行时配置字段
- **When** `common/config.Load` 被调用
- **Then** 配置加载必须返回错误，指出缺失或无效的配置字段
- **Then** 服务启动必须在依赖初始化前失败

#### Scenario: Validate required and optional fields from config structs
- **Given** 配置结构体字段声明了必填、可选和基础范围校验规则
- **When** `common/config.Load` 反序列化配置后执行校验
- **Then** 必填字段缺失或为空时配置加载必须失败
- **Then** 可选字段为空或省略时不得仅因为空而失败
- **Then** 端口、连接池大小和 timeout 等字段超出基础范围时配置加载必须失败

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

#### Scenario: Missing PostgreSQL required field fails validation
- **Given** 任一已声明 PostgreSQL 实例缺少 host、port、username、db_name 或 driver 中任一必填字段
- **When** 系统验证配置结构
- **Then** 配置加载必须返回错误，指出对应 `postgre.<name>.<field>` 必填或无效

### Requirement: Provide shared runtime dependencies through Fx

系统必须通过 `common/infrastructure.Module` 提供配置和日志。Redis 与 PostgreSQL 共享基础能力必须支持按调用方指定的单个命名实例创建具名 client 或连接池，并注册启动 ping 与停止 close 生命周期；具体服务必须在自己的 Fx module 中声明需要哪些具名 Redis client 和 PostgreSQL 连接池。Redis 与 PostgreSQL provider 必须只连接调用方声明的实例，不得因为配置中存在其他实例而自动连接全部实例。

#### Scenario: Provide common dependencies
- **Given** Fx app 包含 `common/infrastructure.Module`
- **When** Fx 解析依赖
- **Then** module 提供 `*config.Config` 和 `*slog.Logger`
- **Then** module 不得固定提供所有已知 Redis client 或业务数据库的具名 PostgreSQL 连接池

#### Scenario: Register one Redis lifecycle
- **Given** 调用方为一个目标 Redis 实例提供逻辑注入名和 Redis 实例名称
- **When** Redis 单实例 provider 被调用
- **Then** 系统创建一个具名 `*redis.Client`
- **Then** 该 client 必须连接到调用方指定的 Redis 实例配置
- **Then** Fx app 启动时 ping 该 Redis 实例
- **Then** Fx app 停止时关闭该 Redis client

#### Scenario: Register one PostgreSQL lifecycle
- **Given** 调用方为一个目标数据库提供逻辑注入名和 PostgreSQL 实例名称
- **When** PostgreSQL 单库 provider 被调用
- **Then** 系统创建一个具名 `*sql.DB` 连接池
- **Then** 该连接池必须连接到调用方指定的 PostgreSQL 实例配置
- **Then** Fx app 启动时 ping 该数据库
- **Then** Fx app 停止时关闭该数据库连接池

#### Scenario: User service declares required PostgreSQL pools
- **Given** 用户服务运行时需要 `user_db` 和 `common_db`
- **When** Fx app 包含用户服务模块
- **Then** 用户服务模块必须声明并提供具名 `user_db` 和 `common_db` PostgreSQL 连接池
- **Then** `user_db` 连接池必须连接到 `postgre.user_db` 指定的数据库
- **Then** `common_db` 连接池必须连接到 `postgre.common_db` 指定的数据库

#### Scenario: Service does not connect unused PostgreSQL pools
- **Given** 某个服务只声明需要 `common_db`
- **When** Fx app 启动该服务
- **Then** 系统必须只创建该服务声明的 PostgreSQL 连接池
- **Then** 系统不得因为其他配置字段存在而连接未声明的业务数据库

#### Scenario: Service does not connect unused Redis clients
- **Given** 某个服务只声明需要 `cache_redis`
- **When** Fx app 启动该服务
- **Then** 系统必须只创建该服务声明的 Redis client
- **Then** 系统不得因为配置中存在 `queue_redis` 而连接未声明的 Redis 实例

#### Scenario: Pay database is configurable without adding payment runtime dependency
- **Given** 配置中存在 `postgre.pay_db`
- **When** `common/infrastructure.Module` 和用户服务模块提供当前用户服务运行时依赖
- **Then** 系统必须允许配置对象读取 `postgre.pay_db`
- **Then** 系统不得仅因为存在 `postgre.pay_db` 而创建支付数据库连接池、支付业务 API、支付 repository 或支付 Ent client

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
