## Purpose

定义 `common/` 提供的跨服务共享契约、HTTP helper、安全原语、runtime primitive、测试基础设施和校验能力，保证服务间基础行为一致且业务边界清晰。
## Requirements
### Requirement: 跨服务 HTTP 契约与 helper

系统 MUST 在 `common/contract` 中维护业务中立的应用错误、响应 envelope 和分页契约，由 `common/http/response` 统一完成 HTTP 渲染，并 MUST 在 `common/http` 和 `common/validation` 中提供行为一致、业务中立的入站绑定、字段校验、认证授权 middleware、CORS、metrics、logging、recovery、OpenAPI 和轻量出站请求能力。应用错误 MUST 使用低基数 `Kind`、稳定 `Reason`、响应 `Code`、公开 `Message` 和可选内部 `Cause` 表达语义，MUST NOT 保存或接收 HTTP status；HTTP status MUST 只根据 `Kind` 推导。

#### Scenario: 响应、错误归一化与 HTTP 映射

- **WHEN** 服务返回成功、分页或错误响应
- **THEN** 系统 MUST 使用共享 envelope 表达 `success`、`code`、`message`、`data`、`pagination` 或结构化错误详情，新服务 MUST 优先复用 `common/contract` 和 `common/http/response`，不得定义不兼容的 envelope
- **WHEN** 系统创建、包装或通过 `FromError` 归一化错误
- **THEN** wrapped application error MUST 保留原始 `Kind`、`Reason`、`Code` 和公开 `Message`
- **AND** nil 或未知错误 MUST 归一化为使用非敏感公开消息的内部错误，原始错误只能作为内部 `Cause` 保留
- **AND** `errors.As` MUST 能解析应用错误，`errors.Is` MUST 按稳定 `Kind`、`Reason` 或内部 `Cause` 语义匹配
- **WHEN** `common/http/response` 写入应用错误响应
- **THEN** 请求格式或字段校验错误 MUST 渲染为 `400 Bad Request`
- **AND** 未认证或 token 无效、过期、撤销 MUST 渲染为 `401 Unauthorized`
- **AND** 权限不足、未找到、冲突和服务不可用 MUST 分别渲染为 `403 Forbidden`、`404 Not Found`、`409 Conflict` 和 `503 Service Unavailable`
- **AND** nil、未知或内部错误 MUST 渲染为 `500 Internal Server Error`

#### Scenario: HTTP 请求处理与 helper

- **WHEN** HTTP 请求被 Gin 路由处理
- **THEN** 服务 MUST 能复用共享 middleware 完成认证上下文、授权检查、日志字段、metrics、panic recovery 和 span error 记录
- **WHEN** HTTP handler 绑定请求或校验字段
- **THEN** `common/validation`、`common/http/binding` 和 response helper MUST 生成或传播语义应用错误，并返回一致的字段名、公开消息和结构化字段错误明细
- **AND** validation tag 的字段名解析顺序 MUST 保持稳定
- **WHEN** 调用方使用 `Created` 或 `NoContent` 写入响应
- **THEN** `Created` MUST 返回包含统一成功 envelope 和调用方 `data` 的 `201 Created`，`NoContent` MUST 返回 body 为空的 `204 No Content`

### Requirement: HTTP 入站请求体容量边界

系统 MUST 在共享 HTTP helper 中提供业务中立的入站请求体字节上限能力。该能力 MUST 在 JSON 解码前限制 `Request.Body` 可读取字节数，MUST 覆盖固定 `Content-Length`、chunked 请求体和首个 JSON 文档后的尾随数据，并 MUST 将超限错误渲染为 `413 Payload Too Large` 与统一错误 envelope。

#### Scenario: JSON 解码前拒绝超限请求体

- **WHEN** HTTP 请求体超过调用方配置的最大字节数，且请求使用固定 `Content-Length` 或 chunked 传输
- **THEN** 共享 body limit 能力 MUST 在业务 binder 完成解码前返回稳定超限错误
- **AND** 后续 feature use case MUST NOT 被调用

#### Scenario: 尾随 JSON 不能绕过容量边界

- **WHEN** 请求体包含一个合法 JSON 文档，随后追加导致总请求体超限的尾随 JSON 或其他数据
- **THEN** 系统 MUST 在读取尾随数据时继续受同一字节上限约束
- **AND** 响应 MUST 为 `413 Payload Too Large`，不得因第二次 JSON 解码分配与载荷大小线性增长的堆内存

#### Scenario: 超限错误使用统一响应契约

- **WHEN** 共享 HTTP helper、middleware 或 binder 产生请求体超限错误
- **THEN** `common/http/response` MUST 渲染 `413 Payload Too Large`
- **AND** 响应 MUST 使用统一错误 envelope、稳定应用错误码和非敏感公开消息
- **AND** 系统 MUST NOT 将请求体超限渲染为 `400 Bad Request`、`429 Too Many Requests` 或 `500 Internal Server Error`

#### Scenario: common 不拥有服务私有策略

- **WHEN** 消费服务需要默认请求体上限、端点覆盖或路径匹配策略
- **THEN** 这些策略 MUST 由消费服务拥有
- **AND** `common` MUST NOT 内置 user-service 路由、auth/user DTO、部署资源预算或服务私有配置字段

#### Scenario: 业务中立的出站 HTTP 请求

