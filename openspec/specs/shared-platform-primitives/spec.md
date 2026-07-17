## Purpose

定义 `common/` 提供的跨服务共享契约、HTTP helper、安全原语、runtime primitive、测试基础设施和校验能力，保证服务间基础行为一致且业务边界清晰。
## Requirements
### Requirement: 跨服务错误与响应契约

系统 MUST 在 `common/contract` 中维护业务中立的应用错误、响应 envelope 和分页契约，并 MUST 由 `common/http/response` 统一完成 HTTP 渲染。应用错误 MUST 使用低基数 `Kind`、稳定 `Reason`、响应 `Code`、公开 `Message` 和可选内部 `Cause` 表达语义，MUST NOT 保存或接收 HTTP status；HTTP status MUST 只根据 `Kind` 推导。

#### Scenario: 返回统一响应

- **WHEN** 服务返回成功、分页或错误响应
- **THEN** 系统 MUST 使用共享 envelope 表达 `success`、`code`、`message`、`data`、`pagination` 或结构化错误详情
- **AND** 新服务 MUST 优先复用 `common/contract` 和 `common/http/response`，不得定义不兼容的 envelope

#### Scenario: 应用错误转换与标准错误链

- **WHEN** 系统创建、包装或通过 `FromError` 归一化错误
- **THEN** wrapped application error MUST 保留原始 `Kind`、`Reason`、`Code` 和公开 `Message`
- **AND** nil 或未知错误 MUST 归一化为使用非敏感公开消息的内部错误，原始错误只能作为内部 `Cause` 保留
- **AND** `errors.As` MUST 能解析应用错误，`errors.Is` MUST 按稳定 `Kind`、`Reason` 或内部 `Cause` 语义匹配

#### Scenario: HTTP 层推导错误状态码

- **WHEN** `common/http/response` 写入应用错误响应
- **THEN** 请求格式或字段校验错误 MUST 渲染为 `400 Bad Request`
- **AND** 未认证或 token 无效、过期、撤销 MUST 渲染为 `401 Unauthorized`
- **AND** 权限不足、未找到、冲突和服务不可用 MUST 分别渲染为 `403 Forbidden`、`404 Not Found`、`409 Conflict` 和 `503 Service Unavailable`
- **AND** nil、未知或内部错误 MUST 渲染为 `500 Internal Server Error`

#### Scenario: 校验错误保持结构化

- **WHEN** 请求绑定或字段校验失败
- **THEN** `common/validation` 和 `common/http/binding` MUST 生成或传播语义应用错误
- **AND** response helper MUST 返回 `success=false`、`CodeValidationFailed`、公开 message 和结构化字段错误明细

#### Scenario: 稳定跨服务错误码

- **WHEN** 服务表达凭据有效但账号要求强制修改密码
- **THEN** 系统 MUST 使用数值为 `20006` 的 `CodePasswordChangeRequired`
- **AND** `common` MUST 只拥有该业务中立错误码，不得承载 user-service 的状态判断、token 签发或登录编排

#### Scenario: 业务错误归属功能边界

- **WHEN** auth、permission、role 或 `internal/shared/identity` 定义稳定业务错误
- **THEN** owning domain MUST 为错误提供共享契约要求的 `Kind`、`Reason`、`Code` 和公开 `Message`，并保持 `errors.Is` 匹配语义
- **AND** `common/http/response.Fail` MUST 能直接渲染该错误
- **AND** 系统 MUST NOT 在 `common`、跨 feature 全局包或 HTTP transport 中维护重复的业务错误映射表

#### Scenario: 共享契约发生变化

- **WHEN** 共享错误分类、错误码、响应 envelope 或分页结构需要改变
- **THEN** change MUST 更新相关主规格或 delta spec，并评估所有共享契约消费者的影响

### Requirement: HTTP helper 与校验基础

系统 MUST 在 `common/http` 和 `common/validation` 中提供业务中立的绑定、字段校验、响应、认证授权 middleware、CORS、metrics、logging、recovery、OpenAPI 和 pprof 能力，并 MUST 保持其外部行为一致。

#### Scenario: HTTP 请求进入服务

- **WHEN** HTTP 请求被 Gin 路由处理
- **THEN** 服务 MUST 能复用共享 middleware 完成认证上下文、授权检查、日志字段、metrics、panic recovery 和 span error 记录

#### Scenario: 绑定和字段校验

