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

### Requirement: Load system timezone configuration

系统 MUST 从 YAML 配置和 `AEGISCORE_` 环境变量覆盖中加载 `system.timezone` 到共享配置对象。配置加载器 MUST 只负责读取、覆盖和反序列化该字段，不得因为该字段缺失、为空或取值无法加载为 IANA 时区而在 `common/config.Load` 阶段返回校验错误。

#### Scenario: Load timezone from YAML
- **Given** YAML 配置包含 `system.timezone: Asia/Shanghai`
- **When** `common/config.Load` 被调用
- **Then** 系统 MUST 将该值反序列化到 `config.Config` 的系统配置中

#### Scenario: Override timezone with environment variable
- **Given** YAML 配置包含 `system.timezone: Asia/Shanghai`
- **Given** 环境变量提供 `AEGISCORE_SYSTEM_TIMEZONE=UTC`
- **When** `common/config.Load` 被调用
- **Then** 系统 MUST 将系统时区配置加载为 `UTC`

#### Scenario: Missing timezone is not rejected by config loader
- **Given** YAML 和环境变量未显式提供 `system.timezone`
- **When** `common/config.Load` 被调用
- **Then** 配置加载 MUST 成功反序列化配置对象
- **Then** 配置加载器 MUST NOT 因系统时区为空而返回校验错误

### Requirement: Load authentication configuration

系统 MUST 从 YAML 配置和 `AEGISCORE_` 环境变量覆盖中加载 `auth` 配置到共享配置对象。认证配置 MUST 支持 JWT secret、issuer、audience 和 token 过期配置。配置加载器 MUST 只负责读取、覆盖和反序列化这些字段，不得在 `common/config.Load` 阶段执行 required、字段存在性或基础取值范围校验。认证配置 MUST NOT 包含 `auth.whitelist` 字段，服务 MUST NOT 通过共享认证配置声明公开路径或认证豁免路径。

#### Scenario: Load auth config from YAML
- **Given** YAML 配置包含 `auth.jwt.secret`、`auth.jwt.issuer` 和 `auth.jwt.audience`
- **When** `common/config.Load` 反序列化配置
- **Then** 系统 MUST 将这些字段反序列化到 `config.Config` 的认证配置中
- **Then** 认证配置对象 MUST NOT 暴露白名单路径集合

#### Scenario: Override auth config with environment variable
- **Given** YAML 配置包含 auth 配置
- **When** 环境变量通过 `AEGISCORE_` 前缀覆盖 auth JWT 或 token 会话相关配置
- **Then** 系统 MUST 使用环境变量覆盖后的 auth 配置值
- **Then** 系统 MUST NOT 支持通过环境变量配置认证白名单路径

#### Scenario: Missing auth config is not rejected by config loader
- **Given** YAML 和环境变量未显式提供 auth 配置
- **When** `common/config.Load` 反序列化配置
- **Then** 配置加载 MUST 成功反序列化配置对象
- **Then** 配置加载器 MUST NOT 因 auth 字段缺失、为空或零值而返回校验错误

#### Scenario: Auth config does not create infrastructure clients
- **Given** 配置中存在 auth 配置
- **When** `common/config.Load` 反序列化配置
- **Then** 系统 MUST NOT 因 auth 配置存在而创建 Redis client、PostgreSQL 连接池、Ent client 或 HTTP server

#### Scenario: Whitelist config is not part of the contract
- **Given** 用户服务示例配置和共享配置结构
- **When** 调用方查看认证配置契约
- **Then** `auth.whitelist` MUST NOT 出现在示例 YAML 中
- **Then** `config.AuthConfig` MUST NOT 包含白名单字段

### Requirement: Load refresh token authentication configuration

系统 SHALL 从 YAML 配置和 `AEGISCORE_` 环境变量覆盖中加载 Refresh Token 和认证会话相关配置，包括 Refresh Token TTL、token version 缓存 TTL 和 Refresh Token 轮转开关。配置加载器 MUST 只负责读取、覆盖和反序列化这些字段，不得在 `common/config.Load` 阶段执行 required、字段存在性或基础取值范围校验。

