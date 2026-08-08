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

### Requirement: Redis mode-driven 配置、client 与生命周期

系统 MUST 将共享 Redis 资源定义为 mode-driven 配置，并通过 Cluster-capable 公开 client 边界构造、探测和关闭单个资源。`cluster` MUST 使用 `addrs`，`standalone` MUST 使用 `addr`；系统 MUST NOT 暴露 Redis DB、支持 Sentinel、隐式推断 mode 或要求 feature 依赖 `*redis.Client` 单机 concrete type。

#### Scenario: 加载最小 Cluster 配置

- **WHEN** 配置包含 `resources.redis.cache_redis.mode=cluster`、至少一个 `addrs` 地址和正数 `timeout`
- **THEN** 配置加载、默认值应用和通用 validation MUST 成功
- **AND** 每个 `addrs` 元素 MUST 按 `host:port` 校验，错误路径 MUST 包含资源名和字段路径

#### Scenario: 按 mode 校验 Redis 配置

- **WHEN** `mode=cluster` 的配置包含 `addr` 或 `db`，或 `mode=standalone` 的配置包含 `addrs`、`cluster.max_redirects` 或 `db`
- **THEN** Redis resource validation MUST 在启动前失败并报告完整字段路径
- **AND** 未声明 mode、未知 mode、Sentinel 字段或未知 Redis 字段 MUST 在启动前失败

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

系统 MUST 在 `common/runtime` 中提供业务中立的 ID、scheduler、workerpool、localcache、Redis key、timezone、logger、Fx provider 和依赖图原语。拥有后台执行的 primitive MUST 具有明确的容量、并发、失败处理、观测和关闭语义；localcache MUST NOT 拥有后台执行或关闭生命周期。构造函数、provider 和 Fx graph helper MUST 只消费真实运行时依赖或调用方显式提供的无副作用 Fx option，MUST NOT 为测试便利暴露生产 API 或读取服务私有配置。公开 provider 名称 MUST 表达其 runtime 能力或资源职责，MUST NOT 仅用模糊的 DI framework 术语隐藏能力语义。

#### Scenario: workerpool 生命周期

- **WHEN** 调用方通过 `workerpool.New` 创建任务池并通过 `Stop(ctx)` 关闭
- **THEN** task pool MUST 作为不依赖 Fx 的普通 Go 资源创建并由拥有者显式关闭；Stop MUST 停止接收新任务、等待已登记或已接受任务 drain，并允许重复调用共享同一 drain 状态
- **AND** Stop 超时 MUST 返回包装 `context.DeadlineExceeded` 的错误，workerpool MUST NOT 承载 refresh session、token version、可靠消息、eventbus、outbox 或业务一致性语义

#### Scenario: Scheduler 注册与配置状态

- **WHEN** 调用方注册 scheduler job
- **THEN** job MUST 提供裁剪后非空且唯一的固定 key、有效 spec、非 nil `func(context.Context) error` task 和非负 timeout
- **AND** scheduler MUST 接受标准五字段、可选 seconds、descriptors、scheduler timezone 和 `CRON_TZ`，并 MUST 支持按固定 key 删除任务而不向调用方暴露底层 `cron.EntryID`
- **AND** nil lock policy MUST 表示不启用分布式锁，非 nil lock policy MUST 表示启用；零 `WaitTimeout` MUST 表示单次尝试，正数 MUST 表示在该上限内等待
- **AND** nil renew policy MUST 表示不续租，非 nil renew policy MUST 表示启用续租；公开配置 MUST NOT 同时保留可与 nil/non-nil 或 timeout 语义冲突的 lock enabled、lock mode 或 auto renew 开关

#### Scenario: Scheduler 分阶段执行与本地重叠

- **WHEN** scheduler 触发已注册任务
- **THEN** 系统 MUST 按 triggered、本地 overlap gate、全局并发 gate、可选分布式锁、任务 context、可选锁续租、started/result 观测、任务执行和逆序 cleanup 的顺序处理
- **AND** 每个内部 stage MUST 只释放自己成功取得的资源，error、panic、skip 和 cancellation MUST NOT 泄漏 local/global token、lock、context cancel 或 renew goroutine
- **AND** `AllowOverlap=false` 时同 key 前一实例仍运行的触发 MUST 立即跳过并记录 `local_overlap`，`AllowOverlap=true` 时不同触发 MAY 并行执行
- **AND** 即使任务未配置 timeout，scheduler MUST 创建可由 shutdown 取消的 context；配置 timeout 时 MUST 创建受 timeout 限制的 context
- **AND** 正常返回 MUST 记录 completed，返回 error 或 panic MUST 记录 failed，panic MUST 由 `robfig/cron` recovery 记录且不得永久占用已获取资源
- **AND** completed/failed duration MUST 从 started 时刻计算，MUST NOT 包含 overlap、全局并发或锁等待时间

#### Scenario: Scheduler 全局并发策略

- **WHEN** `MaxConcurrentJobs` 为零
- **THEN** scheduler MUST 不施加全局并发限制
- **WHEN** 并发上限已满且 policy 为 skip
- **THEN** 当前触发 MUST 立即跳过并记录 `global_concurrency_limit`
- **WHEN** 并发上限已满且 policy 为 wait
- **THEN** 当前触发 MUST 等待配额或 scheduler root context 取消，取得配额后 MUST 继续执行并在结束时释放
- **AND** scheduler MUST 保留 `AllowOverlap=true` 与 wait 组合的既有等待语义，文档 MUST 明确高频触发可能产生等待 goroutine，不得静默改为 skip 或无界持久化队列

#### Scenario: Scheduler 分布式锁、重试与续租

