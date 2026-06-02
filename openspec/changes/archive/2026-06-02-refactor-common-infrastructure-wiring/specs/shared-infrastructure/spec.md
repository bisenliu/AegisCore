## MODIFIED Requirements

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
