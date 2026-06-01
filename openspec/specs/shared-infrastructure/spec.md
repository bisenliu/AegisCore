# shared-infrastructure

## Purpose

共享基础设施能力为服务提供一致的配置加载、结构化日志、Redis client、PostgreSQL 连接池和 Ent client 装配，减少各服务重复实现运行时基础能力。

## Requirements

### Requirement: Load configuration from YAML and environment

系统必须从 YAML 配置文件加载运行时配置，并支持 `AEGISCORE_` 前缀的环境变量覆盖。配置加载必须将 YAML 与环境变量覆盖反序列化为 `config.Config`，但不得执行 required/optional、字段存在性或基础取值范围校验。Redis 与 PostgreSQL 配置必须支持按名称声明多个实例；PostgreSQL 配置必须使用 `postgres.<name>` 命名实例路径，并按实例声明连接参数和数据库名称字段，而不是通过共享字段加固定数据库名字段、旧的 `postgre.<name>` 路径或完整 DSN 表达。

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

#### Scenario: Override PostgreSQL config with environment variable
- **Given** YAML 配置包含 `postgres.user_db` 命名实例
- **Given** 环境变量提供 `AEGISCORE_POSTGRES_USER_DB_PASSWORD` 或 `AEGISCORE_POSTGRES_USER_DB_MAX_OPEN_CONNS`
- **When** `common/config.Load` 被调用
- **Then** 系统必须将环境变量覆盖应用到 `postgres.user_db` 对应字段

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
- **Given** YAML 配置包含 `postgres.user_db`、`postgres.pay_db` 和 `postgres.common_db` 命名实例
- **When** `common/config.Load` 被调用
- **Then** 系统必须加载每个 PostgreSQL 实例的 host、port、username、password、db_name、driver、sslmode、连接池和 ping timeout 字段到配置对象
- **Then** 每个 PostgreSQL 实例必须能独立覆盖连接池和 timeout 等运行时参数

#### Scenario: Core dependency startup failure remains fatal
- **Given** 配置加载成功
- **Given** Redis、`user_db`、`common_db` 或 HTTP server 在启动时不可用或配置无法被底层依赖接受
- **When** 服务启动初始化共享基础设施或 HTTP server
- **Then** 对应初始化必须返回错误并终止服务启动
- **Then** 错误必须保留失败的依赖名称、类型或底层错误上下文

### Requirement: Provide shared runtime dependencies through Fx

系统必须通过 `common/infrastructure.Module` 提供配置和 Zap 日志。Redis 与 PostgreSQL 共享基础能力必须支持按调用方指定的单个命名实例创建具名 client 或连接池，并注册启动 ping 与停止 close 生命周期；具体服务必须在自己的 Fx module 中声明需要哪些具名 Redis client 和 PostgreSQL 连接池。Redis 与 PostgreSQL provider 必须只连接调用方声明的实例，不得因为配置中存在其他实例而自动连接全部实例。用户服务必须声明并提供具名 `cache_redis` Redis client，供用户服务内部组件注入使用。

#### Scenario: Provide common dependencies
- **Given** Fx app 包含 `common/infrastructure.Module`
- **When** Fx 解析依赖
- **Then** module 提供 `*config.Config` 和 `*zap.Logger`
- **Then** module 不得固定提供所有已知 Redis client 或业务数据库的具名 PostgreSQL 连接池
- **Then** Fx app 停止时必须同步或关闭 logger 资源

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
- **Then** 该连接池必须连接到调用方指定的 `postgres.<name>` 实例配置
- **Then** Fx app 启动时 ping 该数据库
- **Then** Fx app 停止时关闭该数据库连接池

#### Scenario: User service declares required Redis client
- **Given** 用户服务运行时需要缓存 Redis
- **When** Fx app 包含用户服务模块
- **Then** 用户服务模块必须声明并提供具名 `cache_redis` Redis client
- **Then** `cache_redis` client 必须连接到 `redis.cache_redis` 指定的 Redis 实例
- **Then** Fx app 启动时必须 ping `cache_redis`
- **Then** Fx app 停止时必须关闭 `cache_redis`