- **WHEN** 调用方使用共享 HTTP client helper 发送出站请求
- **THEN** helper MUST 基于 Resty 支持 query、header、JSON 或 form body、context、逐请求 timeout 和显式 HTTP(S) proxy URL，并 MUST 允许注入调用方拥有的 `*resty.Client`
- **AND** form 与 JSON 同时存在时 MUST 使用 form，并由 Resty 设置 `application/x-www-form-urlencoded`
- **AND** 零值 timeout MUST 使用 60 秒默认值，负值、空 URL、空 method 或非法 proxy MUST 在网络请求前失败
- **AND** helper MUST 使用调用方 context 表达逐请求 timeout，MUST NOT 为设置 timeout 修改共享或注入 client
- **AND** 默认 Resty client MUST 长期复用、MUST NOT 保存 cookie、启用 retry、记录请求或响应 body、注入业务认证信息；调用方注入 client 时 MUST 保留其已有 middleware、retry、transport、TLS 和 response body limit 行为
- **AND** `ProxyURL` 与注入 client 同时存在时 MUST 在网络请求前失败；固定或高频代理 MUST 由调用方预先配置在注入 client 上
- **WHEN** 调用方未注入自定义 TLS transport
- **THEN** helper MUST 保持 Go 默认 TLS 证书校验，MUST NOT 默认或通过隐式选项跳过证书校验
- **WHEN** 上游返回 HTTP 响应
- **THEN** 全部 2xx 状态 MUST 返回成功和完整 response body，其他状态 MUST 返回可检查的状态错误和 response body，错误文本 MUST NOT 包含 response body
- **AND** Resty 构造、middleware、body limit、context、TLS 或 transport 错误 MUST 返回失败、nil body 和可包装的原始错误
- **AND** 具体外部系统的 DTO、认证、重试、业务错误映射和防腐逻辑 MUST 留在消费服务的 `internal/integration/http` 或所属 feature 边界

#### Scenario: CORS 默认策略与预检

- **WHEN** 请求经过 `CORS()` middleware
- **THEN** 响应 MUST 默认包含 `Access-Control-Allow-Origin=*`、`Access-Control-Allow-Methods=GET,POST,PUT,PATCH,DELETE,OPTIONS` 和 `Access-Control-Allow-Headers=Authorization,Content-Type`
- **AND** 默认配置 MUST NOT 启用 credentials、max age、exposed headers 或 `Vary: Origin`
- **AND** middleware MUST 复制默认或调用方传入的 slice，调用方后续修改 MUST NOT 改变已创建 middleware 的行为
- **WHEN** `OPTIONS` 预检请求经过共享 CORS middleware
- **THEN** middleware MUST 返回带默认 CORS header 的 `204 No Content` 并停止调用后续 handler
- **AND** 非 `OPTIONS` 请求 MUST 调用后续 handler，并保持其 status 和 body 可见

#### Scenario: 业务语义与 OpenAPI 留在所属边界

- **WHEN** auth、permission、role 或 `internal/shared/identity` 定义稳定业务错误
- **THEN** owning domain MUST 为错误提供共享契约要求的 `Kind`、`Reason`、`Code` 和公开 `Message`，保持 `errors.Is` 匹配语义，并使 `common/http/response.Fail` 能直接渲染该错误
- **AND** 系统 MUST NOT 在 `common`、跨 feature 全局包或 HTTP transport 中维护重复的业务错误映射表
- **AND** `CodePasswordChangeRequired` 的数值 MUST 保持为 `20006`，但 `common` MUST NOT 承载 user-service 的状态判断、token 签发或登录编排
- **WHEN** 授权依赖 user-service 的 subject schema、角色、权限目录、route diff 或超级管理员基线
- **THEN** 行为 MUST 留在 user-service permission 或 shared 边界，不得进入通用 HTTP middleware 或 `common/security/casbin`
- **WHEN** 服务生成、转换或嵌入 OpenAPI 文档
- **THEN** 系统 MUST 复用 `common/http/openapi` 的规范化、序列化和 Go embed 渲染能力，API server、认证方案、扫描范围、健康路径和输出目录等服务元数据 MUST 留在服务脚本或薄 wrapper

### Requirement: 共享安全原语

系统 MUST 在 `common/security` 中提供业务中立的 JWT 验证、Bearer token 处理、Casbin 请求三元组授权和 bcrypt 密码哈希原语，MUST NOT 固定 user-service 的 claims schema、token subject、会话撤销或业务授权模型。

#### Scenario: JWT 与 Casbin 使用最小业务中立接口

- **WHEN** 服务创建共享 JWT 认证 middleware
- **THEN** middleware constructor MUST 只接收 logger、访问令牌 verifier 和可选 token version validator，MUST NOT 依赖 token issuer、服务私有配置或具备签发能力的 concrete service
- **AND** access token claims、subject 和业务字段校验 MUST 由服务私有 verifier adapter 拥有
- **AND** `common/security/auth` MUST NOT 提供 access、refresh 或 password-change token 签发入口，也 MUST NOT 定义 user-service 专属 subject 或 claims
- **WHEN** 调用方需要获得 Casbin 三元组授权的原始结果
- **THEN** `common/security/casbin.Enforce` MUST 返回 `bool` 和 `error`，拒绝访问到 `ErrDenied` 的转换 MUST 由 `Authorizer.Authorize` 或调用方显式处理

#### Scenario: bcrypt 密码哈希、校验与输入边界

- **WHEN** 服务、CLI 或测试需要执行密码哈希或校验
- **THEN** 调用方 MUST 显式创建 bcrypt 密码服务实例
- **AND** `common/security/password` MUST 使用固定 bcrypt cost 生成新密码哈希，初始 cost MUST 为 `12`
- **AND** `common/security/password` MUST 使用 bcrypt 校验已编码的密码哈希，非 bcrypt、格式非法或无法解析的哈希 MUST 被拒绝
- **AND** `common/security/password` MUST NOT 验证、迁移、fallback 或 rehash Argon2id 密码哈希，也 MUST NOT 暴露包级哈希、校验或可变算法配置入口
- **WHEN** 调用方提交空明文密码或超过 bcrypt 安全输入上限的明文密码
- **THEN** `common/security/password` MUST 在执行 bcrypt 前拒绝该输入，空密码和超长密码 MUST 分别返回可匹配的 `password.ErrEmptyPassword` 和 `password.ErrPasswordTooLong`