- **WHEN** job 配置分布式锁且锁未被其他 owner 持有
- **THEN** scheduler MUST 通过 `Locker` 使用正数 TTL 获取 owner lock，并在任务退出路径使用独立有界 context 释放
- **WHEN** lock `WaitTimeout` 为零且锁忙
- **THEN** 本轮 MUST 跳过并记录 `lock_busy`
- **WHEN** lock `WaitTimeout` 为正且锁忙
- **THEN** Redis locker MUST 在总等待上限内按 initial/max interval、最大尝试次数和可选 jitter 重试，等待到期或尝试耗尽 MUST 返回 lock busy，parent context 取消或 Redis 错误 MUST 返回 error
- **AND** Redis lock MUST 使用随机 owner token 与 Lua owner 校验执行 unlock/renew，非 owner 操作 MUST 返回 `ErrLockNotOwned`
- **WHEN** job 启用自动续租
- **THEN** scheduler MUST 按有效 interval 和 operation timeout 刷新 TTL，并在任务结束前停止且等待 renew goroutine
- **WHEN** 续租失败
- **THEN** scheduler MUST 记录 `lock_renew_failed`；`ContinueOnFailure=false` 时 MUST 取消任务 context，`true` 时 MUST 允许任务继续，最终任务结果 MUST 保留续租失败语义
- **AND** 多实例副作用任务 MUST 使用正数 TTL，执行时间可能超过 TTL 的任务 MUST 使用续租并协作响应 context；Redis lease MUST NOT 被描述为 exactly-once、fencing 或 goroutine 强杀保证

#### Scenario: Scheduler 关闭

- **WHEN** 调用方首次执行 `Stop(ctx)`
- **THEN** scheduler MUST 先停止新 cron 触发，再取消活动任务及 gate/lock wait context，并等待已触发任务 drain；进入 stopping 后 MUST 拒绝新增任务和重新启动
- **WHEN** Stop 的调用方 context 在 drain 完成前取消或超时
- **THEN** Stop MUST 返回包装后的 context error，实际 drain MUST 继续，后续 Stop MUST 继续等待同一个完成状态而不得提前返回成功
- **AND** 多次 Start 在 running 状态 MUST 幂等，多次 Stop MUST 共享单一 drain，任务不协作响应 context 时 scheduler MUST NOT 声称能够强杀 goroutine

#### Scenario: loading cache 构造、读取与回源

- **WHEN** 服务通过 `NewLoadingCache` 创建 loading cache
- **THEN** 配置 MUST 只包含非空名称、正数 `uint64` 容量、正数固定 TTL 和正数 load timeout，并 MUST 提供 `Loader[V] func(context.Context, string) (V, error)`
- **AND** 容量 MUST 表示最大 item 数，不得表示字节、自定义 cost 或 Ristretto admission 参数；cache key MUST 为 `string`，value MUST 保持泛型
- **AND** 公开 API MUST 只提供 `NewLoadingCache`、`Get`、`Invalidate`、`InvalidateAll`、`Name` 和 `Stats`，MUST NOT 暴露底层 `ttlcache` 配置、主动 `Set`、`CloneFunc`、写入拒绝或关闭语义
- **WHEN** `Get` 命中未过期 item
- **THEN** cache MUST 返回该值并记录一次 hit，且读取 MUST NOT 延长该 item 的固定 TTL
- **WHEN** `Get` 未命中
- **THEN** cache MUST 为该公开调用记录一次 miss，并使用单个 `singleflight.Group` 按 string key 合并同 key 并发回源；loader 成功 MUST 记录 `LoadSuccess` 并同步写入 bounded TTL cache，失败 MUST 记录 `LoadError` 且不得缓存错误结果
- **AND** 内部 double-check 与 invalidation retry MUST NOT 增加业务 hit 或 miss，也 MUST NOT 成为公开统计字段
- **WHEN** 同 key 回源正在执行且任一 caller context 被取消
- **THEN** 该 caller MUST 立即返回其 context error，MUST NOT 因自身取消而终止其他等待者共享的 loader，也 MUST NOT 启动等待 loader 完成的 drain goroutine
- **AND** loader context MUST 保留发起请求的 context values、通过 `context.WithoutCancel` 解除 caller cancellation，并通过 `context.WithTimeout` 受 `LoadTimeout` 限制；loader MUST 协作遵守该 context
- **WHEN** 不同 key 同时 miss
- **THEN** cache MUST 允许不同 key 的 loader 并行执行，不得用全局回源锁串行化 loader 本体

#### Scenario: loading cache 强失效

- **WHEN** loader 开始前
- **THEN** cache MUST 在发布锁下记录当前 cache-wide revision
- **WHEN** loader 成功后准备返回或写入
- **THEN** cache MUST 在同一发布锁下比较当前 revision 与开始 revision；一致时 MUST 先执行 `DeleteExpired` 再以固定默认 TTL 写入，不一致时 MUST 禁止返回该旧值且 MUST 禁止写入
- **WHEN** 调用方执行 `Invalidate(key)`
- **THEN** cache MUST 在发布锁下先递增 cache-wide revision，再删除指定 key，并在方法返回时完成失效
- **WHEN** 调用方执行 `InvalidateAll()`
- **THEN** cache MUST 在发布锁下先递增 cache-wide revision，再删除全部 item，并在方法返回时完成失效
- **AND** 单 key 失效 MAY 抑制其他 key 的在途 loader，cache MUST NOT 为此维护 per-key revision map
- **WHEN** 一个公开 `Get` 的回源结果因 revision 变化被抑制
- **THEN** cache MUST 透明重试一次且不得增加请求 miss；第二次仍被失效时 MUST 返回 `ErrInvalidated`
- **AND** 被失效抑制的旧值在任何情况下 MUST NOT 返回给 caller 或写入 cache

#### Scenario: loading cache TTL、容量、统计与值所有权

- **WHEN** cache 存储或返回 slice、map、pointer 或包含引用字段的 value
- **THEN** common MUST NOT 执行业务 deep clone，消费 feature MUST 在 loader 写入和返回调用方的适当边界复制可变 value
- **WHEN** cache 运行
- **THEN** cache MUST 使用固定 TTL、强制正数容量和命中不 touch 的策略，MUST NOT 使用 `ttlcache.WithLoader`、`ttlcache.NewSuppressedLoader` 或 `ttlcache.Cache.Start`，也 MUST NOT 创建定时清理 goroutine
- **AND** 过期 item MUST 在读取时逻辑失效，并在成功写入前通过 `DeleteExpired` 惰性清理；物理 item 数 MUST 始终受配置容量约束
- **WHEN** 达到最大 item 数并发生 `EvictionReasonCapacityReached`
- **THEN** `Stats.CapacityEvictions` MUST 增加，`Stats.Capacity` MUST 返回配置容量
- **WHEN** item 因 TTL 到期、`Invalidate` 或 `InvalidateAll` 被移除
- **THEN** `Stats.CapacityEvictions` MUST NOT 增加
- **AND** `Stats` MUST 使用请求级手工计数并包含 `Hit`、`Miss`、`LoadSuccess`、`LoadError`、`CapacityEvictions` 和 `Capacity`，MUST NOT 直接导出 `ttlcache.Metrics()`