- **WHEN** HTTP handler 绑定请求或校验字段
- **THEN** 系统 MUST 通过共享 binding、validation 和 response helper 返回一致的字段名、公开消息和错误结构
- **AND** validation tag 的字段名解析顺序 MUST 保持稳定

#### Scenario: 成功响应 helper

- **WHEN** 调用方使用 `Created` 或 `NoContent` 写入响应
- **THEN** `Created` MUST 返回包含统一成功 envelope 和调用方 `data` 的 `201 Created`
- **AND** `NoContent` MUST 返回 body 为空的 `204 No Content`

#### Scenario: 默认 CORS 请求

- **WHEN** 请求经过 `CORS()` middleware
- **THEN** 响应 MUST 默认包含 `Access-Control-Allow-Origin=*`、`Access-Control-Allow-Methods=GET,POST,PUT,PATCH,DELETE,OPTIONS` 和 `Access-Control-Allow-Headers=Authorization,Content-Type`
- **AND** 默认配置 MUST NOT 启用 credentials、max age、exposed headers 或 `Vary: Origin`
- **AND** middleware MUST 复制默认或调用方传入的 slice，调用方后续修改 MUST NOT 改变已创建 middleware 的行为

#### Scenario: CORS 预检与普通请求

- **WHEN** `OPTIONS` 预检请求经过共享 CORS middleware
- **THEN** middleware MUST 返回带默认 CORS header 的 `204 No Content` 并停止调用后续 handler
- **WHEN** 非 `OPTIONS` 请求经过该 middleware
- **THEN** middleware MUST 调用后续 handler，并保持其 status 和 body 可见

#### Scenario: OpenAPI 生成与渲染

- **WHEN** 服务生成、转换或嵌入 OpenAPI 文档
- **THEN** 系统 MUST 复用 `common/http/openapi` 的规范化、序列化和 Go embed 渲染能力
- **AND** API server、认证方案、扫描范围、健康路径和输出目录等服务元数据 MUST 留在服务脚本或薄 wrapper

#### Scenario: 服务特定授权行为

- **WHEN** 授权依赖 user-service 的 subject schema、角色、权限目录、route diff 或超级管理员基线
- **THEN** 行为 MUST 留在 user-service permission 或 shared 边界，不得进入通用 HTTP middleware 或 `common/security/casbin`

### Requirement: 共享安全原语

系统 MUST 在 `common/security` 中提供业务中立的 JWT 验证、Bearer token 处理、Casbin 请求三元组授权和 Argon2id 密码 KDF 原语，MUST NOT 固定 user-service 的 claims schema、token subject、会话撤销或业务授权模型。

#### Scenario: JWT middleware 使用最小 verifier

- **WHEN** 服务创建共享 JWT 认证 middleware
- **THEN** middleware constructor MUST 只接收 logger、访问令牌 verifier 和可选 token version validator
- **AND** middleware MUST NOT 依赖 token issuer、服务私有配置或具备签发能力的 concrete service
- **AND** access token claims、subject 和业务字段校验 MUST 由服务私有 verifier adapter 拥有

#### Scenario: common 不提供业务 token 签发

- **WHEN** 任一服务导入 `common/security/auth`
- **THEN** 该包 MUST NOT 提供 access、refresh 或 password-change token 签发入口
- **AND** 该包 MUST NOT 定义 user-service 专属 subject 或包含 `user_id`、`token_version`、`session_id` 的专属 claims

#### Scenario: Casbin 授权入口

- **WHEN** 调用方需要获得 Casbin 三元组授权的原始结果
- **THEN** `common/security/casbin.Enforce` MUST 返回 `bool` 和 `error`
- **AND** 拒绝访问到 `ErrDenied` 的转换 MUST 由 `Authorizer.Authorize` 或调用方显式处理

#### Scenario: 创建密码 KDF 实例

- **WHEN** 服务、CLI 或测试需要执行密码哈希或校验
- **THEN** 调用方 MUST 显式创建 Argon2id KDF 实例并提供正数并发上限和正数队列上限
- **AND** 队列上限 MUST 大于或等于并发上限，无效预算 MUST 被拒绝
- **AND** `common/security/password` MUST NOT 暴露包级哈希、校验或可变门控入口

#### Scenario: 密码哈希与校验

- **WHEN** 调用方使用 KDF 实例哈希或校验合法密码
- **THEN** 系统 MUST 生成或解析包含算法、版本、内存、迭代、并行度、盐和派生密钥的受支持 Argon2id 编码
- **AND** 系统 MUST 只接受当前策略允许的参数并使用常量时间比较
- **AND** 每个实例 MUST 独立执行自身的并发和队列预算