### Requirement: Runtime 配置、具名资源与生命周期

系统 MUST 在 `common/runtime/config` 中维护跨服务 runtime 配置、默认值和通用校验，在 `common/runtime/resources` 中维护具名 Redis/PostgreSQL 资源类型、默认值和通用校验，并由 `common/runtime/datastore` 使用共享资源类型初始化 framework-neutral 的单资源连接池。服务私有业务配置、必需资源名、业务用途和配置 map 到真实资源的选择 MUST 由消费服务拥有；constructor 部分失败、启动探测失败或 instrumentation 失败时 MUST 清理已创建资源并保留主错误和清理错误。

#### Scenario: 严格加载并校验通用配置

- **WHEN** 服务通过配置文件加载 runtime 配置
- **THEN** 共享 loader MUST 使用 `github.com/go-viper/mapstructure/v2` 的 decode 能力解析 duration、slice、具名配置，以及 runtime、HTTP、gRPC、metrics、tracing、pprof、logger 和通用 `local_cache` 配置
- **AND** 未声明字段 MUST 在启动前失败并报告完整路径，不得使用旧字段别名或 fallback
- **AND** 共享 runtime config MUST 声明并校验 `runtime.gin.mode`、server、logger、metrics、tracing、pprof、lifecycle 和通用 local cache 配置
- **AND** `runtime.gin.mode` 默认值 MUST 为 `release`，环境变量覆盖 MUST 使用 `AEGISCORE_RUNTIME_GIN_MODE`，合法值 MUST 仅为 `debug`、`release` 或 `test`
- **AND** `observability.pprof.enabled` 和 `observability.pprof.addr` 默认值 MUST 分别为 `false` 和 `127.0.0.1:6060`，production-like 环境启用 pprof 时 addr MUST 使用 loopback host
- **AND** 至少一个 HTTP 或 gRPC server MUST 启用
- **WHEN** 配置包含 `local_cache.<name>`
- **THEN** loader MUST 保留 `<name>` 并解析为通用缓存实例配置，validation MUST 校验 `capacity > 0`、`ttl > 0` 和 `load_timeout > 0`，错误 MUST 包含完整字段路径
- **AND** 配置契约 MUST NOT 暴露 Ristretto 的 `num_counters`、`buffer_items`、admission 或 write buffer 选项，必需缓存名及其业务含义 MUST 留在消费服务
- **WHEN** `runtime.lifecycle.stop_timeout` 小于 HTTP shutdown timeout、worker drain allowance、tracing flush allowance 和 shutdown safety margin 的组合最低预算
- **THEN** 配置校验 MUST 失败并指出该字段及最低所需预算；大于或等于组合最低预算时共享校验 MUST 继续通过，业务停止策略 MUST 由 owning feature 或服务组合层表达

#### Scenario: 服务私有配置与单一配置来源

- **WHEN** 服务需要 `auth`、`ent`、JWT TTL、refresh session、token version、RBAC 或 production-like secret 校验
- **THEN** 服务私有 loader MUST 负责解析和校验，`common/runtime/config` MUST NOT 声明或校验这些业务配置，服务私有配置 MUST NOT 声明、读取或兼容旧 `auth.password_kdf` 配置
- **WHEN** user-service 正式 `serve` 启动或装配测试构建 Fx App
- **THEN** CLI MUST 在创建 App 前只解析并校验一次 service config，composition root MUST supply 同一个 service config 及由其派生的 runtime config，不得再次读取配置文件
- **AND** `common/runtime/config` MUST NOT 通过 Fx provider 接受配置路径或重复调用共享 loader，配置失败 MUST 在创建 App、资源或 lifecycle hook 前返回

#### Scenario: 声明、校验并选择具名资源

- **WHEN** 服务声明一个或多个 Redis 或 PostgreSQL 实例
- **THEN** 服务 MUST 能使用 `RedisConfigs` 和 `PostgresConfigs` 按名称配置资源
- **AND** resources helper MUST 应用稳定 timeout、SSL 和连接池默认值，并对非法名称、地址、端口、timeout、SSL mode、username 或 pool 参数返回包含资源名和字段路径的错误
- **AND** PostgreSQL max idle connections MUST NOT 大于 max open connections，PostgreSQL password 与 Redis username/password MUST 允许为空
- **AND** 核心 runtime `Config` MUST NOT 固定这些资源或服务私有资源名，具名配置 map 到单份配置的选择 MUST 由消费服务负责

#### Scenario: 单资源构造、探测、清理与 Fx 所有权

- **WHEN** 调用方使用一份 `RedisConfig` 创建 Redis client
- **THEN** 统一 timeout MUST 映射到 dial、read、write 和启动 ping timeout，username MUST 可用于 Redis ACL
- **AND** 普通 Go constructor 和 Fx constructor MUST 一次只创建一个 client，且 MUST NOT 接收或遍历 `RedisConfigs`
- **AND** 启动 PING 或 tracing instrumentation 失败时 provider MUST 关闭 client，并保留探测、instrumentation 和关闭失败信息
- **WHEN** 调用方使用资源名称和一份 `PostgresConfig` 构造 PostgreSQL 资源
- **THEN** `common/runtime/datastore` MUST 一次只创建一个连接池并应用稳定的 DSN、SSL 和 pool 默认值，constructor MUST NOT 接收 `PostgresConfigs`、`fx.Lifecycle` 或其他 DI framework 类型
- **AND** datastore MUST 提供接收资源名称和 `*sql.DB` 的启动 `Ping` 与 `Close` 契约；启动 `Ping` 失败时 MUST 使用稳定的单资源 ping timeout、关闭同一连接池并同时保留具名探测错误和关闭错误；关闭时 MUST 只关闭该单个连接池且错误 MUST 包含资源名称
- **WHEN** Fx 服务声明一个具名 Redis 或 PostgreSQL 资源
- **THEN** `common/runtime/datastore` 中共置的 Fx adapter MUST 调用 framework-neutral 的单资源 constructor，只注册该资源的启动探测和关闭 hook，MUST NOT 遍历具名资源 map 或自动创建其他资源
- **AND** `OnStart` 创建的资源 MUST 由 `OnStop` 或 Fx rollback 关闭，constructor 阶段已创建的部分资源 MUST 在后续失败时立即清理
- **AND** feature 关闭自身 workerpool、watcher、cache 或 store 时 MUST NOT 关闭共享 Redis、Ent 或 PostgreSQL 资源