#### Scenario: User service declares required PostgreSQL pools
- **Given** 用户服务运行时需要 `user_db` 和 `common_db`
- **When** Fx app 包含用户服务模块
- **Then** 用户服务模块必须声明并提供具名 `user_db` 和 `common_db` PostgreSQL 连接池
- **Then** `user_db` 连接池必须连接到 `postgres.user_db` 指定的数据库
- **Then** `common_db` 连接池必须连接到 `postgres.common_db` 指定的数据库

#### Scenario: Service does not connect unused PostgreSQL pools
- **Given** 某个服务只声明需要 `common_db`
- **When** Fx app 启动该服务
- **Then** 系统必须只创建该服务声明的 PostgreSQL 连接池
- **Then** 系统不得因为其他配置字段存在而连接未声明的业务数据库

#### Scenario: Service does not connect unused Redis clients
- **Given** 用户服务只声明需要 `cache_redis`
- **When** Fx app 启动用户服务
- **Then** 系统必须只创建 `cache_redis` Redis client
- **Then** 系统不得因为配置中存在 `queue_redis` 而连接未声明的 Redis 实例

#### Scenario: Pay database is configurable without adding payment runtime dependency
- **Given** 配置中存在 `postgres.pay_db`
- **When** `common/infrastructure.Module` 和用户服务模块提供当前用户服务运行时依赖
- **Then** 系统必须允许配置对象读取 `postgres.pay_db`
- **Then** 系统不得仅因为存在 `postgres.pay_db` 而创建支付数据库连接池、支付业务 API、支付 repository 或支付 Ent client

### Requirement: Provide reusable validation dependency

系统必须允许服务通过 Fx 获取共享请求校验器依赖，并由需要 HTTP 请求校验的服务显式引入该依赖。

#### Scenario: Provide validation module
- **Given** 服务 Fx app 引入共享 validation module
- **When** Fx 解析依赖
- **Then** 系统必须提供共享请求校验器实例
- **Then** controller 必须能够注入该实例并用于请求绑定和校验

#### Scenario: Do not validate runtime config in config loader
- **Given** `common/config.Load` 加载 YAML 和 `AEGISCORE_` 环境变量
- **When** 共享请求校验能力被引入服务
- **Then** `common/config.Load` 仍不得执行 required、optional、字段存在性或基础取值范围校验
- **Then** 请求 DTO 校验与运行时配置加载必须保持职责分离

#### Scenario: Do not connect extra runtime dependencies
- **Given** 服务引入共享 validation module
- **When** Fx app 启动
- **Then** validation module 不得创建 Redis client、PostgreSQL 连接池、Ent client 或 HTTP server
- **Then** validation module 初始化失败时必须返回错误并阻止服务以不完整状态启动

### Requirement: Provide Zap logging with trace-id and file rotation

系统必须提供基于 Zap 的共享日志组件。日志组件必须支持从 YAML 与 `AEGISCORE_` 环境变量加载日志级别、格式、目录、文件名前缀、控制台输出、保留天数、单文件大小和备份数量。所有通过项目 logger context API 输出的日志必须包含 `trace-id` 字段。日志必须按天写入带日期的分类日志文件，文件名格式为 `xxx.yyyy-mm-dd.all.log`、`xxx.yyyy-mm-dd.info.log`、`xxx.yyyy-mm-dd.warning.log`、`xxx.yyyy-mm-dd.error.log`。日期变化后，新日志必须写入新日期文件，旧日期文件必须保持原名作为历史日志。普通 Error 级别日志不得默认自动包含 stacktrace；需要堆栈的关键错误必须通过显式字段记录。

#### Scenario: Initialize Zap logger from config
- **Given** YAML 配置包含 log level、format、directory、filename、console、max_age_days、max_size_mb 和 max_backups
- **When** `common/logger.New` 被调用
- **Then** 系统必须创建 Zap logger
- **Then** logger 必须按配置输出 JSON 或 console 格式
- **Then** logger 必须在配置的目录下准备带日期的分类日志文件 writer
- **Then** logger 必须记录 caller 信息
- **Then** logger 不得默认对所有 Error 及以上日志自动添加 stacktrace