#### Scenario: Redis key、timezone 与 logger 归属

- **WHEN** feature 需要 refresh session、token version、RBAC 或其他业务 Redis key
- **THEN** feature infrastructure MUST 拥有业务 key schema，并只能复用 `common/runtime/rediskey` 的通用构造规则
- **WHEN** runtime 初始化进程时区
- **THEN** timezone primitive MUST 使用调用方从 `runtime.timezone` 配置派生并显式传入的有效 IANA 时区初始化 `time.Local`，MUST NOT 读取或修改平台 `TZ` 环境变量，也 MUST NOT 直接依赖核心 Config 类型
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

### Requirement: 测试基础设施、Cluster fixture 与隔离

系统 MUST 在 `common/testing` 中提供可复用的 PostgreSQL 与 Redis Cluster 容器 fixture，并使用可重复、可观察且不污染生产 API 或进程全局状态的方式验证共享能力。

#### Scenario: 可重复且可观察的测试

- **WHEN** Go 测试需要真实 PostgreSQL 或 Redis
- **THEN** 测试 MUST 优先使用 `common/testing/containers` 管理依赖生命周期，测试数据 MUST 使用稳定 fixture 或 feature-local builder，避免不可重复的随机输入
- **WHEN** 测试需要注入失败、固定返回、控制顺序或观察后台状态
- **THEN** 测试 MUST 使用消费侧最小接口、局部 fixture、通道或可观察状态，正式代码 MUST NOT 为测试新增全局可变函数、测试 flag、`NewXForTest` 或无运行时职责的 adapter
- **WHEN** 测试验证缓存过期、workerpool drain、scheduler overlap/global gate/lock retry/renew/timeout/shutdown 或后台任务取消
- **THEN** 测试 MUST 使用通道、eventually-style 条件或其他可观察同步机制和明确 deadline，MUST NOT 只依赖固定 `time.Sleep` 判断状态已经变化

#### Scenario: 隔离进程级状态

- **WHEN** 测试必须修改默认 logger、`TZ`、`time.Local` 或包级初始化状态
- **THEN** 测试 MUST 在 package-local helper 中保存状态并通过 cleanup 恢复
- **AND** 环境变量 MUST 使用 `t.Setenv`，相关测试 MUST NOT 并行执行
- **AND** 非测试目标所需的日志捕获 MUST 使用 context logger 或局部 logger 注入

#### Scenario: 真实 Cluster 集成测试

- **WHEN** 模块容器测试 target 通过 `-args -aegiscore.testcontainers` 启用真实依赖测试
- **THEN** Redis Cluster 相关集成测试 MUST 实际连接 Cluster fixture 并执行 Cluster-sensitive Redis 命令
- **AND** Docker daemon、Cluster fixture 启动、slot 初始化或连接失败 MUST 使相关集成测试失败而不是静默跳过
- **AND** `common/testing/containers` 自身的 PostgreSQL 与 Redis 集成测试 MUST 包含在根 `make test-containers` 门禁中

### Requirement: 显式配置来源与加载管线

系统 MUST 将配置文档来源、文档合成、严格解码和服务配置策略表达为显式边界。`common/runtime/config` MUST 只拥有业务中立的配置 schema、document contract、deep merge、raw digest、strict decode、encode、render、redact 和通用校验原语；具体 Nacos 环境、认证、client、failover 和文档读取 MUST 位于 Nacos source adapter。服务 MUST 显式组合 defaults、normalize 和 validate，shared loader MUST NOT 通过服务类型的隐式接口自动发现这些行为。本地配置目录与 Namespace 的发布选择 MUST 留在 Compose 初始化服务和仓库级发布工具边界，runtime Nacos source MUST NOT 读取或解释仓库目录。

#### Scenario: Nacos source adapter 职责边界

- **WHEN** Nacos source adapter 通过 Nacos v3 HTTP API 读取配置文档
- **THEN** adapter MUST 将 server endpoint 解析、认证 token 获取与复用、HTTP 请求/响应处理、按声明顺序 failover 和单个 dataId 读取保持在 `common/runtime/config/nacos` package 内
- **AND** adapter MUST 继续实现业务中立 `DocumentSource` contract，使调用方能通过 `config.LoadSource` 合成多 dataId 文档并生成 `SourceMetadata`
- **AND** adapter MUST NOT 把 Nacos 认证、endpoint、failover、watch、长轮询、SDK fallback、服务发现或仓库目录映射能力加入 `common/runtime/config` 核心包

#### Scenario: Nacos source 失败语义

- **WHEN** Nacos source 依次尝试多个 server 读取同一 dataId
- **THEN** adapter MUST 按声明顺序尝试 server，并在总 timeout 预算内为剩余 server 分配尝试预算，单个故障 server MUST NOT 独占全部预算
- **AND** 全部 server 失败时错误 MUST 聚合每个已尝试 server 的 origin 和原始失败原因
- **AND** Nacos envelope、配置结果、HTTP 状态、JSON 解码、响应体读取和响应体超限错误 MUST 保持可诊断但不包含配置文档内容、password 或 access token
- **AND** 认证启用时 adapter MUST 使用 `Bearer` token 调用配置接口并复用已取得 token；未启用认证时 MUST NOT 发送认证 header

#### Scenario: Nacos source 本地示例

- **WHEN** 示例或测试展示 Nacos source 与配置加载管线的组合
- **THEN** 示例 MUST 使用本地 `httptest.Server` 模拟 Nacos v3 响应，MUST NOT 连接真实 Nacos、依赖外部网络或读取仓库部署目录
- **AND** 示例 MUST 展示多个 dataId 按声明顺序加载、后者覆盖前者、metadata 保留 provider、service、namespace、group、dataId 顺序和 digest

### Requirement: 业务中立本地限流与 HTTP 映射

系统 MUST 提供由调用方决定 key 与策略的本地 token bucket primitive，并在共享 contract/response 中提供稳定限流错误与 `429 Too Many Requests` 映射。共享实现 MAY 提供分片存储、后台清理和显式关闭，但 MUST NOT 内置 user-service 路由、身份 schema 或服务私有阈值。