### Requirement: Redis mode-driven 资源契约

系统 MUST 将共享 Redis 资源契约定义为 mode-driven 配置，`mode: cluster` MUST 使用 `addrs`、`timeout` 和可选 `cluster.max_redirects`，`mode: standalone` MUST 使用 `addr` 和 `timeout`。`addrs` MUST 表示 Redis Cluster seed endpoints，并 MUST 允许只配置一个阿里云 Redis 集群访问地址。Redis DB MUST NOT 作为配置项暴露；Cluster 与 standalone 均 MUST 固定使用 Redis 0 号库。系统 MUST NOT 支持 Sentinel 或根据字段隐式推断 mode。

#### Scenario: 加载最小 Cluster 配置

- **WHEN** 配置包含 `resources.redis.cache_redis.mode=cluster`、至少一个 `addrs` 地址和正数 `timeout`
- **THEN** 配置加载、默认值应用和通用 validation MUST 成功
- **AND** 每个 `addrs` 元素 MUST 按 `host:port` 校验，错误路径 MUST 包含资源名和字段路径

#### Scenario: 按 mode 校验 Redis 配置

- **WHEN** `mode=cluster` 的配置包含 `addr` 或 `db`，或 `mode=standalone` 的配置包含 `addrs`、`cluster.max_redirects` 或 `db`
- **THEN** Redis resource validation MUST 在启动前失败并报告完整字段路径
- **AND** 未声明 mode、未知 mode、Sentinel 字段或未知 Redis 字段 MUST 在启动前失败

### Requirement: Redis Cluster client 构造与生命周期

系统 MUST 通过共享 datastore 构造可承载 Redis Cluster 的单资源 client，并在 Fx lifecycle 中执行启动 PING、tracing instrumentation 和关闭清理。Redis client 公开边界 MUST 支持 Cluster client，MUST NOT 要求 `*redis.Client` 单机 concrete type。

#### Scenario: 构造并探测 Cluster client

- **WHEN** 调用方使用有效 Redis Cluster 资源配置创建 `cache_redis`
- **THEN** datastore MUST 使用 Cluster client 初始化，并将 `timeout` 映射到 dial、read、write 和启动 PING timeout
- **AND** `cluster.max_redirects` 配置存在时 MUST 映射到 Cluster redirect 上限
- **AND** 启动 PING 或 tracing instrumentation 失败时 MUST 关闭已创建 client，并保留主错误和关闭错误

#### Scenario: client 所有权与 feature 边界

- **WHEN** feature 消费共享 Redis 资源
- **THEN** feature MUST 只消费 Cluster-capable client 或消费侧最小接口
- **AND** feature 自有 workerpool、watcher、cache 或 store 关闭时 MUST NOT 关闭共享 Redis client

### Requirement: Runtime 执行与装配原语

系统 MUST 在 `common/runtime` 中提供业务中立的 ID、scheduler、workerpool、localcache、Redis key、timezone、logger、Fx provider 和依赖图原语。后台执行 MUST 具有明确的容量、并发、失败处理、观测和关闭语义；构造函数、provider 和 Fx graph helper MUST 只消费真实运行时依赖或调用方显式提供的无副作用 Fx option，MUST NOT 为测试便利暴露生产 API 或读取服务私有配置。公开 provider 名称 MUST 表达其 runtime 能力或资源职责，MUST NOT 仅用模糊的 DI framework 术语隐藏能力语义。

#### Scenario: workerpool 与 scheduler 生命周期

- **WHEN** 调用方通过 `workerpool.New` 创建任务池并通过 `Stop(ctx)` 关闭
- **THEN** task pool MUST 作为不依赖 Fx 的普通 Go 资源创建并由拥有者显式关闭；Stop MUST 停止接收新任务、等待已登记或已接受任务 drain，并允许重复调用共享同一 drain 状态
- **AND** Stop 超时 MUST 返回包装 `context.DeadlineExceeded` 的错误，workerpool MUST NOT 承载 refresh session、token version、可靠消息、eventbus、outbox 或业务一致性语义
- **WHEN** scheduler 触发已注册任务
- **THEN** 系统 MUST 按本地 overlap gate、全局并发 gate、可选分布式锁、任务 context、可选锁续租、任务执行和 cleanup 的顺序处理，并 MUST 记录跳过、开始、完成、失败、拒绝和 panic，在 shutdown 时优雅停止
- **AND** 多实例副作用任务 MUST 声明正数 TTL 的分布式锁策略，长任务 SHOULD 使用续租
- **AND** 即使任务未配置 timeout，scheduler MUST 创建可取消 context，并在自动续租失败时取消任务和记录失败

#### Scenario: loading cache 构造、读取与回源