#### Scenario: KDF 资源繁忙

- **WHEN** KDF 实例达到执行中和等待中的资源预算
- **THEN** 系统 MUST 返回可由 `errors.Is(err, password.ErrPasswordKDFBusy)` 匹配的应用错误
- **AND** 该错误 MUST 携带 `KindServiceUnavailable`、`Reason=password_kdf_busy`、`CodeServiceUnavailable` 和不泄露预算的公开消息
- **AND** response helper MUST 将其直接渲染为 `503 Service Unavailable`

### Requirement: Runtime 配置、资源与 datastore

系统 MUST 在 `common/runtime/config` 和 `common/runtime/resources` 中分别维护跨服务 runtime 配置以及具名 Redis/PostgreSQL 资源类型、默认值和通用校验，并 MUST 由 `common/runtime/datastore` 使用共享资源类型初始化框架无关的单资源连接池。服务私有业务配置、必需资源名、业务用途和配置 map 到真实资源的选择 MUST 由消费服务拥有。

#### Scenario: 严格加载通用配置

- **WHEN** 服务通过配置文件启动
- **THEN** 共享 loader MUST 解析 runtime、HTTP、gRPC、metrics、tracing、logger 和通用 `local_cache` 配置
- **AND** 系统 MUST 使用 `github.com/go-viper/mapstructure/v2` 的 decode 能力解析 duration、slice 和具名配置
- **AND** 未声明字段 MUST 在启动前失败并报告完整路径，不得使用旧字段别名或 fallback

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

### Requirement: Runtime 执行原语

系统 MUST 在 `common/runtime` 中提供业务中立的 ID、scheduler、workerpool、localcache、Redis key 和 timezone 原语，并 MUST 为后台执行提供明确的容量、并发、失败处理、观测和关闭语义。

#### Scenario: workerpool 生命周期

- **WHEN** 调用方通过 `workerpool.New` 创建任务池并通过 `Stop(ctx)` 关闭
- **THEN** task pool MUST 作为不依赖 Fx 的普通 Go 资源创建，由拥有者显式关闭
- **AND** Stop MUST 停止接收新任务、等待已登记或已接受任务 drain，并允许重复调用共享同一 drain 状态
- **AND** Stop 超时 MUST 返回包装 `context.DeadlineExceeded` 的错误

#### Scenario: workerpool 保持业务中立

- **WHEN** feature 向共享 workerpool 提交后台任务
- **THEN** workerpool MUST 只提供并发控制、显式关闭、日志和统计能力
- **AND** workerpool MUST NOT 承载 refresh session、token version、可靠消息、eventbus、outbox 或业务一致性语义

#### Scenario: scheduler 执行任务

- **WHEN** scheduler 触发已注册任务
- **THEN** 系统 MUST 按本地 overlap gate、全局并发 gate、可选分布式锁、任务 context、可选锁续租、任务执行和 cleanup 的顺序处理
- **AND** 系统 MUST 记录跳过、开始、完成、失败、拒绝和 panic，并在 shutdown 时优雅停止

#### Scenario: scheduler 多实例锁与续租

- **WHEN** 定时任务具有多实例副作用
- **THEN** 任务 MUST 声明正数 TTL 的分布式锁策略，长任务 SHOULD 使用续租
- **AND** 即使任务未配置 timeout，scheduler MUST 创建可取消 context，并在自动续租失败时取消任务和记录失败

#### Scenario: 创建有界本地缓存

- **WHEN** 服务创建 loading cache
- **THEN** 配置 MUST 包含名称、正数容量、正数 TTL、key string 编码和 loader，无效配置 MUST 被拒绝
- **AND** 容量 MUST 作为最大条目预算，调用方 MAY 提供 `CloneFunc` 隔离可变对象

#### Scenario: 本地缓存读取与回源

- **WHEN** `GetOrLoad` miss
- **THEN** cache MUST 使用 `singleflight` 合并同 key 并发回源，成功后尝试写入，且 MUST NOT 缓存 loader 错误
- **AND** singleflight 内部 double-check 命中 MUST NOT 污染业务 hit 统计

#### Scenario: 关闭本地缓存

- **WHEN** cache 已关闭
- **THEN** `GetOrLoad` 和 `Set` MUST 返回 `ErrClosed`，`Get` MUST 返回未命中，`Delete` 和 `Clear` MUST 不再访问底层缓存