#### Scenario: 限流错误响应

- **WHEN** HTTP middleware 或 handler 返回限流应用错误
- **THEN** `common/http/response` MUST 渲染 `429 Too Many Requests`
- **AND** 响应 envelope MUST 为 `success=false`、稳定限流 code 和公开限流 message

#### Scenario: 未知错误不伪装为限流

- **WHEN** 系统返回 nil、未知错误或内部错误
- **THEN** `common/http/response` MUST 保持现有内部错误归一化语义
- **AND** 系统 MUST NOT 将非限流错误映射为 `429 Too Many Requests`

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
- **THEN** Gin engine MUST NOT 信任任何代理
- **AND** `c.ClientIP()` MUST 忽略 `X-Forwarded-For` 和 `X-Real-IP`，只返回请求 TCP peer 地址

#### Scenario: 显式信任代理 CIDR

- **WHEN** `server.http.trusted_proxies` 包含请求 TCP peer 所属的 IP 或 CIDR
- **THEN** Gin engine MUST 使用 Gin trusted proxy 机制解析 `X-Forwarded-For` 或 `X-Real-IP`
- **AND** `c.ClientIP()` MUST 返回可信代理链解析后的客户端地址

#### Scenario: 拒绝旧配置位置

- **WHEN** 配置文档包含 `http.trusted_proxies` 或其他未声明 trusted proxy 键
- **THEN** 严格配置解码 MUST 失败并报告完整配置路径
- **AND** 系统 MUST NOT 通过 normalize、alias 或 fallback 接受该旧配置

### Requirement: 通用事务生命周期与直接调用禁令

`common/runtime/datastore` MUST 提供业务中立的泛型事务生命周期 helper，用于 infrastructure 代码创建、提交和回滚显式事务边界。新增或改造的业务 infrastructure MUST 通过该 helper 终结事务，MUST NOT 直接调用底层 commit/rollback 或维护重复 helper。共享实现 MUST 只依赖标准库和最小事务接口。

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

### Requirement: localcache 泛型结果与同步容量统计

系统 MUST 让 `LoadingCache[V]` 对其声明支持的全部泛型 value 保持一致的成功加载语义，并 MUST 在公开读取返回前同步完成由本次发布引起的容量驱逐统计；实现不得为无关删除注册异步 eviction callback。

#### Scenario: 接口类型 loader 返回 nil

- **WHEN** `V` 是接口类型且 loader 成功返回 nil value
- **THEN** `Get` MUST 返回 nil 和 nil error，不得因 singleflight 的 `any` 结果缺少动态类型而 panic
- **AND** 后续同 key `Get` MUST 命中已缓存的 nil value，不得重复回源

#### Scenario: 新值触发容量驱逐

- **WHEN** cache 已达到配置容量且成功发布一个此前不存在的新 key
- **THEN** cache MUST 驱逐一个 item，并在本次 `Get` 返回前同步增加 `Stats.CapacityEvictions`

#### Scenario: 非容量删除不产生异步驱逐统计

- **WHEN** item 因 TTL 清理、`Invalidate` 或 `InvalidateAll` 被移除
- **THEN** `Stats.CapacityEvictions` MUST 保持不变
- **AND** localcache MUST NOT 依赖为每个删除 item 启动 goroutine 的 eviction callback 来维护该统计

### Requirement: Scheduler 配置快照与严格校验

系统 MUST 在注册边界持有独立、归一化且通过校验的 job 配置快照，并 MUST 将零值默认与负数错误明确区分；调用方持有的可变配置不得在注册期间被修改，也不得在注册后改变已注册任务的执行策略。

#### Scenario: 注册嵌套锁与续租策略

- **WHEN** 调用方通过 `Add` 注册包含 `LockPolicy` 和 `RenewPolicy` 指针的 job
- **THEN** scheduler MUST 在填充默认值前复制全部嵌套策略，并只让 cron closure 与执行 pipeline 持有归一化副本
- **AND** `Add` MUST NOT 修改调用方传入的 job、lock 或 renew 对象，调用方在 `Add` 返回后修改这些对象 MUST NOT 改变已注册任务配置或造成共享读写竞态

#### Scenario: duration 使用零值默认

- **WHEN** default lock TTL、job lock TTL、renew interval、renew timeout 或 Redis retry interval 使用文档允许的零值
- **THEN** scheduler MUST 按对应配置层级填充既有默认值，并继续执行既有范围关系校验

#### Scenario: duration 使用负数

- **WHEN** default lock TTL、job lock TTL、renew interval、renew timeout 或 Redis retry interval 为负数
- **THEN** scheduler 或 Redis locker 构造与注册边界 MUST 返回可通过 `errors.Is(err, ErrInvalidLock)` 识别的错误
- **AND** 系统 MUST NOT 把负数静默转换为默认值

### Requirement: 出站 HTTP 请求配置快照与所有权

系统 MUST 在每次发送边界持有独立、归一化且通过校验的 request-level 配置快照，并 MUST 明确只做浅层复制；共享 helper 不得为并发便利隐式 deep-copy 业务 body、clone 注入 client 或改变调用方拥有对象的生命周期。

#### Scenario: 创建逐次发送快照

- **WHEN** 调用方通过 `Send` 或 `SendContext` 发送包含 query、form 或 header maps 的 `SendRequest`
- **THEN** helper MUST 在 Resty 请求构造前复制这些 maps，并使用裁剪首尾空白后的 URL、method 和 proxy URL 形成该次发送快照
- **AND** helper MUST NOT 修改调用方持有的 `SendRequest` 或其中的 maps

#### Scenario: 顺序复用请求配置

- **WHEN** 前一次发送已经返回且调用方修改同一个 `SendRequest` 后再次发送
- **THEN** 后一次发送 MUST 使用后一次调用开始时的配置，不得复用前一次 Resty request 的 query、form 或 header 状态

#### Scenario: body 与注入 client 所有权

- **WHEN** `JSONData` 包含 map、slice、pointer、`io.Reader` 或其他引用值，或者调用方提供 `RestyClient`
- **THEN** helper MUST 将这些值视为调用方拥有的浅层引用，不得隐式 deep-copy、缓存、重放或 clone
- **AND** 调用方 MUST 在发送返回前保持 body 与注入 client 配置稳定，并 MUST NOT 并发修改或并发发送同一个 `SendRequest`