#### Scenario: Write logs to dated classified files
- **Given** 日志文件名前缀为 `aegiscore-user-services`
- **Given** 当前本地日期为 2026-06-02
- **When** 系统输出 Debug、Info、Warn 和 Error 日志
- **Then** 达到全局 level 的所有日志必须写入 `aegiscore-user-services.2026-06-02.all.log`
- **Then** Info 级别日志必须写入 `aegiscore-user-services.2026-06-02.info.log`
- **Then** Warn 级别日志必须写入 `aegiscore-user-services.2026-06-02.warning.log`
- **Then** Error 及以上日志必须写入 `aegiscore-user-services.2026-06-02.error.log`

#### Scenario: Rotate log files daily
- **Given** 服务持续运行跨过本地日期边界
- **When** 新日期的第一条日志写入
- **Then** logger 必须关闭旧日期文件 writer
- **Then** logger 必须创建并写入新日期对应的分类日志文件
- **Then** 旧日期日志文件必须保持原文件名，不得依赖重命名不带日期的活动文件
- **Then** 保留天数、单文件大小和备份数量限制必须按配置生效

#### Scenario: Changing system date does not require renaming active file
- **Given** 服务已在 2026-06-01 写入 `aegiscore-user-services.2026-06-01.info.log`
- **When** 系统日期变为 2026-06-02 且服务写入下一条 Info 日志
- **Then** 新日志必须写入 `aegiscore-user-services.2026-06-02.info.log`
- **Then** `aegiscore-user-services.2026-06-01.info.log` 必须保留历史内容
- **Then** 系统不得要求将 `aegiscore-user-services.info.log` 重命名为日期文件

#### Scenario: Include trace-id from context
- **Given** `context.Context` 中存在 trace-id
- **When** 业务代码调用 `common/logger.Info(ctx, ...)`、`Warn(ctx, ...)` 或 `Error(ctx, ...)`
- **Then** 输出日志必须包含字段 `trace-id` 且值等于 context 中的 trace-id

#### Scenario: Log without request context
- **Given** `context.Context` 中不存在 trace-id
- **When** 系统启动流程或基础设施代码输出日志
- **Then** 输出日志仍必须包含 `trace-id` 字段
- **Then** 字段值必须为空字符串或系统明确生成的 trace-id

#### Scenario: Log expected error without stacktrace
- **Given** 业务代码调用 `common/logger.Error(ctx, "query user profile failed", zap.Error(err))`
- **When** logger 写出该 Error 日志
- **Then** 日志必须包含错误字段、caller 字段和 `trace-id` 字段
- **Then** 日志不得因为 Error 级别自动包含 stacktrace 字段

#### Scenario: Log critical error with explicit stacktrace
- **Given** 关键错误调用点显式传入 `zap.Stack("stacktrace")` 或共享 logger 堆栈辅助函数生成的字段
- **When** logger 写出该 Error 日志
- **Then** 日志必须包含显式请求的 stacktrace 字段
- **Then** 该行为不得重新启用所有 Error 级别日志的自动 stacktrace

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

### Requirement: Provide database connection inputs for migration tooling

共享基础设施配置必须能够为外部迁移工具提供目标 PostgreSQL 数据库连接信息。迁移工具可以使用部署环境提供的 `DATABASE_URL`，也可以从现有 `AEGISCORE_` 环境变量或 YAML PostgreSQL 命名实例配置组装等价连接 URL，但不得要求启动 Fx app、Redis client、HTTP server 或 Ent runtime client。

#### Scenario: Migration uses deployment database URL
- **Given** 部署环境提供 `DATABASE_URL`
- **When** Atlas 迁移执行脚本运行
- **Then** Atlas 迁移执行必须能够使用该 URL 连接目标 PostgreSQL 数据库
- **Then** 迁移执行不得依赖启动 Gin HTTP server、Redis client 或 Fx runtime graph

#### Scenario: Migration targets named user database
- **Given** 部署环境选择从项目配置组装数据库连接信息
- **When** 迁移执行脚本为用户服务构造目标数据库连接
- **Then** 迁移执行必须使用用户服务拥有的 `postgres.user_db` 等价连接信息
- **Then** 迁移执行不得因为配置中存在 `postgres.pay_db` 或 `postgres.common_db` 而迁移非目标数据库