#### Scenario: Redis key 归属

- **WHEN** feature 需要 refresh session、token version、RBAC 或其他业务 Redis key
- **THEN** feature infrastructure MUST 拥有业务 key schema，并只能复用 `common/runtime/rediskey` 的通用构造规则

#### Scenario: 初始化进程时区

- **WHEN** runtime 初始化进程时区
- **THEN** timezone primitive MUST 优先使用平台 `TZ` 环境变量并在缺省时使用稳定默认值
- **AND** timezone primitive MUST NOT 依赖核心 Config 或服务业务配置

### Requirement: Logger、观测与 Fx 装配

系统 MUST 在 `common/runtime` 中提供业务中立的 logger、metrics、tracing、Fx provider 和依赖图原语。构造函数和 provider MUST 只消费真实运行时依赖，MUST NOT 为测试便利暴露生产 API 或读取服务私有配置。

#### Scenario: logger 构造无全局副作用

- **WHEN** 调用方通过 `logger.New`、`NewWithConfig` 或 Fx provider `NewLogger` 创建 logger
- **THEN** 系统 MUST 返回由调用方拥有的 logger，Fx provider MUST 注册既有 Sync 关闭 hook
- **AND** 构造过程 MUST NOT 隐式安装、覆盖或恢复进程级默认 logger
- **AND** 默认 logger 只能通过显式 `SetDefault` 修改，并 MAY 作为未注入 logger 时的兜底

#### Scenario: Fx provider 使用真实输入语义

- **WHEN** 共享 provider 的依赖类型唯一且没有 named、optional、group 或其他 DI metadata
- **THEN** provider MUST 使用普通强类型参数
- **AND** 只有存在真实 DI metadata、复杂输出映射或明显的多依赖可读性收益时 MAY 使用 Params 容器
- **AND** provider MUST 只消费跨服务配置和 primitive，不得导入服务私有配置

#### Scenario: 生成稳定依赖图

- **WHEN** 服务将 Fx option 或 module 传入 `common/runtime/fxgraph`
- **THEN** helper MUST 输出稳定排序的 provider、invoke 和依赖关系图文本
- **AND** helper MUST NOT 构造或要求服务私有配置、feature provider、Ent、Redis、PostgreSQL、OTLP 或 HTTP server 输入

#### Scenario: 公开 API 具有运行时职责

- **WHEN** `common/runtime` 新增公开 constructor、method、option 或 hook
- **THEN** 入口 MUST 具有真实运行时职责或已定义的稳定共享契约
- **AND** 仅测试消费、暴露内部状态或绕过正常 lifecycle 的能力 MUST 留在包内、`_test.go` fixture 或 `common/testing`

### Requirement: common、shared 与外部集成边界

系统 MUST 将 `common` 限制为跨服务稳定且业务中立的能力，将 `user-service/internal/shared` 限制为至少两个 feature 真实消费的服务内纯业务内核，并将真实外部协议适配放入 `user-service/internal/integration/http|grpc|events`。

#### Scenario: 能力归入 common

- **WHEN** 新能力准备放入 `common`
- **THEN** 能力 MUST 跨服务可复用、无 user-service 业务语义且具有稳定契约
- **AND** `common` MUST NOT 依赖 feature 包或承载业务 DTO、业务 key schema、policy loader、route diff、服务 OpenAPI 元数据、eventbus 或 outbox 设计

#### Scenario: 服务内 shared kernel

- **WHEN** user/auth 共享身份语义或 role/permission 共享系统 RBAC 基线
- **THEN** 系统 MUST 分别使用 `internal/shared/identity` 和 `internal/shared/rbacbaseline`
- **AND** shared 包 MUST 保持纯业务语义，不得依赖 feature、Gin、Ent、Redis、SQL、Fx、runtime provider、HTTP response、DTO、store port、外部调用或部署资产

#### Scenario: 新增 shared 能力

- **WHEN** helper 只被一个 feature 使用、只是技术工具或需要基础设施依赖
- **THEN** 系统 MUST NOT 将其放入 `internal/shared`
- **AND** shared 子包 MUST 使用具体业务名称，不得创建根级 `errors`、`enums`、`types`、`utils` 或 `helpers` 兜底包

#### Scenario: 接入真实外部系统