### Requirement: metrics primitive 文件职责与业务中立边界

`common/runtime/observability/metrics` MUST 作为跨服务共享、业务中立的 runtime primitive 维护 provider、registry、context-aware gather、runtime collector 与 adapter 职责。实现可以在同 package 内按职责拆分文件，但 MUST 保持原 package、调用方 import path、导出 API、指标名称、label、bucket 和采集语义兼容。

#### Scenario: 同 package 文件职责可独立审查

- **WHEN** metrics package 维护 Provider/registry、`ContextCollector`/gather wrapper、runtime collector 和 SQL、Redis、scheduler、workerpool、localcache、component status collector adapter
- **THEN** 这些职责 MUST 通过清晰的同 package 文件组织表达
- **AND** 文件拆分 MUST NOT 创建 `metrics` 子包或改变调用方 import path

#### Scenario: 导出 API 与指标契约保持兼容

- **WHEN** 调用方升级到整理后的 metrics package
- **THEN** 既有导出 constructor、method、interface、error 和 adapter 类型 MUST 保持可用
- **AND** 既有 Prometheus 指标名称、label name、label value 枚举、bucket 和采集语义 MUST 保持兼容
- **AND** 系统 MUST NOT 为本次重构新增业务指标或删除既有稳定指标

#### Scenario: common 不承载服务业务语义

- **WHEN** metrics package 增加文档、示例、collector 或 adapter
- **THEN** 内容 MUST 保持业务中立，MUST NOT 导入 user-service feature 包、DTO、业务状态、业务 key schema、policy loader、route diff 或服务私有配置
- **AND** 业务指标 MUST 留在所属 feature 或消费服务边界，只通过共享 provider 注册低基数 collector

#### Scenario: 测试与示例不污染生产 API

- **WHEN** 为 metrics package 增加测试或 executable examples
- **THEN** 测试 MUST 基于现有公开 API、本地 registry、局部 fixture 或内存对象验证行为
- **AND** 正式代码 MUST NOT 仅为测试便利新增全局可变函数、测试 flag、`NewXForTest` 或无运行时职责的 adapter

### Requirement: Tracing primitive 公开 API 与实现边界

`common/runtime/observability/tracing` MUST 作为业务中立的共享 runtime primitive 维护清晰的公开 API 和包内职责边界。公开入口 MUST 表达 tracing 能力、构造、传播和关闭职责；仅供 lifecycle adapter 使用的启动状态机 MUST 留在包内，除非存在经规格确认的跨服务调用需求。

#### Scenario: 公开入口表达真实运行时职责
- **WHEN** 调用方需要创建或消费 tracing provider
- **THEN** 公开 API MUST 优先使用 `Options`、`Provider`、普通 constructor、Fx tracing provider constructor、`Tracer`、`OTelTracerProvider`、`TextMapPropagator` 和 `Shutdown`
- **AND** 公开名称 MUST 表达 tracing 能力语义，MUST NOT 以缺少能力语义的通用 Fx 名称作为主要入口

#### Scenario: Start 暴露面收窄
- **WHEN** 启动状态机只被 tracing 包内 lifecycle 使用
- **THEN** 该启动函数 MUST 保持未导出或包内可见
- **AND** 包外调用方 MUST 使用普通 constructor 表达立即启动所有权，或使用 Fx adapter 表达延迟启动所有权
- **AND** 系统 MUST NOT 保留绕过所有权边界的兼容双生命周期路径

#### Scenario: 包内文件职责聚焦
- **WHEN** 维护 tracing package 实现
- **THEN** provider facade、resource 构造、OTLP exporter、dynamic tracer、lifecycle 状态机、Fx adapter、包文档和 examples MUST 分别放置在聚焦文件中
- **AND** Fx adapter MUST 只负责配置映射和 hook 注册，MUST NOT 承载 resource、exporter 或 dynamic tracer 细节
- **AND** tracing primitive MUST NOT 导入 user-service feature、router、bootstrap、服务私有配置包、部署资产或业务 DTO

#### Scenario: 文档和示例不连接真实 endpoint
- **WHEN** tracing package 提供 `doc.go` 和 executable examples
- **THEN** 示例 MUST 覆盖 disabled provider、span 创建、propagator inject/extract 和显式 `Shutdown`
- **AND** 示例 MUST NOT 连接真实 OTLP endpoint、修改 OpenTelemetry global provider 或依赖外部网络服务

### Requirement: Runtime config pipeline 文档与布局

`common/runtime/config` MUST 作为跨服务共享、业务中立的配置原语维护清晰的 source、merge、decode、defaults、validation、encode、redact 和 render 导航。实现 MAY 在同 package 内按职责拆分文件和测试，但 MUST 保持原 package、`Config` 类型、导出函数、默认值、字段路径、错误聚合、strict decode、raw digest、effective encode、redact 和 render 语义兼容。

#### Scenario: 固定配置加载管线可从包文档导航

- **WHEN** 调用方阅读 `common/runtime/config` 包文档或 executable example
- **THEN** 文档 MUST 展示通过 `DocumentSource` 调用 `LoadSource` 取得 raw merged settings 和 source metadata
- **AND** 文档 MUST 展示 `LoadSource` 内部按文档顺序执行 YAML deep merge，并在 defaults 和 normalize 前基于 raw settings 计算 `SourceMetadata.Digest`
- **AND** 文档 MUST 展示调用方随后显式执行 `DecodeStrict`、`Validate`、`EncodeSettings`、可选 `RedactSettings` 和 `RenderYAML`

#### Scenario: 服务扩展配置显式组合

- **WHEN** 服务在共享 `Config` 之外增加服务私有配置字段
- **THEN** 服务 MUST 通过 `DecodeOptions` 显式提供完整 defaults、可选 normalize 和最终 validate
- **AND** `DecodeStrict` MUST 在 normalize 和 validate 前拒绝未知配置键并报告完整叶子路径
- **AND** shared loader MUST NOT 通过目标类型自动发现 `ConfigDefaults()`、`ApplyDefaults()` 或其他服务 hook
- **AND** user-service 的 auth、RBAC、Ent、rate limit、具名资源必需名称和业务校验 MUST 留在 `user-service/internal/config` 或所属 feature 边界

#### Scenario: validation 文件职责清晰且 API 兼容

