## MODIFIED Requirements

### Requirement: Runtime primitive 基础

系统 MUST 在 `common/runtime/` 中维护跨服务稳定的配置加载、资源类型、datastore、logger、observability、scheduler、workerpool、localcache、Redis key 和 timezone primitive。`common/runtime/config.Config` MUST 只包含 `app`、`server`、`log` 和 `observability`，MUST NOT 包含服务资源、业务缓存、pprof、trusted proxies 或文件日志配置。`common/runtime/config` MUST 使用 `github.com/go-viper/mapstructure/v2` 作为配置反序列化依赖，并 MUST NOT 保留旧版 `github.com/mitchellh/mapstructure` 导入、兼容层或旧行为 fallback。

#### Scenario: 加载核心和服务扩展配置

- **WHEN** 服务通过 `LoadInto` 加载配置
- **THEN** 核心字段 MUST 从 `app`、`server`、`log` 和 `observability` 解析
- **AND** 服务 MUST 能通过自有根 Config 声明 resources 和业务配置

#### Scenario: 拒绝未知配置字段

- **WHEN** YAML 包含目标 Config 未声明的字段
- **THEN** 加载 MUST 在服务启动前失败
- **AND** 错误 MUST 包含未知字段的完整配置路径
- **AND** 系统 MUST NOT 提供旧字段别名、自动迁移、白名单绕过或 fallback

#### Scenario: 协议 server 最小配置

- **WHEN** 服务声明 `server.http` 或 `server.grpc`
- **THEN** HTTP MUST 支持 enabled、host、port、read、write、idle 和 shutdown timeout
- **AND** gRPC MUST 支持 enabled、host、port 和 shutdown timeout
- **AND** 至少一个 server MUST 启用

#### Scenario: 初始化进程时区

- **WHEN** runtime 初始化进程时区
- **THEN** timezone primitive MUST 优先使用平台 `TZ` 环境变量，并在缺省时使用稳定默认值
- **AND** timezone primitive MUST NOT 依赖核心 Config 或重新引入 `system.timezone`

#### Scenario: runtime 依赖初始化

- **WHEN** 服务需要连接 PostgreSQL、Redis、logger、metrics 或 tracing provider
- **THEN** 服务 MUST 优先复用 `common/runtime/` 中的 provider 和 Fx module

#### Scenario: PostgreSQL 启动探测失败关闭连接池

- **WHEN** 共享 PostgreSQL Fx provider 已创建连接池但启动 PING 失败
- **THEN** provider MUST 关闭已创建的连接池
- **AND** 返回错误 MUST 保留 PING 失败和关闭失败信息
- **AND** 启动失败后连接池 MUST 不再泄露为可继续使用的资源

#### Scenario: Redis 启动探测失败关闭 client

- **WHEN** 共享 Redis Fx provider 已创建 client 但启动 PING 失败
- **THEN** provider MUST 关闭已创建的 Redis client
- **AND** 返回错误 MUST 保留 PING 失败和关闭失败信息
- **AND** 启动失败后 Redis client MUST 不再泄露为可继续使用的资源

#### Scenario: Redis client 显式 tracing provider

- **WHEN** 服务通过 `OpenRedisClient` 或 `NewRedisClient` 创建共享 Redis client
- **THEN** 调用方 MUST 能通过 `WithRedisTracerProvider` 显式指定 tracing provider
- **AND** 未显式指定时 MAY 使用全局 tracing provider
- **AND** no-op tracing provider MUST NOT 改变 Redis 命令结果、连接生命周期或启动 PING 语义

#### Scenario: 后台任务执行

- **WHEN** 服务需要执行定时任务、分布式锁或固定 worker pool 任务
- **THEN** 系统 MUST 使用共享 scheduler、lock、workerpool 和 metrics 约束，并记录失败、拒绝、panic 和完成事件

#### Scenario: workerpool Stop drain 语义

- **WHEN** 调用方对 `common/runtime/workerpool` 执行 `Stop(ctx)`
- **THEN** workerpool MUST 停止接收新任务并拒绝后续提交
- **AND** workerpool MUST 等待已经登记或已经接受的任务完成 drain
- **AND** `Stop(ctx)` 超时时 MUST 返回包装 `context.DeadlineExceeded` 的错误
- **AND** 重复 `Stop` MUST 共享同一 drain 状态，不得重复释放底层池或丢失已接收任务

#### Scenario: auth Redis key schema

- **WHEN** 认证功能需要 refresh session、token version 或撤销相关 Redis key
- **THEN** 认证 infrastructure MUST 拥有功能 key schema，只能复用 `common/runtime/rediskey` 的通用构造规则

#### Scenario: workerpool 业务边界

- **WHEN** feature 代码使用 `common/runtime/workerpool` 提交后台任务
- **THEN** workerpool MUST 只提供并发控制、生命周期、日志和统计能力，MUST NOT 承载 refresh session 上限裁剪、token version 撤销、可靠消息、eventbus、outbox 或业务一致性协议

#### Scenario: scheduler 分布式锁

- **WHEN** 定时任务具有多实例副作用
- **THEN** 任务 MUST 声明锁策略，锁 TTL MUST 为正值，长任务 SHOULD 具备续租策略