#### Scenario: Load refresh token config from YAML
- **Given** YAML 配置包含 `auth.jwt.refresh_token_ttl`、`auth.token_version_cache_ttl` 和 `auth.refresh_token_rotation`
- **When** `common/config.Load` 被调用
- **Then** 系统 MUST 将这些字段反序列化到 `config.Config` 的认证配置中
- **Then** 配置加载器 MUST NOT 因 TTL 为零值或轮转开关未显式设置而返回校验错误

#### Scenario: Override refresh token config from environment
- **Given** YAML 配置包含 Refresh Token 相关认证配置
- **Given** 环境变量提供 `AEGISCORE_AUTH_JWT_REFRESH_TOKEN_TTL` 或 `AEGISCORE_AUTH_TOKEN_VERSION_CACHE_TTL`
- **When** `common/config.Load` 被调用
- **Then** 系统 MUST 使用环境变量覆盖后的认证配置值

### Requirement: Reuse user service datastore dependencies for authentication sessions

系统 SHALL 复用用户服务已声明的 `cache_redis` Redis client 和 `user_db` Ent client 支撑认证会话能力。认证会话能力 MUST NOT 因配置中存在其他 Redis 或 PostgreSQL 命名实例而自动连接未声明的实例。

#### Scenario: Authentication session store uses cache redis
- **Given** 用户服务运行时声明 `cache_redis` Redis client
- **When** 认证会话 store 被 Fx 构造
- **Then** 该 store MUST 使用具名 `cache_redis` Redis client
- **Then** 系统 MUST NOT 为认证会话额外连接未声明 Redis 实例

#### Scenario: Token version lookup uses user database
- **Given** 用户服务运行时声明具名 `user_db` Ent client
- **When** token version validator 需要回源 PostgreSQL
- **Then** 系统 MUST 使用 `user_db` Ent client 查询用户当前 `token_version`
- **Then** 系统 MUST NOT 连接未声明 PostgreSQL 实例

### Requirement: Provide shared timezone initialization module

系统 MUST 在 `common` 模块提供可复用的 timezone Fx module。该 module MUST 基于共享配置初始化进程本地时区，默认使用 `Asia/Shanghai`，成功时设置 `time.Local` 并同步 `TZ` 环境变量。无效时区 MUST 返回启动错误并保留底层加载错误上下文。初始化 MUST 在进程内只执行一次。

#### Scenario: Initialize configured timezone
- **Given** 配置中 `system.timezone` 为 `UTC`
- **When** 服务引入共享 timezone module 并启动 Fx app
- **Then** 系统 MUST 加载 `UTC` 时区
- **Then** 系统 MUST 将 `time.Local` 设置为该时区
- **Then** 系统 MUST 将 `TZ` 环境变量设置为 `UTC`

#### Scenario: Initialize default timezone
- **Given** 配置中未提供 `system.timezone`
- **When** 服务引入共享 timezone module 并启动 Fx app
- **Then** 系统 MUST 使用默认时区 `Asia/Shanghai`
- **Then** 系统 MUST 将 `time.Local` 设置为该时区
- **Then** 系统 MUST 将 `TZ` 环境变量设置为 `Asia/Shanghai`

#### Scenario: Invalid timezone fails startup
- **Given** 配置中 `system.timezone` 为无法被 `time.LoadLocation` 加载的值
- **When** 服务引入共享 timezone module 并启动 Fx app
- **Then** 系统 MUST 返回启动错误
- **Then** 错误 MUST 包含失败的时区值或底层加载错误上下文
- **Then** 服务 MUST NOT 以不确定时区继续启动

#### Scenario: Timezone initialization is process-global once
- **Given** 进程内已经成功执行过共享 timezone 初始化
- **When** 后续 Fx app 或依赖图再次触发 timezone 初始化
- **Then** 系统 MUST NOT 重复修改 `time.Local` 或 `TZ`
- **Then** 系统 MUST 保持第一次成功初始化的时区设置