- **WHEN** config package 维护 validation error/aggregate、runtime、server、log 和 observability 校验
- **THEN** 这些职责 MUST 通过清晰的同 package 文件组织表达
- **AND** 文件拆分 MUST NOT 创建 config 子包、改变调用方 import path、移除导出 helper 或改变错误文本和字段路径

#### Scenario: 测试与示例覆盖 pipeline 而不污染生产 API

- **WHEN** 为 config package 增加或重排测试和 executable examples
- **THEN** 测试 MUST 覆盖 defaults、strict decode、server validation 和 observability validation 的既有行为
- **AND** executable example MUST 使用本地内存 `DocumentSource` 或局部 YAML fixture，不访问公网、Nacos、PostgreSQL、Redis、user-service feature 或部署资产
- **AND** 正式代码 MUST NOT 仅为测试便利新增全局可变函数、测试 flag、`NewXForTest` 或无运行时职责的 adapter

### Requirement: 通用 Redis Pub/Sub 单 channel 订阅原语

`common/runtime/redispubsub` MUST 提供业务中立的 Redis classic Pub/Sub 单 channel subscriber，负责显式订阅确认、阻塞接收、有界消息缓冲、context 背压、带抖动的有界指数退避重连、单向生命周期和结构化状态。该 primitive MUST 保持 Redis Pub/Sub 的 at-most-once 通知语义，MUST NOT 承担 Publish 业务封装、消息 envelope、revision、outbox、幂等、数据库校准、Casbin reload、缓存失效、模式订阅、sharded Pub/Sub、Redis Streams 或可靠投递。

#### Scenario: 严格 options 与无副作用构造

- **WHEN** 调用方构造 subscriber
- **THEN** `Options.Name` 与 `Options.Channel` MUST 为非空非空白字符串，`BufferSize`、`SubscribeTimeout`、`BackoffInitial` 与 `BackoffMax` MUST 为正数，且 `BackoffMax` MUST 大于或等于 `BackoffInitial`
- **AND** 任一 option 非法或必需依赖缺失时 `NewSubscriber` MUST 返回 error，primitive MUST NOT 补默认值、静默归一化非法值或进入部分可用状态
- **AND** constructor MUST NOT 创建 goroutine、调用 Redis、建立订阅或关闭共享 client，初始状态 MUST 为 `created` 且 `Running` MUST 为 false

#### Scenario: 单向启动状态

- **WHEN** created subscriber 首次调用 `Start()`
- **THEN** subscriber MUST 原子进入 `starting`、创建唯一 root lifecycle 并异步建立订阅
- **WHEN** subscriber 已处于 starting、connected 或 reconnecting 状态并再次调用 `Start()`
- **THEN** `Start()` MUST 幂等返回 nil 且 MUST NOT 创建额外 goroutine或订阅
- **WHEN** subscriber 已进入 stopping 或 stopped 状态并调用 `Start()`
- **THEN** `Start()` MUST 返回稳定的 `ErrStopped`，MUST NOT 恢复或创建新的生命周期

#### Scenario: 显式确认与成功接收

- **WHEN** subscriber 为 `Options.Channel` 创建一次 classic Pub/Sub attempt
- **THEN** subscriber MUST 使用不超过 `SubscribeTimeout` 的 context 阻塞等待 go-redis 订阅确认，确认前 MUST NOT 将状态报告为 connected
- **WHEN** 收到合法订阅确认
- **THEN** subscriber MUST 将状态置为 `connected`、记录 `LastConnectedAt`、将当前 `ErrorCategory` 清为 `none` 并把下一次退避重置为 `BackoffInitial`
- **WHEN** 已确认订阅收到合法 go-redis message
- **THEN** subscriber MUST 将 `Channel`、`Pattern` 和 `Payload` 投影为公开 `Message` 并写入唯一的 `Messages()` channel

#### Scenario: 故障分类与有界重连

- **WHEN** 等待订阅确认的 Receive 失败或超时
- **THEN** subscriber MUST 记录 `subscribe_failed`、`LastFailureAt` 和重连次数，关闭当前 attempt 并进入 `reconnecting`
- **WHEN** 已确认订阅的 Receive 返回非取消错误
- **THEN** subscriber MUST 记录 `receive_failed`、`LastFailureAt` 和重连次数，关闭当前 attempt 并进入 `reconnecting`
- **WHEN** 确认或接收阶段收到不受支持的协议类型
- **THEN** subscriber MUST 记录 `protocol_failed`、`LastFailureAt` 和重连次数，关闭当前 attempt 并进入 `reconnecting`
- **AND** 重连 MUST 使用从 `BackoffInitial` 指数增长且不超过 `BackoffMax` 的基准延迟，每次实际等待 MUST 位于对应基准延迟的 `[1/2, 1]` 闭区间，且 MUST NOT 设置永久终止次数

#### Scenario: 有界缓冲与 context 背压

- **WHEN** `Messages()` 缓冲尚有容量
- **THEN** subscriber MUST 按接收顺序交付消息且缓冲容量 MUST 等于显式 `BufferSize`
- **WHEN** 缓冲已满且 subscriber 已从 Redis 读取下一条消息
- **THEN** 接收 goroutine MUST 阻塞等待消费者腾出容量，MUST NOT 主动丢弃该消息、扩展为无界缓冲或启动绕过背压的额外交付 goroutine
- **WHEN** subscriber 在消息交付阻塞期间被停止
- **THEN** root context 取消 MUST 解除阻塞并允许 lifecycle drain 完成

#### Scenario: 单 owner 停止与资源所有权

- **WHEN** subscriber 首次调用 `Stop(ctx)`，包括尚未 Start 的 created 状态
- **THEN** subscriber MUST 单向进入 `stopping`、取消 root context、触发活动 PubSub 关闭并等待唯一完成状态
- **AND** 订阅确认、Receive、重连 backoff 或缓冲交付中的阻塞 MUST 能被停止解除，取消后 MUST NOT 创建新 attempt
- **WHEN** Stop 与 attempt 失败路径并发关闭同一 PubSub
- **THEN** 每个 attempt MUST 通过单 owner 与恰好一个 close-once gate 保证 `PubSub.Close()` 最多执行一次，subscriber MUST NOT 关闭共享 Redis client
- **WHEN** lifecycle 完全 drain
- **THEN** subscriber MUST 将状态置为 `stopped`、将 `Running` 置为 false，并且只关闭一次 `Messages()` channel

