## MODIFIED Requirements

### Requirement: Runtime 配置、资源与 datastore

系统 MUST 在 `common/runtime/config` 和 `common/runtime/resources` 中分别维护跨服务 runtime 配置以及具名 Redis/PostgreSQL 资源类型、默认值和通用校验，并 MUST 由 `common/runtime/datastore` 使用共享资源类型初始化框架无关的单资源连接池。服务私有业务配置、必需资源名、业务用途和配置 map 到真实资源的选择 MUST 由消费服务拥有。

#### Scenario: 严格加载通用配置

- **WHEN** 服务通过配置文件启动
- **THEN** 共享 loader MUST 解析 runtime、HTTP、gRPC、metrics、tracing、pprof、logger 和通用 `local_cache` 配置
- **AND** 系统 MUST 使用 `github.com/go-viper/mapstructure/v2` 的 decode 能力解析 duration、slice 和具名配置
- **AND** 未声明字段 MUST 在启动前失败并报告完整路径，不得使用旧字段别名或 fallback

#### Scenario: 进程级 Gin mode 配置

- **WHEN** 服务加载 runtime 配置
- **THEN** 共享 runtime config MUST 声明 `runtime.gin.mode`
- **AND** 默认值 MUST 为 `release`
- **AND** 环境变量覆盖 MUST 使用 `AEGISCORE_RUNTIME_GIN_MODE`
- **AND** 校验 MUST 只接受 `debug`、`release` 或 `test`

#### Scenario: pprof 诊断配置

- **WHEN** 服务加载 observability 配置
- **THEN** 共享 runtime config MUST 声明 `observability.pprof.enabled` 和 `observability.pprof.addr`
- **AND** 默认值 MUST 分别为 `false` 和 `127.0.0.1:6060`
- **AND** 环境变量覆盖 MUST 使用 `AEGISCORE_OBSERVABILITY_PPROF_ENABLED` 和 `AEGISCORE_OBSERVABILITY_PPROF_ADDR`
- **AND** `observability.pprof.addr` MUST 在配置加载阶段校验为合法 `host:port`

#### Scenario: pprof 生产类环境安全校验

- **WHEN** `app.environment` 为 `prod`、`production` 或 `staging` 且 `observability.pprof.enabled=true`
- **THEN** `observability.pprof.addr` MUST 使用 loopback host
- **AND** 非 loopback host MUST 在配置加载校验阶段失败

#### Scenario: 服务私有配置留在服务边界

- **WHEN** 服务需要 `auth`、`ent`、JWT TTL、password KDF、refresh session、token version 或 production-like secret 校验
- **THEN** 服务私有 loader MUST 负责解析和校验这些配置
- **AND** `common/runtime/config` MUST NOT 声明或校验这些业务配置

#### Scenario: 协议 server 配置

- **WHEN** 服务声明 `server.http` 或 `server.grpc`
- **THEN** HTTP 配置 MUST 支持 enabled、host、port、read、write、idle 和 shutdown timeout
- **AND** gRPC 配置 MUST 支持 enabled、host、port 和 shutdown timeout
- **AND** 至少一个 server MUST 启用

#### Scenario: 通用具名本地缓存配置

- **WHEN** 配置包含 `local_cache.<name>`
- **THEN** loader MUST 保留 `<name>` 并解析为通用缓存实例配置
- **AND** validation MUST 校验 `capacity > 0`、`ttl > 0`、`load_timeout > 0`、`num_counters >= 0` 和 `buffer_items >= 0`，错误 MUST 包含完整字段路径
- **AND** 必需缓存名及其业务含义 MUST 留在消费服务

#### Scenario: 声明和校验具名资源

- **WHEN** 服务声明一个或多个 Redis 或 PostgreSQL 实例
- **THEN** 服务 MUST 能使用 `RedisConfigs` 和 `PostgresConfigs` 按名称配置资源
- **AND** resources helper MUST 应用稳定 timeout、SSL 和连接池默认值，并对非法名称、地址、端口、timeout、SSL mode、username 或 pool 参数返回包含资源名和字段路径的错误
- **AND** PostgreSQL max idle connections MUST NOT 大于 max open connections，PostgreSQL password 与 Redis username/password MUST 允许为空
- **AND** 核心 runtime `Config` MUST NOT 固定这些资源或服务私有资源名

#### Scenario: 初始化 Redis client

- **WHEN** 调用方使用一份 `RedisConfig` 创建 Redis client
- **THEN** 统一 timeout MUST 映射到 dial、read、write 和启动 ping timeout，username MUST 可用于 Redis ACL
- **AND** 调用方 MUST 能显式指定 tracing provider，未指定时 MAY 使用全局 provider
- **AND** 普通 Go constructor 和 Fx constructor MUST 一次只创建一个 client，且 MUST NOT 接收或遍历 `RedisConfigs`
- **AND** 具名配置 map 到单份配置的选择 MUST 由消费服务负责
- **AND** 启动 PING 失败时 provider MUST 关闭 client，并保留探测失败和关闭失败信息

#### Scenario: 构造单个 PostgreSQL 连接池

- **WHEN** 调用方使用一个资源名称和一份 `PostgresConfig` 构造 PostgreSQL 资源
- **THEN** `common/runtime/datastore` MUST 一次只创建一个连接池，并应用稳定的 DSN、SSL 和 pool 默认值
- **AND** constructor MUST NOT 接收 `PostgresConfigs`、`fx.Lifecycle` 或其他 DI framework 类型
- **AND** constructor MUST NOT 遍历具名配置 map、返回 map result 或隐式创建其他资源
- **AND** datastore MUST 提供接收资源名称和 `*sql.DB` 的启动 `Ping` 与 `Close` 契约

#### Scenario: PostgreSQL 启动探测失败回滚

- **WHEN** 调用方对单个具名 PostgreSQL 连接池执行启动 `Ping`
- **THEN** 探测 MUST 使用稳定的单资源 ping timeout
- **AND** 探测失败 MUST 关闭同一连接池
- **AND** 返回错误 MUST 同时保留具名探测错误和关闭错误

#### Scenario: 关闭单个 PostgreSQL 连接池

- **WHEN** 调用方关闭具名 PostgreSQL 连接池
- **THEN** datastore MUST 关闭该单个连接池
- **AND** 关闭错误 MUST 包含资源名称
- **AND** 关闭操作 MUST NOT 关闭其他资源

#### Scenario: 使用 Fx adapter 组合 PostgreSQL

- **WHEN** Fx 服务声明一个具名 PostgreSQL 资源
- **THEN** `common/runtime/datastore` 中共置的 Fx adapter MUST 调用框架无关的单资源 constructor
- **AND** adapter MUST 只注册该资源的启动 `Ping` 和 `Close` hook
- **AND** adapter MUST NOT 遍历 `PostgresConfigs` 或自动创建配置中出现的其他资源
- **AND** 成功连接和关闭日志 MUST 保留 PostgreSQL component 与资源名称

#### Scenario: 单一配置来源组装服务

- **WHEN** user-service 正式 `serve` 启动或装配测试构建 Fx App
- **THEN** CLI MUST 在创建 App 前只解析并校验一次 service config
- **AND** composition root MUST supply 同一个 service config 及由其派生的 runtime config，不得再次读取配置文件
- **AND** bootstrap MUST 提供无配置文件 I/O 的基础 Fx options 入口供正式 App 和测试复用
- **AND** 配置失败 MUST 在创建 App、资源或 lifecycle hook 前返回