- **WHEN** feature 调用真实外部 HTTP、gRPC 或事件系统
- **THEN** feature application MUST 拥有最小消费侧端口，`integration/http|grpc|events` MUST 只实现协议适配与防腐
- **AND** 入站 gRPC handler MUST 位于所属 feature 的 `transport/grpc`，feature-specific consumer 映射和 handler MUST 位于所属 feature

#### Scenario: 禁止推测性集成

- **WHEN** 没有真实 broker、外部 API 或单独批准的设计
- **THEN** 系统 MUST NOT 新增 eventbus、outbox、producer、subscriber、consumer handler、dispatcher、Ent hook 或 transaction wrapper

### Requirement: 测试基础设施与测试边界

系统 MUST 在 `common/testing` 中提供可复用的容器和 fixture，并 MUST 使用可重复、可观察且不污染生产 API 或进程全局状态的测试方式验证共享能力。

#### Scenario: 集成测试依赖服务

- **WHEN** Go 测试需要真实 PostgreSQL 或 Redis
- **THEN** 测试 MUST 优先使用 `common/testing/containers` 管理依赖生命周期
- **AND** 测试数据 MUST 使用稳定 fixture 或 feature-local builder，避免不可重复的随机输入

#### Scenario: 使用生成 mock

- **WHEN** 测试以 expectation 表达高重复 interface 的调用参数、次数或返回值
- **THEN** 测试 MUST 优先使用 `go.uber.org/mock/mockgen` 生成消费侧 package-local mock
- **AND** 系统 MUST NOT 创建全局或跨 feature 中央 mock 仓库
- **AND** 复杂内存状态或并发协调 MAY 使用 package-local 状态型 harness

#### Scenario: 测试不扩张生产接口

- **WHEN** 测试需要注入失败、固定返回、控制顺序或观察后台状态
- **THEN** 测试 MUST 使用消费侧最小接口、局部 fixture、通道或可观察状态
- **AND** 正式代码 MUST NOT 为测试新增全局可变函数、测试 flag、`NewXForTest` 或无运行时职责的 adapter

#### Scenario: 语义化测试断言

- **WHEN** 测试检查错误、相等性、nil、布尔、集合、字符串、JSON、时间或 panic 等常见结果
- **THEN** 测试 MUST 优先使用 `testify/require` 的语义化断言；多个互不依赖的检查 MAY 使用 `testify/assert`
- **AND** 测试 MUST NOT 用 `Fail*` 机械包装可语义化的断言
- **AND** 直接 `testing.T` 失败调用 MUST 限于自定义控制流、特殊诊断或不适合依赖 testify 的 helper

#### Scenario: 异步测试使用可观察条件

- **WHEN** 测试验证缓存过期、workerpool drain、scheduler 续租或后台任务取消
- **THEN** 测试 MUST 使用通道、eventually-style 条件或其他可观察同步机制
- **AND** 测试 MUST NOT 只依赖固定 `time.Sleep` 判断状态已经变化

#### Scenario: 隔离进程级状态

- **WHEN** 测试必须修改默认 logger、`TZ`、`time.Local` 或包级初始化状态
- **THEN** 测试 MUST 在 package-local helper 中保存状态并通过 cleanup 恢复
- **AND** 环境变量 MUST 使用 `t.Setenv`，相关测试 MUST NOT 并行执行
- **AND** 非测试目标所需的日志捕获 MUST 使用 context logger 或局部 logger 注入

### Requirement: 共享默认值与只读状态安全

系统 MUST 保证配置允许值、middleware 默认策略和 validation tag 等共享只读数据不暴露可被调用方或同包无意修改的底层状态，同时保持公开默认值、错误消息和运行时行为稳定。

#### Scenario: 固定集合不可变

- **WHEN** runtime config 或 resources 校验 log、server、SSL、tracing 等固定允许值
- **THEN** 实现 MUST 使用不可被共享写入的查询方式表达固定集合
- **AND** 合法值、非法值和公开错误消息 MUST 保持稳定

#### Scenario: 默认配置与调用方输入隔离

- **WHEN** helper 接收 slice、map 或默认 struct 并据此创建长期存活对象
- **THEN** helper MUST 持有与 package default 和调用方输入隔离的副本
- **AND** 调用方后续修改原始数据 MUST NOT 改变已创建对象的行为

#### Scenario: 运行时可变状态保持显式

- **WHEN** package-level 变量承载 sentinel error、regexp、Fx Module、`sync.Pool`、atomic counter 或其他必要运行时状态
- **THEN** 系统 MUST 明确其可变性和并发边界，不得将其伪装为可写的只读配置集合