#### Scenario: Stop 超时与重复等待

- **WHEN** `Stop(ctx)` 的调用方 context 在后台 lifecycle drain 完成前结束
- **THEN** 本次 Stop MUST 返回 context error，但后台 drain MUST 继续，完成状态、消息 channel 和 stopping 状态 MUST NOT 被替换或重置
- **WHEN** 调用方随后再次调用 `Stop(ctx)`
- **THEN** 后续调用 MUST 等待同一个完成状态，并在该 drain 已完成时幂等返回 nil

#### Scenario: Redis Cluster classic Pub/Sub 边界

- **WHEN** subscriber 使用仓库 Redis Cluster fixture 订阅 channel 且另一个 client 通过 classic `Publish` 发送消息
- **THEN** subscriber MUST 能完成订阅确认并接收对应消息
- **AND** 该验证 MUST NOT 使用 `PSubscribe`、`SSubscribe`、Redis Streams 或宣称断线、重连和背压期间的消息可恢复

### Requirement: 业务中立的 managed HTTP server 生命周期

系统 MUST 在 `common/runtime/httpserver` 提供不依赖 Gin、Fx、服务私有配置或业务日志字段的 managed `net/http` server。公开 API MUST 包含严格的 `Options`、`Managed`、`New`、`Start`、`Stop`、`HTTPServer` 以及可匹配的 `ErrInvalidOptions`、`ErrAlreadyStarted` 和 `ErrStopped`；启用策略、地址来源、默认 timeout、日志和进程 shutdown 策略 MUST 由调用服务拥有。

#### Scenario: 构造严格校验且无运行时副作用

- **WHEN** 调用方执行 `New(options)`
- **THEN** `Name`、`Addr` 和 `Handler` MUST 非空，`ShutdownTimeout` MUST 大于零，read/write/idle timeout MUST 不小于零
- **AND** 任一字段非法时错误 MUST 包装 `ErrInvalidOptions` 并包含 server name、addr 与具体字段或操作
- **AND** `Options` MUST NOT 提供默认值，`New` MUST NOT 监听端口或创建 goroutine
- **WHEN** 构造成功
- **THEN** `HTTPServer()` MUST 返回 addr 与 timeout 均匹配 options 的 `*http.Server`，其 `Handler` MUST 是内部 drain tracker 而不是原始 handler

#### Scenario: 同步启动与不可重启状态机

- **WHEN** created 状态的实例执行 `Start(ctx)`
- **THEN** 系统 MUST 使用 `net.ListenConfig.Listen` 同步绑定 `Addr`，成功绑定并保存 listener 后才启动异步 `Serve`
- **WHEN** 地址被占用或监听失败
- **THEN** `Start` MUST 直接返回包含 server name、addr 与 listen 操作的错误，MUST NOT 留下 listener 或 goroutine，也 MUST NOT 调用 `OnServeError`
- **WHEN** `Start` 成功返回
- **THEN** 地址 MUST 已经可以接受连接，状态 MUST 为 running
- **WHEN** running 或 failed 状态再次调用 `Start`
- **THEN** 系统 MUST 返回包装 `ErrAlreadyStarted` 的稳定错误
- **WHEN** stopping 或 stopped 状态调用 `Start`
- **THEN** 系统 MUST 返回包装 `ErrStopped` 的稳定错误且 MUST NOT 重启

#### Scenario: Serve 退出分类与故障回调

- **WHEN** `Serve` 因 `http.ErrServerClosed`、`net.ErrClosed` 或停止期间的 context cancellation 返回
- **THEN** 系统 MUST 将其视为正常退出且 MUST NOT 调用 `OnServeError`
- **WHEN** `Serve` 返回其他错误
- **THEN** 状态 MUST 从 running 进入 failed，错误 MUST 被保存，`OnServeError` MUST 恰好调用一次
- **AND** 调用 `OnServeError` 时 MUST NOT 持有内部锁，callback MUST 能安全触发调用方 shutdown

#### Scenario: 单一后台 cleanup 与调用方独立等待

- **WHEN** created 状态首次调用 `Stop(ctx)`
- **THEN** 系统 MUST 直接进入 stopped 并返回 nil
- **WHEN** running 或 failed 状态首次调用 `Stop(ctx)`
- **THEN** 系统 MUST 只启动一个使用 `context.Background()` 和 `ShutdownTimeout` 的后台 cleanup，并切换为 stopping
- **AND** cleanup MUST 调用 `http.Server.Shutdown`，无论其是否识别到 listener 都 MUST 显式关闭 Managed 持有的 listener
- **WHEN** `Shutdown` 失败
- **THEN** cleanup MUST 调用 `http.Server.Close` 强制关闭活跃连接
- **AND** cleanup MUST 等待 drain tracker 与 `Serve` goroutine，使用 `errors.Join` 保存最终错误，切换为 stopped 并只关闭一次完成 channel
- **WHEN** 多个 goroutine 并发或重复调用 `Stop`
- **THEN** 所有调用 MUST 共享同一 cleanup 与最终结果，MUST NOT 重复关闭、panic 或永久阻塞
- **WHEN** 某次 `Stop(ctx)` 的调用方 context 在 cleanup 完成前取消或超时
- **THEN** 本次调用 MUST 返回 `ctx.Err()`，后台 cleanup MUST 继续，后续 `Stop` MUST 能继续等待并取得同一个最终结果

#### Scenario: 已进入 handler 的请求 drain

- **WHEN** 请求进入真实 handler
- **THEN** drain tracker MUST 在调用前增加 active，并通过 defer 在 handler 返回或 panic unwind 时减少 active，active 归零时 MUST 唤醒等待者
- **WHEN** `Wait(ctx)` 等待期间 context 取消或超时
- **THEN** wait MUST 被唤醒并返回对应 context error，MUST NOT 泄漏永久等待 goroutine
- **WHEN** 慢 handler 在 shutdown timeout 内完成
- **THEN** `Stop` MUST 等待其完成并优雅退出
- **WHEN** handler 超过 shutdown timeout
- **THEN** 系统 MUST 强制关闭连接；handler 仍忽略取消时 drain timeout MUST 保留到最终错误
- **AND** tracker MUST 只代表已进入 handler 的请求，MUST NOT 声称管理 hijacked connection、WebSocket、进程外负载均衡 drain 或强杀 goroutine