### Requirement: Provide shared runtime dependencies through Fx

系统必须提供可由服务侧显式注入的配置和 Zap 日志 provider。Redis 与 PostgreSQL 共享基础能力必须支持按调用方指定的单个命名实例创建具名 client 或连接池，并注册启动 ping 与停止 close 生命周期；具体服务必须在自己的 Fx 装配中声明需要哪些公共配置、日志、具名 Redis client 和 PostgreSQL 连接池。Redis 与 PostgreSQL provider 必须只连接调用方声明的实例，不得因为配置中存在其他实例而自动连接全部实例。用户服务必须声明并提供具名 `cache_redis` Redis client，供用户服务内部组件注入使用。

#### Scenario: Provide common dependencies explicitly
- **Given** 服务 Fx app 提供 `ConfigPath`
- **When** 服务在自己的启动装配中显式提供共享配置和 Zap logger provider
- **Then** Fx app 必须解析出 `*config.Config` 和 `*zap.Logger`
- **Then** 公共配置和日志 provider 不得固定提供所有已知 Redis client 或业务数据库的具名 PostgreSQL 连接池
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
- **When** 用户服务显式提供公共配置、Zap logger 和当前用户服务运行时依赖
- **Then** 系统必须允许配置对象读取 `postgres.pay_db`
- **Then** 系统不得仅因为存在 `postgres.pay_db` 而创建支付数据库连接池、支付业务 API、支付 repository 或支付 Ent client

#### Scenario: Redis helper is organized with Redis infrastructure
- **Given** 维护者需要定位命名 Redis Fx provider helper
- **When** 查看 `common/infrastructure` 包中的 Redis 相关文件
- **Then** `ProvideNamedRedis` 必须与 Redis runtime factory 组织在 Redis 相关文件中
- **Then** PostgreSQL 相关文件不得包含 Redis provider helper 实现

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

系统必须提供基于 Zap 的共享日志组件。日志组件必须支持从 YAML 与 `AEGISCORE_` 环境变量加载日志级别、格式、目录、文件名前缀、控制台输出、保留天数、单文件大小和备份数量。所有通过项目 logger context API 输出的日志必须包含 `trace-id` 字段。所有通过项目 logger context API 输出的日志，其 `caller` 字段必须指向调用该 context API 的业务代码位置，而不是 `common/logger` 的封装函数位置。日志必须按天写入带日期的分类日志文件，文件名格式为 `xxx.yyyy-mm-dd.all.log`、`xxx.yyyy-mm-dd.info.log`、`xxx.yyyy-mm-dd.warning.log`、`xxx.yyyy-mm-dd.error.log`。日期变化后，新日志必须写入新日期文件，旧日期文件必须保持原名作为历史日志。普通 Error 级别日志不得默认自动包含 stacktrace；需要堆栈的关键错误必须通过显式字段记录。Fx 停止流程同步 logger 时，系统 MUST 忽略 stdout/stderr 等不可同步设备常见的 `syscall.EINVAL` 与 `syscall.ENOTTY`，但 MUST 继续返回其他 logger sync 错误。

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

#### Scenario: Context API records business caller
- **Given** 业务代码在 `user-services/internal/service/user_service.go` 中调用 `common/logger.Info(ctx, "create user", ...)`
- **When** logger 写出该 Info 日志
- **Then** 日志必须包含 `caller` 字段
- **Then** `caller` 字段必须指向 `user-services/internal/service/user_service.go` 的调用行
- **Then** `caller` 字段不得指向 `common/logger/context.go` 中的 `Info`、`Debug`、`Warn` 或 `Error` 封装函数

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

#### Scenario: Logger sync ignores terminal device sync errors
- **Given** Fx app 停止流程正在同步 Zap logger
- **Given** logger 底层 stdout 或 stderr writer 的 `Sync()` 返回 `syscall.EINVAL` 或 `syscall.ENOTTY`
- **When** logger stop hook 处理该错误
- **Then** 系统 MUST 忽略该错误并继续正常停止
- **Then** 服务 MUST NOT 因终端设备不支持同步而报告退出异常