- **WHEN** 服务通过 `NewLoadingCache` 创建 loading cache
- **THEN** 配置 MUST 只包含非空名称、正数 `uint64` 容量、正数固定 TTL 和正数 load timeout，并 MUST 提供 loader
- **AND** 容量 MUST 表示最大 item 数，不得表示字节、自定义 cost 或 Ristretto admission 参数；cache key MUST 保留调用方选择的 comparable 类型，common MUST NOT 要求业务 key 字符串编码
- **AND** 公开 API MUST NOT 暴露底层 `ttlcache` 配置、独立 `Get`、主动 `Set`、`CloneFunc` 或写入拒绝语义
- **WHEN** `GetOrLoad` 命中未过期 item
- **THEN** cache MUST 返回该值并记录一次 hit，且读取 MUST NOT 延长该 item 的固定 TTL
- **WHEN** `GetOrLoad` 未命中
- **THEN** cache MUST 为每个调用记录一次 miss，并使用 `singleflight` 合并同一业务 key 的并发回源；loader 成功 MUST 记录 `LoadSuccess` 并同步写入 bounded TTL cache，失败 MUST 记录 `LoadError` 且不得缓存错误结果
- **AND** 内部 double-check MAY 避免重复回源，但 MUST NOT 计为业务 hit 或成为公开统计字段
- **WHEN** 同 key 回源正在执行且任一 caller context 被取消
- **THEN** 该 caller MUST 返回其 context error，MUST NOT 因自身取消而终止其他等待者共享的 loader；loader context MUST 保留发起请求的 context values、解除 caller cancellation，并受 `LoadTimeout` 限制

#### Scenario: loading cache 值所有权、失效与关闭

- **WHEN** cache 存储或返回 slice、map、pointer 或包含引用字段的 value
- **THEN** common MUST NOT 执行业务 deep clone，消费 feature MUST 在 loader 写入和返回调用方的适当边界复制可变 value
- **WHEN** 调用方执行 `Delete` 或 `Clear`
- **THEN** 对应 item 或全部 item MUST 在方法成功返回时失效，显式失效 MUST NOT 计入自动 eviction
- **WHEN** 调用方一次或多次执行 `Close`
- **THEN** cache MUST 幂等停止 wrapper 拥有的 TTL 清理 goroutine 并阻止新的底层访问，关闭后的 `GetOrLoad`、`Delete` 和 `Clear` MUST 返回 `ErrClosed`
- **AND** 已开始的 loader MAY 在 load timeout 内完成，但关闭后 MUST NOT 再写入底层 cache；`Name` 和 `Stats` MUST 在关闭后继续返回稳定名称与最后累计快照

#### Scenario: Redis key、timezone 与 logger 归属

- **WHEN** feature 需要 refresh session、token version、RBAC 或其他业务 Redis key
- **THEN** feature infrastructure MUST 拥有业务 key schema，并只能复用 `common/runtime/rediskey` 的通用构造规则
- **WHEN** runtime 初始化进程时区
- **THEN** timezone primitive MUST 优先使用平台 `TZ` 环境变量并在缺省时使用稳定默认值，MUST NOT 依赖核心 Config 或服务业务配置
- **AND** 如果通过 Fx 初始化，服务 composition root MUST 显式绑定初始化调用或服务级 runtime 初始化函数，common MUST NOT 仅为了包装 `Init` 暴露无额外运行时职责的 Fx provider
- **WHEN** 调用方通过 `logger.New`、`NewWithConfig` 或 Fx provider `NewLogger` 创建 logger
- **THEN** 系统 MUST 返回由调用方拥有的 logger，Fx provider MUST 注册既有 Sync 关闭 hook；构造过程 MUST NOT 隐式安装、覆盖或恢复进程级默认 logger
- **AND** 默认 logger 只能通过显式 `SetDefault` 修改，并 MAY 作为未注入 logger 时的兜底

#### Scenario: 共享 provider、fxgraph 与公开 API 边界

- **WHEN** 共享 provider 暴露依赖
- **THEN** provider MUST 只消费跨服务配置和 primitive，不得导入服务私有配置；公开命名 MUST 能区分 logger、metrics、tracing、datastore 或其他具体 runtime 能力，不得在多个 common 包中重复使用缺少能力语义的通用名称作为主要入口
- **WHEN** 服务将 Fx option 或 module 传入 `common/runtime/fxgraph`
- **THEN** helper MUST 输出稳定排序的 provider、invoke 和依赖关系图文本，只处理调用方显式传入的 graph-safe Fx option，MUST NOT 构造或要求服务私有配置、feature provider、Ent、Redis、PostgreSQL、OTLP 或 HTTP server 输入
- **AND** helper MUST NOT 通过服务完整 runtime module 间接执行生产 runtime `fx.Invoke`
- **WHEN** `common/runtime` 新增公开 constructor、method、option 或 hook
- **THEN** 入口 MUST 具有真实运行时职责或已定义的稳定共享契约；仅测试消费、暴露内部状态或绕过正常 lifecycle 的能力 MUST 留在包内、`_test.go` fixture 或 `common/testing`
- **AND** 仅包装另一个无参初始化函数且不提供额外资源、配置、错误处理、顺序控制或 lifecycle 语义的 Fx provider MUST NOT 作为 common 公开 API 新增或保留

### Requirement: common、shared 与外部集成边界

系统 MUST 将 `common` 限制为跨服务稳定且业务中立的能力，将 `user-service/internal/shared` 限制为至少两个 feature 真实消费的服务内纯业务内核，并将真实外部协议适配放入 `user-service/internal/integration/http|grpc|events`。

#### Scenario: common 与 shared kernel 准入边界