### Requirement: Runtime metrics 保持业务中立

`common/runtime/observability/metrics` MUST 只提供跨服务通用的 metrics Provider、registry 注册、稳定 label 常量、HTTP metrics 支撑和业务中立 collector。该包 MUST NOT 定义、注册或导出 Casbin policy reload、permission、role、RBAC、user-service 或其他单一服务业务指标的接口、空实现、constructor、collector 或指标名称。

#### Scenario: common metrics 不拥有 Casbin reload 指标

- **WHEN** 系统编译或审查 `common/runtime/observability/metrics`
- **THEN** 该包 MUST NOT 包含 `ReloadMetrics`、`NopReloadMetrics`、`NewCasbinPolicyReloadMetrics` 或 `aegiscore_casbin_*` 指标定义
- **AND** 该包 MUST NOT 通过业务命名的 no-op recorder 为 user-service permission/RBAC 运行时提供依赖

#### Scenario: 通用 component status collector 保留

- **WHEN** 服务需要导出业务中立运行时组件的 running 和 last error 状态
- **THEN** 系统 MUST 继续复用 `common/runtime/observability/metrics` 的 component status collector
- **AND** collector MUST 只使用稳定 resource label 与只读状态接口，不得内置 Casbin、permission、role 或 user-service 组件名

#### Scenario: 业务指标归属调用方

- **WHEN** 消费服务或 feature 需要记录带业务语义的 metrics
- **THEN** 业务指标接口、空实现、指标名称和 collector MUST 由 owning service、feature 或其 observability adapter 拥有
- **AND** owning 代码 MAY 复用通用 metrics Provider 注册 collector，但 MUST NOT 把业务 recorder 下沉到 `common`

### Requirement: 共享事务 helper 不得注入隐藏事务生命周期上限

`common/runtime/datastore` 的共享事务 helper MUST 使用调用方拥有的 context 策略表达事务生命周期。`BeginTransaction` MUST 保留原始 context value，并 MUST 隔离原始取消信号以允许请求取消后执行 rollback cleanup；当原始 context 提供 deadline 时，事务 lifecycle context MUST 继承该 deadline；当原始 context 没有 deadline 时，helper MUST NOT 为传入 `Ent Tx`、`database/sql.BeginTx` 或其他 transaction starter 的 context 注入固定 timeout、cleanup budget 或隐藏 deadline。

#### Scenario: 无原始 deadline 的事务不会获得默认 5 秒 deadline
- **WHEN** 调用方以没有 deadline 的 context 调用 `datastore.BeginTransaction`
- **THEN** transaction starter 接收的 lifecycle context MUST 保留原始 context value 且不继承原始取消信号
- **AND** lifecycle context MUST NOT 包含由 datastore helper 注入的 5 秒 deadline 或其他固定 deadline

#### Scenario: 显式原始 deadline 继续约束事务生命周期
- **WHEN** 调用方以带 deadline 的 context 调用 `datastore.BeginTransaction`
- **THEN** transaction starter 接收的 lifecycle context MUST 继承同一个 deadline
- **AND** helper MUST NOT 放宽、覆盖或替换调用方显式 deadline

#### Scenario: 超过清理预算的无 deadline 事务仍可提交
- **WHEN** 原始 context 没有 deadline，且事务从 begin 到 commit 的耗时超过 `DefaultTransactionCleanupTimeout`
- **THEN** datastore helper MUST NOT 因该常量到期取消事务 lifecycle context
- **AND** 事务 MUST 能在 transaction implementation 和数据库策略允许时成功提交

#### Scenario: 提交前原始 context 取消时拒绝提交
- **WHEN** 事务已经开始，且原始 request context 在提交前被取消或超时
- **THEN** `Finish.Commit(ctx)` MUST 返回原始 context 错误并拒绝调用底层 commit
- **AND** 调用方 MUST 能通过 `Fail` 或 `RollbackUnlessCommitted` 回滚未提交事务

#### Scenario: rollback cleanup 不依赖 BeginTx 固定 timeout
- **WHEN** 事务需要在业务错误、提交拒绝或 defer 兜底中回滚
- **THEN** helper MUST NOT 通过传给 `BeginTx` 的固定 timeout 表达 rollback cleanup budget
- **AND** 长事务和泄漏保护 MUST 由调用方 context deadline、数据库 timeout 或拥有者显式生命周期策略表达

### Requirement: workerpool 消费端日志字段安全

系统 MUST 要求 `workerpool.Task.Fields` 的生产者只传入低敏定位字段。任何消费端提交的后台任务字段 MUST NOT 包含密码、token、完整 Redis key、Redis key prefix、可拼装 Redis key 的材料、SQL、原始凭据或其他敏感值。workerpool MAY 在 error 和 panic 路径原样记录 `Task.Fields`，字段安全责任 MUST 由消费端在提交任务前满足。

#### Scenario: 任务失败日志只记录低敏字段
- **WHEN** 消费端提交 workerpool 任务且任务返回 error
- **THEN** workerpool MAY 记录任务池名称、任务名称、调用方提供的低敏 `Task.Fields` 和 error
- **AND** 消费端提供的 `Task.Fields` MUST NOT 包含完整 Redis key、Redis key prefix、可拼装 Redis key 的材料、token、密码、SQL 或原始凭据

#### Scenario: 任务 panic 日志只记录低敏字段
- **WHEN** 消费端提交 workerpool 任务且任务发生 panic
- **THEN** workerpool MAY 记录任务池名称、任务名称、调用方提供的低敏 `Task.Fields`、panic 和 stacktrace
- **AND** 消费端提供的 `Task.Fields` MUST NOT 包含完整 Redis key、Redis key prefix、可拼装 Redis key 的材料、token、密码、SQL 或原始凭据

#### Scenario: common 不承担业务字段脱敏
- **WHEN** feature 需要在后台任务失败或 panic 日志中关联单次业务操作
- **THEN** feature MUST 在自身边界生成低敏字段或不可逆 opaque 标识后再写入 `Task.Fields`
- **AND** `common/runtime/workerpool` MUST NOT 为 user-service auth、RBAC、refresh session、Redis key schema 或其他服务私有字段提供业务专用过滤、兼容字段或脱敏分支