#### Scenario: mapstructure v2 配置反序列化

- **WHEN** `common/runtime/config` 将 Viper 读取到的配置反序列化为服务 Config
- **THEN** 系统 MUST 使用 `github.com/go-viper/mapstructure/v2` 提供的 decode hook 和 decode 配置能力
- **AND** duration、slice、核心配置和服务自有具名 resources MUST 按 v2 标准行为解析
- **AND** 系统 MUST NOT 导入 `github.com/mitchellh/mapstructure` 或保留面向旧版行为的兼容代码

### Requirement: 共享只读集合不得暴露共享可写状态

`common` 中用于配置校验、HTTP middleware 默认策略和 validation tag 解析的只读集合或默认 struct MUST 使用不暴露共享可写底层状态的表达方式。实现 MUST 保持配置允许值、validation 字段名解析顺序、CORS 默认策略、公开错误消息和 HTTP 响应行为不变。

#### Scenario: 配置校验集合不可被包内误写

- **WHEN** `common/runtime/config` 或 `common/runtime/resources` 校验 log level、log format、server enabled、PostgreSQL SSL mode、tracing sample ratio 或 production-like insecure transport
- **THEN** 校验逻辑 MUST 使用 `switch`、私有查询函数、局部构造或等价方式表达固定集合
- **AND** 系统 MUST NOT 暴露可被同包未来代码直接写入的 package-level map 作为这些固定集合的权威来源
- **AND** 合法值、非法值和错误消息 MUST 保持当前语义不变

#### Scenario: 默认 CORS 配置隔离共享 slice

- **WHEN** 调用方使用 `common/http/middleware.CORS()` 或 `CORSWithOptions` 创建 CORS middleware
- **THEN** middleware MUST 在构造时持有与 package-level 默认值和调用方传入 slice 隔离的配置副本
- **AND** 调用方后续修改其传入的 origins、methods、headers 或 exposed headers slice MUST NOT 改变已创建 middleware 的行为
- **AND** `CORS()` 的默认响应 MUST 继续使用 `Access-Control-Allow-Origin=*`、`Access-Control-Allow-Methods=GET,POST,PUT,PATCH,DELETE,OPTIONS` 和 `Access-Control-Allow-Headers=Authorization,Content-Type`

#### Scenario: validation request tag 顺序稳定且不可共享写入

- **WHEN** `common/validation` 从 struct field tag 推导请求字段名
- **THEN** tag 优先级和支持集合 MUST 保持当前顺序与语义
- **AND** 实现 MUST NOT 依赖可被同包未来代码修改的 package-level slice 作为共享底层状态

#### Scenario: 保留非只读集合变量需有理由

- **WHEN** 实现阶段发现 package-level var 不迁移
- **THEN** 该变量 MUST 不属于本次只读 map、slice 或默认 struct 风险范围，或具备明确保留理由
- **AND** 合理保留理由 MAY 包括 sentinel error、regexp 编译结果、Fx Module、`sync.Pool`、atomic counter 或需要运行时状态的对象

## ADDED Requirements

### Requirement: 共享资源配置边界

系统 MUST 在 `common/runtime/resources` 提供 Redis/PostgreSQL 具名资源类型、默认值和通用校验，但 MUST NOT 将这些资源挂入核心 `config.Config`。

#### Scenario: 声明多具名资源

- **WHEN** 服务依赖多个 Redis 或 PostgreSQL 实例
- **THEN** 服务 MUST 能使用 `RedisConfigs` 和 `PostgresConfigs` 按名称声明资源
- **AND** 必需资源名称和业务用途 MUST 由消费服务校验

#### Scenario: 应用资源默认值

- **WHEN** Redis timeout 或 PostgreSQL sslmode、pool 参数未显式配置
- **THEN** resources helper MUST 应用稳定默认值
- **AND** PostgreSQL ping timeout MUST 作为内部 helper 默认值存在而不进入 YAML 契约

#### Scenario: 校验资源参数

- **WHEN** 资源名称、地址、端口、timeout、sslmode、PostgreSQL username 或 pool 参数非法
- **THEN** 通用校验 MUST 返回包含资源名和字段路径的错误
- **AND** PostgreSQL max idle connections MUST NOT 大于 max open connections
- **AND** PostgreSQL password 与 Redis username/password MUST 允许为空

### Requirement: Datastore 使用共享资源类型

系统 MUST 使用 `common/runtime/resources` 的 Redis/PostgreSQL 配置初始化共享 datastore，并 MUST NOT 依赖已删除的核心 Config 资源字段或 helper。

#### Scenario: 初始化 Redis client

- **WHEN** datastore 创建 Redis client
- **THEN** 统一 timeout MUST 映射到 dial、read、write 和启动 ping timeout
- **AND** username MUST 可用于 Redis ACL

#### Scenario: 初始化 PostgreSQL pool

- **WHEN** datastore 创建 PostgreSQL pool
- **THEN** DSN MUST 在 datastore 或 resources 边界构建
- **AND** pool 与 ping timeout MUST 使用 resources 默认值和可选覆盖