- **WHEN** 新能力准备放入 `common`
- **THEN** 能力 MUST 跨服务可复用、无 user-service 业务语义且具有稳定契约
- **AND** `common` MUST NOT 依赖 feature 包或承载业务 DTO、业务 key schema、policy loader、route diff、服务 OpenAPI 元数据、eventbus 或 outbox 设计
- **WHEN** user/auth 共享身份语义或 role/permission 共享系统 RBAC 基线
- **THEN** 系统 MUST 分别使用 `internal/shared/identity` 和 `internal/shared/rbacbaseline`
- **AND** shared 包 MUST 保持纯业务语义，不得依赖 feature、Gin、Ent、Redis、SQL、Fx、runtime provider、HTTP response、DTO、store port、外部调用或部署资产
- **AND** helper 只被一个 feature 使用、只是技术工具或需要基础设施依赖时，系统 MUST NOT 将其放入 `internal/shared`
- **AND** shared 子包 MUST 使用具体业务名称，不得创建根级 `errors`、`enums`、`types`、`utils` 或 `helpers` 兜底包

#### Scenario: 外部集成准入与归属边界

- **WHEN** feature 调用真实外部 HTTP、gRPC 或事件系统
- **THEN** feature application MUST 拥有最小消费侧端口，`integration/http|grpc|events` MUST 只实现协议适配与防腐
- **AND** 入站 gRPC handler MUST 位于所属 feature 的 `transport/grpc`，feature-specific consumer 映射和 handler MUST 位于所属 feature
- **WHEN** 没有真实 broker、外部 API 或单独批准的设计
- **THEN** 系统 MUST NOT 新增 eventbus、outbox、producer、subscriber、consumer handler、dispatcher、Ent hook 或 transaction wrapper

### Requirement: 测试基础设施与隔离

系统 MUST 在 `common/testing` 中提供可复用的容器和 fixture，并 MUST 使用可重复、可观察且不污染生产 API 或进程全局状态的测试方式验证共享能力。

#### Scenario: 可重复且可观察的测试

- **WHEN** Go 测试需要真实 PostgreSQL 或 Redis
- **THEN** 测试 MUST 优先使用 `common/testing/containers` 管理依赖生命周期，测试数据 MUST 使用稳定 fixture 或 feature-local builder，避免不可重复的随机输入
- **WHEN** 测试需要注入失败、固定返回、控制顺序或观察后台状态
- **THEN** 测试 MUST 使用消费侧最小接口、局部 fixture、通道或可观察状态，正式代码 MUST NOT 为测试新增全局可变函数、测试 flag、`NewXForTest` 或无运行时职责的 adapter
- **WHEN** 测试验证缓存过期、workerpool drain、scheduler 续租或后台任务取消
- **THEN** 测试 MUST 使用通道、eventually-style 条件或其他可观察同步机制和明确 deadline，MUST NOT 只依赖固定 `time.Sleep` 判断状态已经变化

#### Scenario: 隔离进程级状态

- **WHEN** 测试必须修改默认 logger、`TZ`、`time.Local` 或包级初始化状态
- **THEN** 测试 MUST 在 package-local helper 中保存状态并通过 cleanup 恢复
- **AND** 环境变量 MUST 使用 `t.Setenv`，相关测试 MUST NOT 并行执行
- **AND** 非测试目标所需的日志捕获 MUST 使用 context logger 或局部 logger 注入

### Requirement: Redis Cluster 测试基础设施

系统 MUST 提供可由集成测试复用的 Redis Cluster 测试能力，用于验证 hash slot、多 key Lua、Pub/Sub、PING 和 MOVED/ASK redirect 相关行为。普通单元测试 MAY 继续使用 mock 或轻量 Redis fixture，但 Cluster 兼容性 MUST 通过真实 Redis Cluster 覆盖。

#### Scenario: 真实 Cluster 集成测试

- **WHEN** `AEGISCORE_TEST_CONTAINERS=1` 启用真实依赖测试
- **THEN** Redis Cluster 相关集成测试 MUST 实际连接 Cluster fixture 并执行 Cluster-sensitive Redis 命令
- **AND** Docker daemon、Cluster fixture 启动、slot 初始化或连接失败 MUST 使相关集成测试失败而不是静默跳过

### Requirement: 显式配置来源与加载管线

系统 MUST 将配置文档来源、文档合成、严格解码和服务配置策略表达为显式边界。`common/runtime/config` MUST 只拥有业务中立的配置 schema、document contract、deep merge、raw digest、strict decode、encode、render、redact 和通用校验原语；具体 Nacos 环境、认证、client、failover 和文档读取 MUST 位于 Nacos source adapter。服务 MUST 显式组合 defaults、normalize 和 validate，shared loader MUST NOT 通过服务类型的隐式接口自动发现这些行为。本地配置目录与 Namespace 的发布选择 MUST 留在 Compose 初始化服务和仓库级发布工具边界，runtime Nacos source MUST NOT 读取或解释仓库目录。

#### Scenario: Nacos 缺省文档来源

- **WHEN** 服务已设置必需 Nacos 环境变量但未设置 `AEGISCORE_NACOS_DATA_IDS`
- **THEN** Nacos source MUST 按 `base.yaml`、`resources.yaml`、`<service>.yaml` 的稳定顺序读取文档
- **AND** 显式 `AEGISCORE_NACOS_DATA_IDS` MUST 继续按声明顺序读取，认证、timeout 和 server failover 行为 MUST 保持不变

#### Scenario: 本地主机和 Compose 选择独立 Namespace

- **WHEN** 本地主机或 Compose workload 使用各自完整 Nacos 配置
- **THEN** 进程 MUST 通过 `AEGISCORE_NACOS_NAMESPACE` 显式选择 `loca-host` 或 `loca-docker`
- **AND** 两个 Namespace MUST 都使用 `base.yaml`、`resources.yaml`、`user-service.yaml` 三 dataId，不得要求环境专用第四文档
- **AND** Nacos source MUST 只根据已解析环境加载文档，MUST NOT 读取 Git 配置目录、感知 Compose 初始化服务、推断主机或容器环境、创建 Namespace 或改写资源地址

### Requirement: Go 模块依赖声明

