## MODIFIED Requirements

### Requirement: 共享认证授权 helper API 治理

系统 MUST 在 `common/http` 和 `common/security` 中保持认证、授权 helper 的导出 API 语义清晰且避免重复简写入口；当共享 helper 只包装另一个推荐入口、暴露未参与行为的参数或没有额外稳定语义时，系统 MUST 通过显式推荐入口或移除策略治理该 helper。`common/security/auth` MUST 只提供通用 JWT verifier、Bearer token 处理和无业务语义安全 primitive，MUST NOT 暴露 access、refresh、password-change token 签发 API，MUST NOT 固定 user-service claims schema、token subject 或会话撤销语义。

#### Scenario: Casbin 授权 helper 收紧

- **WHEN** 调用方需要获得 Casbin 三元组授权的原始允许结果
- **THEN** 系统 MUST 提供 `common/security/casbin.Enforce` 作为返回 `bool` 和 `error` 的推荐入口
- **AND** 拒绝访问转换为 `ErrDenied` 的 error-only 语义 MUST 由 `Authorizer.Authorize` 或调用方显式处理

#### Scenario: JWT middleware 依赖最小 verifier

- **WHEN** 服务需要创建共享 JWT 认证 middleware
- **THEN** middleware MUST 只依赖能验证访问令牌的最小接口
- **AND** middleware MUST NOT 依赖具备 token 签发能力的 concrete JWT service
- **AND** access token 的 claims schema、subject 检查和服务业务字段校验 MUST 由服务私有 verifier adapter 拥有

#### Scenario: JWT middleware 不接收无效配置参数

- **WHEN** 服务需要创建共享 JWT 认证 middleware
- **THEN** middleware constructor MUST 只接收 logger、访问令牌 verifier 和可选 token version validator 作为调用参数
- **AND** middleware constructor MUST NOT 接收 `config.AuthConfig` 或其他不参与运行时认证行为的配置参数
- **AND** JWT 配置 MUST 由服务私有配置和服务私有 verifier/issuer 组合消费后再注入 middleware

#### Scenario: token version validator 函数适配器移除

- **WHEN** 服务需要为共享 JWT 认证 middleware 提供 token version 撤销校验
- **THEN** 调用方 MUST 直接提供实现 `common/security/auth.TokenVersionValidator` 的具体类型
- **AND** `common/http/middleware` MUST NOT 暴露只将函数包装为 `TokenVersionValidator` 的 `TokenVersionValidatorFunc` 适配器

#### Scenario: common 不暴露用户服务签发能力

- **WHEN** 任一服务导入 `common/security/auth`
- **THEN** 该包 MUST NOT 提供 `SignAccessToken`、`SignRefreshToken`、`SignPasswordChangeToken` 或等价签发入口
- **AND** 该包 MUST NOT 定义 `SubjectRefresh`、`SubjectPasswordChange` 或其他 user-service 认证流程 subject
- **AND** 该包 MUST NOT 定义包含 `user_id`、`token_version`、`session_id` 的 user-service 专属 claims 结构

#### Scenario: 行为保持不变

- **WHEN** 共享认证授权 helper 的重复入口、无效参数或签发能力被移除
- **THEN** 系统 MUST 保持 JWT 验签、token version 校验、Casbin 三元组校验、`ErrNotConfigured`、`ErrDenied` 和 HTTP 响应语义不变
- **AND** user-service 的认证路由挂载和 RBAC 保护路由 MUST 不因该 API 治理发生行为变化

### Requirement: Runtime primitive 基础

系统 MUST 在 `common/runtime/` 中维护配置加载、数据存储、logger、metrics、tracing、scheduler、workerpool、localcache、Redis key 和 timezone 等 runtime primitive。`common/runtime/config` MUST 将 `local_cache` 表达为通用具名缓存实例集合，并 MUST NOT 固定 user-service 的 `auth_token_version`、`rbac_user_roles` 或其他业务缓存名。`common/runtime/config` MUST 只声明和校验跨服务通用 runtime 配置，MUST NOT 声明或校验 user-service 的 `auth`、`ent`、JWT TTL、password KDF、refresh session 或 token version 策略。`common/runtime/config` MUST 使用 `github.com/go-viper/mapstructure/v2` 作为配置反序列化依赖，并 MUST NOT 保留旧版 `github.com/mitchellh/mapstructure` 导入、兼容层或旧行为 fallback。

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

#### Scenario: runtime 依赖初始化

- **WHEN** 服务需要连接 Postgres、Redis、logger、metrics 或 tracing provider
- **THEN** 服务 MUST 优先复用 `common/runtime/` 中的 provider 和 Fx module

#### Scenario: Postgres 启动探测失败关闭连接池

- **WHEN** 共享 Postgres Fx provider 已创建连接池但启动 PING 失败
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
- **THEN** workerpool MUST 只提供并发控制、生命周期、日志和统计能力，MUST NOT 承载 refresh session 上限裁剪、token version 撤销、可靠消息、eventbus、outbox 或业务一致性协议

#### Scenario: scheduler 分布式锁

- **WHEN** 定时任务具有多实例副作用
- **THEN** 任务 MUST 声明锁策略，锁 TTL MUST 为正值，长任务 SHOULD 具备续租策略

#### Scenario: mapstructure v2 配置反序列化

- **WHEN** `common/runtime/config` 将 Viper 读取到的配置反序列化为目标配置结构
- **THEN** 系统 MUST 使用 `github.com/go-viper/mapstructure/v2` 提供的 decode hook 和 decode 配置能力
- **AND** duration、slice、具名 Postgres、具名 Redis 和具名 `local_cache` 配置 MUST 按 v2 标准行为解析
- **AND** 系统 MUST NOT 导入 `github.com/mitchellh/mapstructure` 或保留面向旧版行为的兼容代码