#### Scenario: Logger sync preserves unexpected errors
- **Given** Fx app 停止流程正在同步 Zap logger
- **Given** logger `Sync()` 返回非 `syscall.EINVAL` 且非 `syscall.ENOTTY` 的错误
- **When** logger stop hook 处理该错误
- **Then** 系统 MUST 返回该错误
- **Then** 真实日志同步失败 MUST 保持可观察

### Requirement: Provide service-specific Ent clients

系统必须为用户服务基于共享 PostgreSQL 连接池创建 Ent clients。用户服务 Ent client provider 必须组织在 `user-services/internal/bootstrap` 包中，与服务侧 Redis/PostgreSQL runtime dependency wiring 保持同一启动装配边界。

#### Scenario: Create named Ent clients
- **Given** Fx 容器中存在具名 `user_db` 和 `common_db` PostgreSQL 连接池
- **When** `user-services/internal/bootstrap.NewNamedClients` 被调用
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

### Requirement: Configure datastore ping timeouts consistently

系统 MUST 允许 Redis 和 PostgreSQL 命名实例分别声明 ping timeout，并在 Fx lifecycle 启动检查中使用对应实例的配置值。

#### Scenario: Redis uses configured ping timeout
- **Given** `redis.cache_redis` 配置声明 ping timeout
- **When** Redis 单实例 provider 注册启动 lifecycle
- **Then** 系统必须使用 `redis.cache_redis` 的 ping timeout 创建 ping context
- **Then** 系统不得使用隐藏的固定 ping timeout 覆盖实例配置

#### Scenario: PostgreSQL keeps configured ping timeout
- **Given** `postgres.user_db` 配置声明 ping timeout
- **When** PostgreSQL 单实例 provider 注册启动 lifecycle
- **Then** 系统必须使用 `postgres.user_db` 的 ping timeout 创建 ping context

### Requirement: Provide explicit named datastore Fx helpers

系统 MUST 提供 opt-in 的命名 Redis 和 PostgreSQL Fx provider helper，以减少服务侧重复 wiring。helper 必须只为调用方声明的单个命名实例创建 client 或连接池，不得自动连接配置中存在但未声明的实例。

#### Scenario: Provide one named Redis helper
- **Given** 调用方声明需要逻辑名为 `cache_redis` 的 Redis client
- **When** 调用方使用命名 Redis helper 组装 Fx app
- **Then** 系统必须只创建 `redis.cache_redis` 对应的 `*redis.Client`
- **Then** 系统必须注册该 client 的启动 ping 和停止 close lifecycle

#### Scenario: Provide one named PostgreSQL helper
- **Given** 调用方声明需要逻辑名为 `user_db` 的 PostgreSQL 连接池
- **When** 调用方使用命名 PostgreSQL helper 组装 Fx app
- **Then** 系统必须只创建 `postgres.user_db` 对应的 `*sql.DB`
- **Then** 系统必须注册该连接池的启动 ping 和停止 close lifecycle

#### Scenario: Do not connect undeclared datastores through helpers
- **Given** 配置中存在 `redis.queue_redis` 和 `postgres.pay_db`
- **When** 服务只声明 `cache_redis` Redis helper 和 `user_db` PostgreSQL helper
- **Then** 系统不得创建 `queue_redis` Redis client
- **Then** 系统不得创建 `pay_db` PostgreSQL 连接池

### Requirement: Keep shared logger access concurrency-safe

系统 MUST 保证共享 logger 的默认实例读写在并发场景下是安全的，并继续支持通过 context 输出 `trace-id` 字段。

#### Scenario: Set and read default logger concurrently
- **Given** 测试或启动流程并发调用默认 logger 设置和 context logger 获取 API
- **When** 这些调用同时发生
- **Then** 系统不得产生数据竞争
- **Then** 获取到的 logger 必须可用于正常写日志