`common` 模块 MUST 将生产代码直接 import 的第三方模块声明为 direct require，MUST NOT 将直接 import 的运行时依赖标记为 `// indirect`。

#### Scenario: PostgreSQL tracing direct dependency

- **WHEN** `common/runtime/datastore/postgres.go` 直接 import `github.com/XSAM/otelsql`
- **THEN** `common/go.mod` MUST 在 direct require 组声明 `github.com/XSAM/otelsql`
- **AND** 该 require MUST NOT 标记为 `// indirect`

#### Scenario: 文档合成与严格解码

- **WHEN** source 返回多份 YAML 文档
- **THEN** pipeline MUST 按文档顺序递归合并 map，后者 MUST 覆盖相同 scalar、slice 或显式 null，未被覆盖的嵌套字段 MUST 保留
- **AND** pipeline MUST 依次执行 raw settings 合成、raw digest、strict decode、normalize 和 validate
- **AND** 未声明配置键 MUST 在返回 typed config 前失败并报告完整叶子路径

#### Scenario: 显式服务配置策略

- **WHEN** user-service 解码服务私有配置
- **THEN** 调用点 MUST 显式提供完整默认配置、可选 normalize 和最终 validate
- **AND** shared loader MUST NOT 依赖 `ConfigDefaults()`、`ApplyDefaults()` 或其他按目标类型自动发现的服务 hook
- **AND** user-service 的 auth、RBAC、Ent、具名资源默认值和业务校验 MUST 留在 `user-service/internal/config`

#### Scenario: Raw digest 与 effective render

- **WHEN** pipeline 成功合成 raw settings
- **THEN** `SourceMetadata.Digest` MUST 基于 defaults 和 normalize 前的 raw merged settings 生成稳定摘要
- **AND** 默认值、normalize 或 typed config 的后续修改 MUST NOT 反向改变已记录的 source digest
- **WHEN** CLI 渲染 effective settings
- **THEN** 系统 MUST 从最终 typed config 编码可读 duration 和 mapstructure 字段，并在输出前脱敏 JWT、Redis、PostgreSQL 及调用方声明的敏感路径
- **AND** effective render MUST NOT 泄漏原始 secret

#### Scenario: 新增配置来源

- **WHEN** 测试或未来服务使用非 Nacos 配置来源
- **THEN** 来源 MUST 能通过业务中立的 document source contract 接入同一 merge、digest、decode、normalize 和 validate 管线
- **AND** 新来源 MUST NOT 要求修改 Nacos adapter 或把服务业务配置加入 `common`

### Requirement: 共享限流错误与 HTTP 映射

系统 MUST 在 `common/contract` 中提供业务中立的限流应用错误语义，并在 `common/http/response` 中将该错误渲染为 `429 Too Many Requests`。限流错误 MUST 使用稳定低基数 `Kind`、`Reason`、`Code` 和公开 `Message`，不得暴露 limiter key、IP、User ID、token、内部分片或实现细节。

#### Scenario: 限流错误响应

- **WHEN** HTTP middleware 或 handler 返回限流应用错误
- **THEN** `common/http/response` MUST 渲染 `429 Too Many Requests`
- **AND** 响应 envelope MUST 为 `success=false`、稳定限流 code 和公开限流 message

#### Scenario: 未知错误不伪装为限流

- **WHEN** 系统返回 nil、未知错误或内部错误
- **THEN** `common/http/response` MUST 保持现有内部错误归一化语义
- **AND** 系统 MUST NOT 将非限流错误映射为 `429 Too Many Requests`

### Requirement: 业务中立本地限流 primitive

`common/http/middleware` 或 `common/runtime` MUST 提供业务中立的本地限流 primitive。该 primitive MUST 支持基于调用方提供 key 的 `Allow` 判定、`golang.org/x/time/rate` token bucket、分片存储、后台清理和显式关闭。

#### Scenario: 调用方提供限流 key

- **WHEN** 调用方使用共享限流 middleware
- **THEN** 调用方 MUST 提供 key resolver 或等价 key 来源
- **AND** 共享 primitive MUST NOT 内置 user-service 路由、业务 DTO、权限目录、Casbin subject 或服务私有限流阈值

#### Scenario: 本地 limiter 并发访问

- **WHEN** 多个请求并发访问不同 IP 或 User ID 对应的限流 key
- **THEN** 本地限流 store MUST 使用分片或等价机制降低单一全局锁竞争
- **AND** 每个 key MUST 拥有独立 token bucket 状态

### Requirement: 应用错误码段治理

系统 MUST 将 `common/contract/errors.Code` 作为稳定公开应用码治理，并 MUST 在共享契约中维护明确的错误码段分配、预留范围和扩展准入规则。`Code` MUST NOT 作为 HTTP status 使用，HTTP status MUST 继续只根据低基数 `Kind` 推导。

#### Scenario: 使用既定错误码段

- **WHEN** 系统定义或审查 `common/contract/errors.Code`
- **THEN** `0` MUST 只用于成功响应 `CodeOK`
- **AND** `10xxx` MUST 用于请求解析、绑定和字段校验错误
- **AND** `20xxx` MUST 用于认证、凭证、token、session 和账号登录态错误
- **AND** `30xxx` MUST 用于授权、访问控制和策略拒绝错误
- **AND** `40xxx` MUST 用于业务冲突、资源状态不允许和幂等冲突错误
- **AND** `50xxx` MUST 用于资源不存在或不可见错误
- **AND** `60xxx` MUST 预留给限流、配额或用量约束，启用前必须先定义对应 `Kind`、HTTP 映射和测试
- **AND** `70xxx` 至 `89xxx` MUST 保持预留，未经规格变更不得使用
- **AND** `90xxx` MUST 用于内部错误、依赖不可用和服务端临时故障

#### Scenario: 新增错误码准入

