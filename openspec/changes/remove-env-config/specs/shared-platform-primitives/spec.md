## MODIFIED Requirements

### Requirement: Runtime 配置、具名资源与生命周期

系统 MUST 在 `common/runtime/config` 中维护跨服务 runtime 配置、默认值、通用校验和显式单文件配置加载，在 `common/runtime/resources` 中维护具名 Redis/PostgreSQL 资源类型、默认值和通用校验，并由 `common/runtime/datastore` 使用共享资源类型初始化 framework-neutral 的单资源连接池。服务私有业务配置、必需资源名、业务用途和配置 map 到真实资源的选择 MUST 由消费服务拥有；constructor 部分失败、启动探测失败或 instrumentation 失败时 MUST 清理已创建资源并保留主错误和清理错误。运行时配置 MUST 只来自调用方显式指定的一份完整配置文件，MUST NOT 从进程环境变量读取、覆盖或补全配置。

#### Scenario: 严格加载并校验通用配置

- **WHEN** 服务通过显式配置文件加载 runtime 配置
- **THEN** 共享 loader MUST 只读取调用方给定的一份 YAML，使用 `github.com/go-viper/mapstructure/v2` 的 decode 能力解析 duration、slice、具名配置，以及 runtime、HTTP、gRPC、metrics、tracing、pprof、logger 和通用 `local_cache` 配置
- **AND** 文件缺失、不可读、格式非法或包含未声明字段 MUST 在启动前失败，并报告文件与完整字段路径
- **AND** loader MUST NOT 调用 `AutomaticEnv`、`BindEnv`、`os.Getenv`、`os.LookupEnv` 或等价环境变量读取能力，不得使用旧字段别名、环境变量 fallback 或隐式默认路径
- **AND** 共享 runtime config MUST 声明并校验 `runtime.timezone`、`runtime.gin.mode`、server、logger、metrics、tracing、pprof、lifecycle 和通用 local cache 配置
- **AND** `runtime.gin.mode` 默认值 MUST 为 `release`，合法值 MUST 仅为 `debug`、`release` 或 `test`
- **AND** `runtime.timezone` 缺省时 MUST 使用稳定默认值，非法 timezone MUST 在启动前失败
- **AND** `observability.pprof.enabled` 和 `observability.pprof.addr` 默认值 MUST 分别为 `false` 和 `127.0.0.1:6060`，production-like 环境启用 pprof 时 addr MUST 使用 loopback host
- **AND** 至少一个 HTTP 或 gRPC server MUST 启用
- **WHEN** 配置包含 `local_cache.<name>`
- **THEN** loader MUST 保留 `<name>` 并解析为通用缓存实例配置，validation MUST 校验 `capacity > 0`、`ttl > 0` 和 `load_timeout > 0`，错误 MUST 包含完整字段路径
- **AND** 配置契约 MUST NOT 暴露 Ristretto 的 `num_counters`、`buffer_items`、admission 或 write buffer 选项，必需缓存名及其业务含义 MUST 留在消费服务
- **WHEN** `runtime.lifecycle.stop_timeout` 小于 HTTP shutdown timeout、worker drain allowance、tracing flush allowance 和 shutdown safety margin 的组合最低预算
- **THEN** 配置校验 MUST 失败并指出该字段及最低所需预算；大于或等于组合最低预算时共享校验 MUST 继续通过，业务停止策略 MUST 由 owning feature 或服务组合层表达

#### Scenario: 服务私有配置与单一配置来源

- **WHEN** 服务需要 `auth`、`ent`、JWT TTL、refresh session、token version、RBAC、bootstrap 凭据或 production-like secret 校验
- **THEN** 服务私有 loader MUST 负责解析和校验，`common/runtime/config` MUST NOT 声明或校验这些业务配置，服务私有配置 MUST NOT 声明、读取或兼容旧 `auth.password_kdf` 配置
- **AND** 敏感配置 MUST 来自显式配置文件；仓库 MAY 提供无真实 secret 的示例文件，但 MUST NOT 提交实际凭据
- **WHEN** user-service 正式 `serve`、执行 RBAC CLI 或装配测试构建 Fx App
- **THEN** CLI MUST 在创建 App 或 use case 前只解析并校验一次完整 service config，composition root MUST supply 同一个 service config 及由其派生的 runtime config，不得再次读取配置文件
- **AND** `common/runtime/config` MUST NOT 通过 Fx provider 接受配置路径或重复调用共享 loader，配置失败 MUST 在创建 App、资源或 lifecycle hook 前返回

#### Scenario: 声明、校验并选择具名资源

- **WHEN** 服务声明一个或多个 Redis 或 PostgreSQL 实例
- **THEN** 服务 MUST 能使用 `RedisConfigs` 和 `PostgresConfigs` 按名称配置资源
- **AND** resources helper MUST 应用稳定 timeout、SSL 和连接池默认值，并对非法名称、地址、端口、timeout、SSL mode、username 或 pool 参数返回包含资源名和字段路径的错误
- **AND** PostgreSQL max idle connections MUST NOT 大于 max open connections，连接生命周期参数 MUST 为正值
- **WHEN** user-service 需要选择 `cache_redis` 或 `primary_db`
- **THEN** 必需资源名、业务用途和缺失错误 MUST 留在 user-service provider 或 feature infrastructure，MUST NOT 进入 common

#### Scenario: 进程时区只来自配置文件

- **WHEN** runtime 初始化进程时区
- **THEN** timezone primitive MUST 使用已校验的 `runtime.timezone`，MUST NOT 读取 `TZ` 或其他环境变量
- **AND** 如果通过 Fx 初始化，服务 composition root MUST 显式绑定初始化调用或服务级 runtime 初始化函数，common MUST NOT 仅为了包装 `Init` 暴露无额外运行时职责的 Fx provider

#### Scenario: 资源构造和清理保留完整错误

- **WHEN** PostgreSQL 或 Redis constructor 在创建底层资源后发生 ping、instrumentation 或其他初始化失败
- **THEN** constructor MUST 关闭已创建资源，并返回同时保留主错误与清理错误的结果
- **WHEN** Fx lifecycle 启动探测失败
- **THEN** lifecycle MUST 返回启动错误，已注册关闭路径 MUST 能清理成功创建的资源
