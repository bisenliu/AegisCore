## MODIFIED Requirements

### Requirement: Runtime primitive 基础

系统 MUST 在 `common/runtime/` 中维护配置加载、数据存储、logger、metrics、tracing、scheduler、workerpool、localcache、Redis key 和 timezone 等 runtime primitive。`common/runtime/config` MUST 将 `local_cache` 表达为通用具名缓存实例集合，并 MUST NOT 固定 user-service 的 `auth_token_version`、`rbac_user_roles` 或其他业务缓存名。`common/runtime/config` MUST 只声明和校验跨服务通用 runtime 配置，MUST NOT 声明或校验 user-service 的 `auth`、`ent`、JWT TTL、password KDF、refresh session 或 token version 策略。`common/runtime/config` MUST 使用 `github.com/go-viper/mapstructure/v2` 作为配置反序列化依赖，并 MUST NOT 保留旧版 `github.com/mitchellh/mapstructure` 导入、兼容层或旧行为 fallback。`common/runtime/workerpool` MUST 作为普通 Go runtime primitive 提供构造和显式关闭能力，MUST NOT 依赖 `go.uber.org/fx`、`fx.Lifecycle`、`fx.Hook` 或 `fxtest`。

#### Scenario: 服务启动加载配置

- **WHEN** 服务通过配置文件启动
- **THEN** 系统 MUST 使用共享配置 loader 解析 runtime、HTTP、Postgres、Redis、metrics、tracing、logger 和通用 `local_cache` 配置
- **AND** 服务私有配置 loader MUST 负责解析和校验该服务的 `auth`、`ent` 或其他业务配置块

#### Scenario: production-like JWT secret 长度校验

- **WHEN** user-service runtime environment 为 production-like 环境且配置包含 `auth.jwt.secret`
- **THEN** user-service 私有配置 validation MUST 要求该 secret 至少为 32 bytes
- **AND** development 环境 MAY 不执行该长度约束
- **AND** 校验错误 MUST 明确定位到 `auth.jwt.secret`
- **AND** `common/runtime/config` validation MUST NOT 校验 `auth.jwt.secret`

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

#### Scenario: workerpool 显式生命周期所有权

- **WHEN** 调用方通过 `common/runtime/workerpool.New` 创建后台任务池
- **THEN** `New` MUST 作为普通 Go 构造器创建未绑定 DI 框架的任务池
- **AND** `New` MUST NOT 接收 `fx.Lifecycle`、注册 `fx.Hook` 或导入 `go.uber.org/fx`
- **AND** 调用方 MUST 在拥有该资源的生命周期边界显式调用公开 `Stop(ctx)` 关闭任务池
- **AND** 系统 MUST NOT 保留旧签名、deprecated wrapper、可选 lifecycle 参数或 Fx 兼容 adapter

#### Scenario: workerpool Stop drain 语义

- **WHEN** 调用方对 `common/runtime/workerpool` 执行 `Stop(ctx)`
- **THEN** workerpool MUST 停止接收新任务并拒绝后续提交
- **AND** workerpool MUST 等待已经登记或已经接受的任务完成 drain
- **AND** `Stop(ctx)` 超时时 MUST 返回包装 `context.DeadlineExceeded` 的错误
- **AND** 重复 `Stop` MUST 共享同一 drain 状态，不得重复释放底层池或丢失已接收任务

#### Scenario: 本地缓存配置解析

- **WHEN** 配置文件包含 `local_cache.<name>` entry
- **THEN** `common/runtime/config` MUST 将其解析为以 `<name>` 为 key 的 `LocalCacheInstanceConfig`
- **AND** 配置 key MUST 保持原样供服务按名称读取

#### Scenario: 本地缓存配置通用校验

- **WHEN** `local_cache` 中存在一个或多个 entry
- **THEN** validation MUST 遍历所有 entry 并校验 `capacity > 0`、`ttl > 0`、`load_timeout > 0`、`num_counters >= 0` 和 `buffer_items >= 0`
- **AND** 校验错误 MUST 包含对应 `local_cache.<name>.<field>` 路径

#### Scenario: 拒绝 common 固化业务缓存名

- **WHEN** user-service 或其他服务需要声明必需本地缓存实例
- **THEN** 必需缓存名、缺失实例检查和业务含义 MUST 位于对应服务的 feature/provider 边界
- **AND** `common/runtime/config` MUST NOT 增加该业务缓存的固定字段或专用校验

#### Scenario: 服务资源名私有化

- **WHEN** 服务需要声明数据库、Redis 或 Ent resource 的业务资源名
- **THEN** 资源名常量 MUST 位于对应服务私有边界
- **AND** `common/runtime/resources` MUST NOT 声明 `NameUserDB` 或其他服务私有业务资源名
- **AND** 通用 datastore provider MUST 只消费调用方传入的资源名字符串

#### Scenario: auth Redis key schema

- **WHEN** 认证功能需要 refresh session、token version 或撤销相关 Redis key
- **THEN** 认证 infrastructure MUST 拥有功能 key schema，只能复用 `common/runtime/rediskey` 的通用构造规则

#### Scenario: workerpool 业务边界

- **WHEN** feature 代码使用 `common/runtime/workerpool` 提交后台任务
- **THEN** workerpool MUST 只提供并发控制、显式关闭、日志和统计能力，MUST NOT 承载 refresh session 上限裁剪、token version 撤销、可靠消息、eventbus、outbox 或业务一致性协议

#### Scenario: scheduler 分布式锁

- **WHEN** 定时任务具有多实例副作用
- **THEN** 任务 MUST 声明锁策略，锁 TTL MUST 为正值，长任务 SHOULD 具备续租策略

#### Scenario: mapstructure v2 配置反序列化

- **WHEN** `common/runtime/config` 将 Viper 读取到的配置反序列化为目标配置结构
- **THEN** 系统 MUST 使用 `github.com/go-viper/mapstructure/v2` 提供的 decode hook 和 decode 配置能力
- **AND** duration、slice、具名 Postgres、具名 Redis 和具名 `local_cache` 配置 MUST 按 v2 标准行为解析
- **AND** 系统 MUST NOT 导入 `github.com/mitchellh/mapstructure` 或保留面向旧版行为的兼容代码