- **WHEN** 系统新增应用错误码
- **THEN** 新错误码 MUST 优先复用现有低基数 `Kind`，并使用稳定 `Reason` 表达可细分原因
- **AND** 系统 MUST NOT 按 feature、目录、临时实现任务或调用方便利随意开辟错误码段
- **AND** 新错误码 MUST 位于其语义对应的既定段位内，不得复用其他稳定公开错误码数值
- **AND** 内部错误对外 MUST 使用非敏感公开消息，`Cause` 不得进入响应 envelope

#### Scenario: 新增 Kind 同步 HTTP 映射

- **WHEN** 现有 `Kind` 无法表达新的低基数 HTTP 映射语义，系统新增 `Kind`
- **THEN** 系统 MUST 同步更新 `common/http/response.statusCode` 的 HTTP status 推导
- **AND** 系统 MUST 添加或更新响应测试，覆盖新增 `Kind` 到 HTTP status 和响应 code 的映射
- **AND** 未定义 HTTP 映射的 `Kind` MUST NOT 作为公开业务错误进入 feature 或 HTTP transport

### Requirement: HTTP trusted proxy 配置契约

系统 MUST 在共享 runtime HTTP server 配置中提供 `server.http.trusted_proxies`，用于声明 Gin 可信任的上游代理 IP 或 CIDR 列表。该配置 MUST 由 `common/runtime/config` 严格解码、默认保持空值，并由服务级 Gin engine 初始化直接传入 `SetTrustedProxies`；系统 MUST NOT 读取、迁移、兼容或双写 `http.trusted_proxies` 或其他旧配置位置。

#### Scenario: 默认不信任代理

- **WHEN** `server.http.trusted_proxies` 未配置或为空
- **THEN** Gin engine MUST 不信任任何代理
- **AND** `c.ClientIP()` MUST 忽略 `X-Forwarded-For` 和 `X-Real-IP`，只返回请求 TCP peer 地址

#### Scenario: 显式信任代理 CIDR

- **WHEN** `server.http.trusted_proxies` 包含请求 TCP peer 所属的 IP 或 CIDR
- **THEN** Gin engine MUST 使用 Gin trusted proxy 机制解析 `X-Forwarded-For` 或 `X-Real-IP`
- **AND** `c.ClientIP()` MUST 返回可信代理链解析后的客户端地址

#### Scenario: 拒绝旧配置位置

- **WHEN** 配置文档包含 `http.trusted_proxies` 或其他未声明 trusted proxy 键
- **THEN** 严格配置解码 MUST 失败并报告完整配置路径
- **AND** 系统 MUST NOT 通过 normalize、alias 或 fallback 接受该旧配置

### Requirement: 通用事务生命周期 helper

`common/runtime/datastore` MUST 提供业务中立的泛型事务生命周期 helper，用于 infrastructure 代码创建、提交和回滚显式事务边界。helper MUST 只依赖标准库和最小事务接口，MUST NOT 导入 user-service、Ent 生成代码、feature 包或服务私有类型。

#### Scenario: 使用 detached lifecycle context 创建事务

- **WHEN** infrastructure 代码通过共享 helper 使用 request context 开始事务
- **THEN** helper MUST 使用保留原始 context values 的事务 lifecycle context 调用事务 starter
- **AND** 事务 lifecycle context MUST NOT 继承 request cancellation
- **AND** 原始 context 存在 deadline 时，事务 lifecycle context MUST 继承该 deadline
- **AND** 原始 context 不存在 deadline 时，事务 lifecycle context MUST 使用有界 cleanup timeout

#### Scenario: 事务内业务操作使用原始 request context

- **WHEN** application 或 infrastructure 代码在事务内执行 SQL 或 Ent 业务操作
- **THEN** 这些业务操作 MUST 继续使用原始 request context
- **AND** request cancellation MUST 仍能中断事务内业务查询

#### Scenario: request 取消后拒绝提交

- **WHEN** 原始 request context 在 commit 前已经取消
- **THEN** 事务完成器 MUST 拒绝提交并返回原始 context error
- **AND** 调用方 MUST 通过事务完成器的 rollback 兜底路径回滚事务

#### Scenario: 失败分支保留 rollback 错误

- **WHEN** 事务内业务操作失败且 rollback 也失败
- **THEN** 事务完成器 MUST 返回同时保留原始业务错误和 rollback 错误的 error
- **AND** rollback 错误 MUST NOT 被吞掉或覆盖原始业务错误

### Requirement: 禁止直接事务边界

新增或改造后的 infrastructure 事务边界 MUST 通过 `common/runtime/datastore` 的共享事务 helper 创建、提交和回滚。业务 infrastructure MUST NOT 直接调用 `client.Tx(ctx)`、`db.BeginTx(ctx, ...)`、`tx.Commit()`、`tx.Rollback()` 或手写 `rollback(tx, err)` helper 管理可由共享 helper 表达的事务生命周期。

#### Scenario: Ent 事务边界适配共享 helper

- **WHEN** user-service infrastructure 需要创建 Ent 事务
- **THEN** infrastructure 包 MUST 在消费侧适配 Ent client 到共享 transaction starter 接口
- **AND** Ent 适配器 MUST 留在 user-service infrastructure 边界内
- **AND** `common/runtime/datastore` MUST NOT 依赖 user-service Ent 生成类型

#### Scenario: 使用事务完成器终结事务

- **WHEN** infrastructure 代码已经通过共享 helper 开始事务
- **THEN** 成功路径 MUST 使用事务完成器执行 commit
- **AND** 错误路径 MUST 使用事务完成器执行 rollback 并保留原始错误
- **AND** defer 兜底路径 MUST 使用事务完成器在未提交时回滚事务
- **AND** 调用方 MUST NOT 绕过事务完成器直接提交或回滚事务