#### Scenario: Logger package remains modular
- **Given** 维护者需要修改 logger factory、context helper 或 file writer
- **When** 查看 `common/logger` 包
- **Then** 不同职责的实现必须组织在聚焦文件中
- **Then** 文件组织不得要求修改一个聚合大文件才能维护无关职责

### Requirement: Provide configurable shared HTTP middleware policies

系统 MUST 允许服务通过 options 配置共享 CORS 与 trace-id 中间件策略，并保留现有便捷 middleware 的可用性。

#### Scenario: Configure CORS policy
- **Given** 服务声明允许的 origins、methods、headers、exposed headers、credentials 和 max age
- **When** 服务使用可配置 CORS middleware
- **Then** 响应必须按声明策略写入 CORS headers
- **Then** 使用 origin 反射策略时响应必须设置 `Vary: Origin`

#### Scenario: Preserve trace id header contract
- **Given** HTTP 请求包含合法 `X-Trace-ID` header
- **When** trace-id middleware 处理请求
- **Then** 系统必须将该值写入 Gin context、Go context 和响应 `X-Trace-ID` header
- **Then** 日志字段必须继续使用 `trace-id`

#### Scenario: Reject unsafe inbound trace id values
- **Given** HTTP 请求包含超长或不符合策略的 `X-Trace-ID` header
- **When** trace-id middleware 使用配置的校验策略处理请求
- **Then** 系统必须生成替代 trace id 或按配置拒绝该值
- **Then** 系统不得把不安全的原始值写入日志字段或响应 header

### Requirement: Use one trace identifier across HTTP and logger contexts

系统 MUST 将 HTTP trace header、Gin context trace key、Go context trace value 和 Zap 日志 trace 字段视为同一个 trace 标识在不同边界的规范表达。HTTP header MUST 保持为 `X-Trace-ID`，Gin context key MUST 保持为 `trace_id`，日志字段 MUST 保持为 `trace-id`。系统 MUST NOT 因这些边界名称不同而生成多个互不一致的 trace 标识。

#### Scenario: Request trace id is shared across contexts
- **Given** HTTP 请求包含合法 `X-Trace-ID` header
- **When** trace-id 中间件处理请求并业务代码通过 `common/logger` context API 输出日志
- **Then** Gin context 中的 `trace_id` 值 MUST 等于请求 header 中的 trace id
- **Then** Go `context.Context` 中的 trace id 值 MUST 等于请求 header 中的 trace id
- **Then** 日志字段 `trace-id` 值 MUST 等于请求 header 中的 trace id

#### Scenario: Generated trace id is shared across contexts
- **Given** HTTP 请求未提供 `X-Trace-ID` header
- **When** trace-id 中间件生成新的 trace id
- **Then** 系统 MUST 将生成值写入响应 `X-Trace-ID` header
- **Then** 系统 MUST 将同一个生成值写入 Gin context、Go `context.Context` 和日志字段 `trace-id`

#### Scenario: Unsafe inbound trace id is not logged
- **Given** HTTP 请求包含超长或未通过配置校验的 `X-Trace-ID` header
- **When** trace-id 中间件处理请求
- **Then** 系统 MUST 生成替代 trace id
- **Then** 系统 MUST NOT 将不安全的原始 header 值写入 Gin context、Go `context.Context`、响应 header 或日志字段

### Requirement: Shared infrastructure naming cleanup preserves runtime behavior
共享基础设施相关命名标准化 SHALL 只修改低风险内部名称或文档表达，不得改变配置加载、Zap 日志、trace-id 边界名称、Redis provider、PostgreSQL provider 或 Ent runtime client 的行为。

#### Scenario: Shared infrastructure names are reviewed
- **WHEN** 实现审查 `common/config`、`common/infrastructure`、`common/logger`、`common/middleware`、`common/validation` 和服务侧基础设施 wiring 的命名
- **THEN** 实现 MUST 区分公共 Go API、内部参数名、文档表达和外部配置契约

