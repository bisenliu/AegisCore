## MODIFIED Requirements

### Requirement: 具名资源、datastore 与生命周期

系统 MUST 在 `common/runtime/resources` 中维护具名 Redis/PostgreSQL 资源类型、默认值和通用校验，并 MUST 由 `common/runtime/datastore` 使用共享资源类型初始化 framework-neutral 的单资源连接池。constructor 部分失败、启动探测失败或 instrumentation 失败时 MUST 清理已创建资源并保留主错误和清理错误。

#### Scenario: 声明和校验具名资源

- **WHEN** 服务声明一个或多个 Redis 或 PostgreSQL 实例
- **THEN** 服务 MUST 能使用 `RedisConfigs` 和 `PostgresConfigs` 按名称配置资源
- **AND** resources helper MUST 应用稳定 timeout、SSL 和连接池默认值，并对非法名称、地址、端口、timeout、SSL mode、username 或 pool 参数返回包含资源名和字段路径的错误
- **AND** PostgreSQL max idle connections MUST NOT 大于 max open connections，PostgreSQL password 与 Redis username/password MUST 允许为空
- **AND** 核心 runtime `Config` MUST NOT 固定这些资源或服务私有资源名

#### Scenario: 初始化 Redis client

- **WHEN** 调用方使用一份 `RedisConfig` 创建 Redis client
- **THEN** 统一 timeout MUST 映射到 dial、read、write 和启动 ping timeout，username MUST 可用于 Redis ACL
- **AND** 普通 Go constructor 和 Fx constructor MUST 一次只创建一个 client，且 MUST NOT 接收或遍历 `RedisConfigs`
- **AND** 具名配置 map 到单份配置的选择 MUST 由消费服务负责
- **AND** 启动 PING 或 tracing instrumentation 失败时 provider MUST 关闭 client，并保留探测、instrumentation 和关闭失败信息

#### Scenario: 构造和关闭单个 PostgreSQL 连接池

- **WHEN** 调用方使用一个资源名称和一份 `PostgresConfig` 构造 PostgreSQL 资源
- **THEN** `common/runtime/datastore` MUST 一次只创建一个连接池，并应用稳定的 DSN、SSL 和 pool 默认值
- **AND** constructor MUST NOT 接收 `PostgresConfigs`、`fx.Lifecycle` 或其他 DI framework 类型
- **AND** datastore MUST 提供接收资源名称和 `*sql.DB` 的启动 `Ping` 与 `Close` 契约
- **WHEN** 启动 `Ping` 失败
- **THEN** 探测 MUST 使用稳定的单资源 ping timeout，关闭同一连接池，并返回同时保留具名探测错误和关闭错误的错误
- **WHEN** 调用方关闭具名 PostgreSQL 连接池
- **THEN** datastore MUST 只关闭该单个连接池，关闭错误 MUST 包含资源名称

#### Scenario: Fx adapter 资源所有权

- **WHEN** Fx 服务声明一个具名 Redis 或 PostgreSQL 资源
- **THEN** `common/runtime/datastore` 中共置的 Fx adapter MUST 调用 framework-neutral 的单资源 constructor
- **AND** adapter MUST 只注册该资源的启动探测和关闭 hook，MUST NOT 遍历具名资源 map 或自动创建配置中出现的其他资源
- **AND** `OnStart` 创建的资源 MUST 由 `OnStop` 或 Fx rollback 关闭，constructor 阶段已创建的部分资源 MUST 在后续失败时立即清理
- **AND** feature 关闭自身 workerpool、watcher、cache 或 store 时 MUST NOT 关闭共享 Redis、Ent 或 PostgreSQL 资源

#### Scenario: 单一配置来源组装服务

- **WHEN** user-service 正式 `serve` 启动或装配测试构建 Fx App
- **THEN** CLI MUST 在创建 App 前只解析并校验一次 service config
- **AND** composition root MUST supply 同一个 service config 及由其派生的 runtime config，不得再次读取配置文件
- **AND** `common/runtime/config` MUST NOT 通过 Fx provider 接受配置路径或重复调用共享 loader
- **AND** 配置失败 MUST 在创建 App、资源或 lifecycle hook 前返回