#### Scenario: Runtime contracts are preserved
- **WHEN** 共享基础设施相关名称被标准化
- **THEN** YAML key、`AEGISCORE_` 环境变量覆盖、Redis/PostgreSQL 命名实例、`X-Trace-ID` header 和日志 `trace-id` 字段 MUST 保持不变

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

### Requirement: Provide shared Argon2id password helpers
系统 SHALL 在 `common` 模块提供统一的密码哈希和密码校验方法。密码哈希 MUST 使用 Argon2id、随机盐和可解析的编码格式保存算法版本与参数；密码校验 MUST 使用编码 hash 中的参数重新计算并执行常量时间比较。该能力 MUST 不创建 Redis client、PostgreSQL 连接池、Ent client、HTTP server 或 Fx runtime dependency。

#### Scenario: Hash password with Argon2id
- **Given** 调用方传入非空明文密码
- **When** 调用 `common` 密码哈希方法
- **Then** 系统 MUST 使用 Argon2id 生成密码 hash
- **Then** hash MUST 包含算法标识、版本、内存、迭代次数、并行度、salt 和派生值
- **Then** hash MUST NOT 等于明文密码

#### Scenario: Verify matching password
- **Given** 数据库中保存由 `common` 密码哈希方法生成的 Argon2id hash
- **When** 调用方使用相同明文密码调用 `common` 密码校验方法
- **Then** 系统 MUST 返回校验通过

#### Scenario: Reject non-matching password
- **Given** 数据库中保存由 `common` 密码哈希方法生成的 Argon2id hash
- **When** 调用方使用不同明文密码调用 `common` 密码校验方法
- **Then** 系统 MUST 返回校验不通过
- **Then** 系统 MUST NOT 在错误中公开明文密码、完整 hash、salt 或 hash 参数

#### Scenario: Reject malformed password hash
- **Given** 数据库中的密码 hash 不是 `common` 支持的 Argon2id 编码格式
- **When** 调用方调用 `common` 密码校验方法
- **Then** 系统 MUST 返回校验错误或校验不通过
- **Then** 系统 MUST NOT panic

#### Scenario: Password helper has no runtime datastore side effects
- **Given** 服务或测试引入 `common` 密码 helper
- **When** 调用密码哈希或校验方法
- **Then** 系统 MUST NOT 创建 Redis client、PostgreSQL 连接池、Ent client、HTTP server 或 Fx app

### Requirement: Structure shared infrastructure organization review feedback
系统 SHALL 在整理共享基础设施目录组织类代码评审意见时，使用中文分别给出问题说明、原因分析和建议改法，并围绕 `common/infrastructure/` 的可维护性、职责边界和后续扩展成本给出可执行建议。

#### Scenario: Explain flat infrastructure directory risk
- **WHEN** 评审意见涉及 `common/infrastructure/` 未按基础设施类型进一步拆分
- **THEN** 反馈 MUST 在问题说明中指出 Redis、PostgreSQL、MongoDB、RabbitMQ 等组件持续增加后，单目录容易出现文件过多和职责混杂
- **THEN** 反馈 MUST 在原因分析中说明平铺目录会降低定位效率、增加跨组件修改风险，并弱化共享基础设施 provider 的职责边界
- **THEN** 反馈 MUST 在建议改法中给出按基础设施类型或职责分层的组织建议

#### Scenario: Recommend maintainable infrastructure layering
- **WHEN** 反馈给出 `common/infrastructure/` 的拆分建议
- **THEN** 反馈 MUST 至少包含按基础设施类型拆分的示例，例如 `redis/`、`postgres/`、`mongo/`、`rabbitmq/`
- **THEN** 反馈 MAY 补充按职责拆分的示例，例如 `datastore/`、`messaging/`、`logging/`、`config/`
- **THEN** 反馈 MUST 明确后续重构需要保持 YAML key、`AEGISCORE_` 环境变量、Redis/PostgreSQL 命名实例和 Fx named injection 行为不变
