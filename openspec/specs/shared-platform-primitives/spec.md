## Purpose

定义 `common/` 提供的跨服务共享契约、HTTP helper、安全原语、runtime primitive、测试基础设施和校验能力，保证服务间基础行为一致。
## Requirements
### Requirement: 跨服务契约基础

系统 MUST 在 `common/` 中维护跨服务共享的错误、响应 envelope、分页和 HTTP response helper，以保证服务之间的外部契约保持一致，并保持业务中立。共享错误契约 MUST 使用语义驱动的应用错误模型表达错误类别、业务原因、稳定响应码、公开消息和内部原因，并 MUST NOT 在 `common/contract/errors` 中保存、暴露或推导 HTTP status。HTTP status MUST 只由 `common/http/response` 根据应用错误 `Kind` 推导。

#### Scenario: 返回统一响应

- **WHEN** 服务处理成功响应或错误响应
- **THEN** 系统 MUST 使用共享响应和错误契约表达 code、message、data、pagination 或错误详情

#### Scenario: 新服务复用契约

- **WHEN** 新服务模块需要对外暴露 HTTP API
- **THEN** 该服务 MUST 优先复用 `common/contract/` 和 `common/http/response/` 中的稳定契约，而不是定义不兼容的 envelope

#### Scenario: 契约变更需要规格化

- **WHEN** 共享错误码、响应 envelope 或分页结构需要改变
- **THEN** change MUST 更新相关主规格或 delta spec，并评估所有使用 `common/contract/` 的服务影响

#### Scenario: 新增错误分类

- **WHEN** 需要在 `common/contract/errors` 新增跨服务错误分类或原因
- **THEN** `Kind` 和 `Reason` MUST 保持业务中立或由明确业务边界声明
- **AND** `Kind` MUST 表达低基数错误类别
- **AND** `Reason` MUST 表达稳定、可公开的错误原因
- **AND** HTTP status MUST 由 `common/http/response` 根据 `Kind` 渲染

#### Scenario: 应用错误不暴露 HTTP status

- **WHEN** 调用方创建、包装或检查 `common/contract/errors` 应用错误
- **THEN** 应用错误 MUST 暴露 `Kind`、`Reason`、`Code`、`Message` 和可选 `Cause`
- **AND** 应用错误 MUST NOT 暴露 `HTTPStatus` 字段
- **AND** 应用错误构造 API MUST NOT 接收 HTTP status 参数

#### Scenario: HTTP 层推导错误状态码

- **WHEN** `common/http/response` 写入应用错误响应
- **THEN** 系统 MUST 根据应用错误 `Kind` 推导 HTTP status
- **AND** 请求格式错误或字段校验失败 MUST 渲染为 `400 Bad Request`
- **AND** 未认证或 token 无效、过期、撤销 MUST 渲染为 `401 Unauthorized`
- **AND** 权限不足 MUST 渲染为 `403 Forbidden`
- **AND** 冲突 MUST 渲染为 `409 Conflict`
- **AND** 未找到 MUST 渲染为 `404 Not Found`
- **AND** 服务不可用 MUST 渲染为 `503 Service Unavailable`
- **AND** nil、未知或内部错误 MUST 渲染为 `500 Internal Server Error`

#### Scenario: 应用错误转换和包装

- **WHEN** 系统通过 `FromError` 归一化错误
- **THEN** wrapped application error MUST 保留原始 `Kind`、`Reason`、`Code` 和公开 `Message`
- **AND** nil error MUST 按内部错误处理
- **AND** 未知 error MUST 按内部错误处理并使用非敏感公开 message
- **AND** 原始错误 MUST 只作为内部 `Cause` 保留

#### Scenario: 标准错误链支持

- **WHEN** 调用方使用 `errors.As` 检查 wrapped application error
- **THEN** 系统 MUST 能从错误链中解析出应用错误
- **WHEN** 调用方使用 `errors.Is` 按应用错误类别或原因匹配
- **THEN** 系统 MUST 按 `Kind` 和 `Reason` 的稳定语义进行匹配
- **AND** 内部 `Cause` MUST 继续支持标准 `errors.Is` 和 `errors.As`

#### Scenario: 校验错误响应

- **WHEN** 请求绑定或字段校验失败
- **THEN** `common/validation` 和 `common/http/binding` MUST 生成或传播语义应用错误分类
- **AND** `common/http/response` MUST 将字段校验失败渲染为 `400 Bad Request`
- **AND** 响应 envelope MUST 保持 `success=false`、`code=CodeValidationFailed`、公开 message 和结构化字段错误明细

#### Scenario: 强制改密错误码稳定

- **WHEN** 服务需要表达用户凭据有效但账号要求强制修改密码
- **THEN** 系统 MUST 使用 `CodePasswordChangeRequired`
- **AND** 该 code 的数值 MUST 为 `20006`

#### Scenario: 错误码保持业务中立

- **WHEN** `common/contract/errors` 新增 `CodePasswordChangeRequired`
- **THEN** `common` MUST 只定义共享错误码和通用错误构造能力
- **AND** `common` MUST NOT 承载 user-service 的受限 token 签发、强制改密状态判断或登录响应编排逻辑

#### Scenario: 服务不可用错误

- **WHEN** 服务需要表达临时资源池繁忙、依赖暂时不可用或实例无法处理当前请求
- **THEN** 共享错误契约 MUST 提供业务中立的服务不可用 `Kind`
- **AND** `common/http/response` MUST 将该 `Kind` 渲染为 `503 Service Unavailable`
- **AND** 具体业务边界 MUST 提供不泄露内部实现细节的公开消息

#### Scenario: 不保留旧兼容路径

- **WHEN** 系统完成语义应用错误模型迁移
- **THEN** `common/contract/errors` MUST NOT 保留旧 `HTTPStatus` 字段
- **AND** 系统 MUST NOT 保留接收 HTTP status 的旧 factory API
- **AND** 系统 MUST NOT 保留从旧状态码直连模型到新模型的兼容适配层

### Requirement: HTTP 与安全中间件基础

系统 MUST 在 `common/http/` 和 `common/security/` 中提供可复用的绑定、校验、认证、授权、CORS、metrics、logging、recovery、OpenAPI 和 pprof 基础能力。

#### Scenario: HTTP 请求进入服务

- **WHEN** HTTP 请求被 Gin 路由处理
- **THEN** 服务 MUST 能复用共享 middleware 完成认证上下文、授权检查、日志字段、metrics 采集、panic recovery 和 span error 记录

#### Scenario: 输入校验失败

- **WHEN** 请求绑定或字段校验失败
- **THEN** 系统 MUST 通过共享 binding、validation 和 response helper 返回一致的校验错误结构

#### Scenario: OpenAPI 输出

- **WHEN** 服务生成或转换 OpenAPI 文档
- **THEN** 系统 MUST 复用 `common/http/openapi/` 的转换与渲染约束，避免服务间文档格式漂移

#### Scenario: 服务特定授权行为

- **WHEN** 授权行为依赖 user-service 的 `user:<uuid>` subject、角色、权限目录、route diff 或超级管理员基线
- **THEN** 行为 MUST 留在 user-service permission/shared 边界，不得放入 `common/security/casbin` 或通用 HTTP middleware

#### Scenario: 服务特定 OpenAPI 元数据

- **WHEN** OpenAPI 元数据描述 user-service API server、认证方案、源码扫描范围、健康路径或输出目录
- **THEN** 元数据 MUST 位于 user-service 脚本或薄 wrapper，不得放入 `common/http/openapi`

### Requirement: 共享认证授权 helper API 治理

系统 MUST 在 `common/http` 和 `common/security` 中保持认证、授权 helper 的导出 API 语义清晰且避免重复简写入口；当共享 helper 只包装另一个推荐入口、暴露未参与行为的参数或没有额外稳定语义时，系统 MUST 通过显式推荐入口、废弃标记或移除策略治理该 helper。

#### Scenario: Casbin 授权 helper 收紧

- **WHEN** 调用方需要获得 Casbin 三元组授权的原始允许结果
- **THEN** 系统 MUST 提供 `common/security/casbin.Enforce` 作为返回 `bool` 和 `error` 的推荐入口
- **AND** 拒绝访问转换为 `ErrDenied` 的 error-only 语义 MUST 由 `Authorizer.Authorize` 或调用方显式处理

#### Scenario: JWT middleware 无 token version 校验

- **WHEN** 服务需要创建不执行 token version 撤销校验的 JWT 认证中间件
- **THEN** 系统 MUST 推荐调用 `AuthWithTokenVersionValidator(log, jwtService, nil)` 显式表达该行为
- **AND** 仅作为兼容保留的简写 helper MUST 标记为废弃或在确认无消费者后移除

#### Scenario: JWT middleware 不接收无效配置参数

- **WHEN** 服务需要创建共享 JWT 认证 middleware
- **THEN** `AuthWithTokenVersionValidator` MUST 只接收 logger、JWT service 和可选 token version validator 作为调用参数
- **AND** `AuthWithTokenVersionValidator` MUST NOT 接收 `config.AuthConfig` 或其他不参与运行时认证行为的配置参数
- **AND** JWT 配置 MUST 由 `auth.NewJWTService(config.AuthConfig)` 消费后通过 `JWTService` 注入 middleware

#### Scenario: token version validator 函数适配器移除

- **WHEN** 服务需要为共享 JWT 认证 middleware 提供 token version 撤销校验
- **THEN** 调用方 MUST 直接提供实现 `common/security/auth.TokenVersionValidator` 的具体类型
- **AND** `common/http/middleware` MUST NOT 暴露只将函数包装为 `TokenVersionValidator` 的 `TokenVersionValidatorFunc` 适配器

#### Scenario: 行为保持不变

- **WHEN** 共享认证授权 helper 的重复入口或无效参数被废弃或移除
- **THEN** 系统 MUST 保持 JWT 解析、token version 校验、Casbin 三元组校验、`ErrNotConfigured`、`ErrDenied` 和 HTTP 响应语义不变
- **AND** user-service 的认证路由挂载和 RBAC 保护路由 MUST 不因该 API 治理发生行为变化

### Requirement: 密码 KDF 显式实例化

系统 MUST 在 `common/security/password` 中提供可显式实例化的 Argon2id 密码哈希与校验 primitive。调用方 MUST 通过实例方法执行密码哈希和校验，并 MUST 在构造实例时声明本实例的 Argon2id 并发上限和队列上限。`common/security/password` MUST NOT 暴露包级密码哈希、包级密码校验或包级可变 Argon2id 门控入口。

#### Scenario: 创建密码 KDF 服务实例

- **WHEN** 服务、CLI 或测试需要执行密码哈希或校验
- **THEN** 调用方 MUST 显式创建 `common/security/password` 的密码 KDF 服务实例
- **AND** 构造参数 MUST 包含正数 Argon2id 并发上限和正数队列上限
- **AND** 队列上限 MUST 大于或等于并发上限

#### Scenario: 拒绝无效 KDF 资源预算

- **WHEN** 调用方使用非正数并发上限、非正数队列上限或小于并发上限的队列上限创建密码 KDF 服务
- **THEN** 系统 MUST 返回明确错误并拒绝创建实例

#### Scenario: 通过实例执行密码哈希

- **WHEN** 调用方使用密码 KDF 服务实例对合法明文密码执行哈希
- **THEN** 系统 MUST 使用 Argon2id 当前安全参数生成包含算法、版本、内存、迭代、并行度、盐和派生密钥的编码哈希
- **AND** 系统 MUST 使用该实例的队列和并发预算限制本实例内执行中和等待中的 KDF 请求

#### Scenario: 通过实例执行密码校验

- **WHEN** 调用方使用密码 KDF 服务实例校验合法明文密码和受支持的编码哈希
- **THEN** 系统 MUST 解析编码哈希中的算法、版本和参数
- **AND** 系统 MUST 只接受当前策略允许的 Argon2id 参数
- **AND** 系统 MUST 使用常量时间比较返回密码是否匹配

#### Scenario: KDF 门控只属于实例

- **WHEN** 多个服务组件、CLI 或测试在同一进程内需要不同密码 KDF 资源预算
- **THEN** 系统 MUST 允许它们持有不同密码 KDF 服务实例
- **AND** 一个实例的队列和并发占用 MUST NOT 消耗另一个实例的队列和并发预算

### Requirement: Runtime primitive 基础

系统 MUST 在 `common/runtime/` 中维护配置加载、数据存储、logger、metrics、tracing、scheduler、workerpool、localcache、Redis key 和 timezone 等 runtime primitive。`common/runtime/config` MUST 将 `local_cache` 表达为通用具名缓存实例集合，并 MUST NOT 固定 user-service 的 `auth_token_version`、`rbac_user_roles` 或其他业务缓存名。`common/runtime/config` MUST 使用 `github.com/go-viper/mapstructure/v2` 作为配置反序列化依赖，并 MUST NOT 保留旧版 `github.com/mitchellh/mapstructure` 导入、兼容层或旧行为 fallback。

#### Scenario: 服务启动加载配置

- **WHEN** 服务通过配置文件启动
- **THEN** 系统 MUST 使用共享配置 loader 与 validation 解析 runtime、HTTP、auth、Postgres、Redis、metrics、tracing、logger 和通用 `local_cache` 配置

#### Scenario: runtime 依赖初始化

- **WHEN** 服务需要连接 Postgres、Redis、logger、metrics 或 tracing provider
- **THEN** 服务 MUST 优先复用 `common/runtime/` 中的 provider 和 Fx module

#### Scenario: 后台任务执行

- **WHEN** 服务需要执行定时任务、分布式锁或固定 worker pool 任务
- **THEN** 系统 MUST 使用共享 scheduler、lock、workerpool 和 metrics 约束，并记录失败、拒绝、panic 和完成事件

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

- **WHEN** `common/runtime/config` 将 Viper 读取到的配置反序列化为 `Config`
- **THEN** 系统 MUST 使用 `github.com/go-viper/mapstructure/v2` 提供的 decode hook 和 decode 配置能力
- **AND** duration、slice、具名 Postgres、具名 Redis 和具名 `local_cache` 配置 MUST 按 v2 标准行为解析
- **AND** 系统 MUST NOT 导入 `github.com/mitchellh/mapstructure` 或保留面向旧版行为的兼容代码

### Requirement: scheduler 包内结构保持稳定契约

系统 MUST 允许 `common/runtime/scheduler` 按公开配置类型、调度器生命周期、任务执行流程、分布式锁、锁续租和校验逻辑拆分包内文件，同时保持 `package scheduler`、导出 API、错误变量、metrics 事件、日志语义、cron parser、并发控制、锁策略、续租策略和 shutdown 行为不变。

#### Scenario: 拆分 scheduler 源码文件

- **WHEN** `common/runtime/scheduler` 将 `scheduler.go` 中的类型、生命周期、执行、锁续租或校验逻辑移动到不同源码文件
- **THEN** 系统 MUST 保持原有导出符号、函数签名、常量值、错误语义和调用方导入路径不变
- **AND** scheduler 的任务注册、触发、跳过、失败、panic recovery、完成、分布式锁获取、自动续租和优雅关闭行为 MUST 不变

#### Scenario: 保持业务中立边界

- **WHEN** scheduler 包内结构被拆分或命名调整
- **THEN** `common/runtime/scheduler` MUST 继续只承载无业务语义 runtime primitive
- **AND** 系统 MUST NOT 将 user-service feature 语义、业务 Redis key schema、HTTP controller、Fx provider、Ent、部署资产或观测 dashboard 逻辑放入 scheduler 包

### Requirement: Fx 依赖图 runtime primitive

系统 MUST 在 `common/` 中提供业务中立的 Fx 依赖图构建与渲染能力，使服务可以从自身 Fx module 或 app option 生成稳定、可审查的依赖图文本。

#### Scenario: 生成业务中立依赖图

- **WHEN** 服务将 Fx option 或 module 传入共享依赖图 helper
- **THEN** 系统 MUST 返回描述 provider、invoke、输入输出依赖或等价 Fx 依赖关系的图文本
- **AND** 该 helper MUST NOT 引入 user-service feature、HTTP route、RBAC policy、Ent schema 或服务专用配置语义

#### Scenario: 输出稳定图文本

- **WHEN** 相同 Fx module 在代码未变化的情况下重复生成依赖图
- **THEN** 系统 MUST 输出稳定排序的图文本，避免产生无意义 diff

#### Scenario: 拒绝放入服务内 shared kernel

- **WHEN** Fx 依赖图能力与具体业务 feature 无关
- **THEN** 系统 MUST 将公共方法放在 `common/` 的 runtime primitive 边界
- **AND** 系统 MUST NOT 将该能力放入 `user-service/internal/shared` 或任一 feature 包

### Requirement: 有界本地缓存 primitive

系统 MUST 在 `common/runtime/localcache` 中提供有明确容量上限、TTL、回源合并、主动失效、统计快照和关闭语义的本地缓存 primitive。缓存实例 MUST 通过显式配置创建，配置 MUST 包含名称、容量、TTL 和 key string 编码；旧的仅传入 TTL 的构造方式 MUST 被移除。

#### Scenario: 创建有界本地缓存

- **WHEN** 服务创建 `localcache` 实例
- **THEN** 系统 MUST 要求配置 `Name`、正数 `Capacity`、正数 `TTL` 和 `KeyString`
- **AND** 系统 MUST 将 `Capacity` 作为本地缓存容量预算，第一版以 `cost=1` 表示最大条目预算

#### Scenario: 拒绝无效缓存配置

- **WHEN** 服务使用空名称、非正数容量、非正数 TTL、空 key string 编码或空 loader 创建 loading cache
- **THEN** 系统 MUST 返回明确错误并拒绝创建缓存

#### Scenario: 缓存读取与回源

- **WHEN** 调用方通过 `GetOrLoad` 读取 key 且本地缓存 miss
- **THEN** 系统 MUST 使用 `singleflight` 合并同 key 并发回源
- **AND** 系统 MUST 在回源成功后尝试写入本地缓存
- **AND** 系统 MUST NOT 缓存 loader 返回的错误

#### Scenario: 缓存对象隔离

- **WHEN** 缓存 value 类型可被调用方修改
- **THEN** 系统 MUST 允许调用方提供 `CloneFunc`
- **AND** 系统 MUST 使用 clone 隔离 loader 返回对象、缓存内部对象和调用方返回对象

#### Scenario: 关闭后的访问

- **WHEN** 缓存实例已经调用 `Close`
- **THEN** `GetOrLoad` 和 `Set` MUST 返回 `ErrClosed`
- **AND** `Get` MUST 返回未命中
- **AND** `Delete` 和 `Clear` MUST 不再触碰底层缓存

#### Scenario: 统计不污染命中率

- **WHEN** `GetOrLoad` 进入 singleflight 后执行 double-check 并命中缓存
- **THEN** 系统 MUST 记录 double-check 命中
- **AND** 系统 MUST NOT 将该内部命中计入业务 hit 统计

### Requirement: localcache 包内结构保持稳定契约

系统 MUST 允许 `common/runtime/localcache` 按错误变量、公开类型和核心实现拆分包内文件，同时保持 `package localcache`、导出 API、错误变量和运行时行为不变。

#### Scenario: 拆分包内文件

- **WHEN** `common/runtime/localcache` 将错误变量、公开类型和 `Cache` 实现拆分到不同源码文件
- **THEN** 系统 MUST 保持原有导出符号、错误语义、Ristretto 配置、TTL、singleflight、stats 和 `Close` 行为不变

### Requirement: 服务内 shared kernel

系统 MUST 将 `user-service/internal/shared` 限制为服务内稳定业务内核，只承载已被至少两个 feature 真实消费、边界稳定且不能归入 `common` 的纯业务规格。

#### Scenario: 当前允许共享包

- **WHEN** user/auth 需要用户状态或身份错误，role/permission 需要系统 RBAC 基线
- **THEN** 系统 MUST 分别使用 `internal/shared/identity` 和 `internal/shared/rbacbaseline`，并 MUST NOT 在 feature 内复制常量或保留兼容 alias

#### Scenario: 新增共享 helper

- **WHEN** helper 只被一个 feature 使用或只是技术工具函数
- **THEN** 系统 MUST NOT 将其放入 `internal/shared`

#### Scenario: shared 子包命名

- **WHEN** 确需新增或调整 shared 子包
- **THEN** 包名 MUST 使用稳定业务内核语义，MUST NOT 新增根级 `errors`、`enums`、`types`、`utils` 或 `helpers` 兜底包；公共错误和枚举 MUST 放入 owning 子包的具体文件

#### Scenario: shared 禁止依赖

- **WHEN** 拟新增 shared 包需要 Ent、SQL、Redis、Gin、Fx、HTTP response、runtime provider、feature use case、store port、DTO、外部调用或部署资产
- **THEN** 设计 MUST 被拒绝或移动到所属 feature 的 application/infrastructure 边界

### Requirement: 外部集成边界

系统 MUST 将 `user-service/internal/integration/http|grpc|events` 作为真实外部系统协议适配和防腐层边界，避免推测性集成污染服务内架构。

#### Scenario: 新增外部 HTTP client

- **WHEN** feature 需要调用真实外部 HTTP 系统
- **THEN** feature application MUST 拥有最小消费侧端口，`integration/http` MAY 实现协议适配器

#### Scenario: 禁止推测性集成

- **WHEN** 没有单独批准的设计
- **THEN** 系统 MUST NOT 新增 Kafka、RabbitMQ、NATS、Redis Stream、eventbus、outbox、producer、subscriber、consumer handler、dispatcher、Ent hook 或 transaction wrapper

#### Scenario: gRPC 与事件边界分离

- **WHEN** user-service 暴露真实入站 gRPC API 或消费真实外部 broker 事件
- **THEN** 入站 gRPC handler MUST 位于所属 feature 的 `transport/grpc`；broker subscription、ack 和协议机制 MAY 位于 `integration/events`，feature-specific 映射和 handler adapter MUST 位于所属 feature 的 `infrastructure/consumers`

### Requirement: 测试 mock 生成规范

系统 MUST 为高重复 interface 测试 double 提供生成化 mock 规范，并保持 mock 生成物归属于接口消费侧 feature-local 测试包。系统 MUST NOT 创建全局 `mocks/`、`testmocks/`、`common/mocks/` 或等价中央 mock 仓库来承载跨 feature mock。

#### Scenario: 生成 feature-local mock

- **WHEN** application、transport 或 infrastructure 测试需要替代高重复 store、notifier、policy engine、session store 或 metrics recorder interface
- **THEN** 测试 MUST 优先使用 `go.uber.org/mock/mockgen` 生成的 feature-local mock
- **AND** mock 生成物 MUST 放在接口消费侧 package 或其测试专用子包内
- **AND** mock 生成物 MUST NOT 放入中央 mock 仓库

#### Scenario: 保留状态型测试 harness

- **WHEN** 测试需要复杂内存状态、E2E 流程状态或比 expectation mock 更清晰的领域测试夹具
- **THEN** 测试 MAY 保留 package-local 测试 harness
- **AND** 该对象 MUST 使用 `testHarness`、`testStore`、`recordingMetrics` 或等价描述性名称
- **AND** 该对象 MUST NOT 作为跨 feature 共享 mock 导出

#### Scenario: 禁止测试驱动生产冗余接口

- **WHEN** 为了生成 mock 或迁移测试 double 调整代码
- **THEN** 系统 MUST NOT 仅为单元测试新增与业务无关的生产接口、分支、适配层或 `NewXForTest` 入口
- **AND** 测试 MUST 基于现有 feature/application 边界和合理的可测试性设计

### Requirement: 测试基础设施

系统 MUST 在 `common/testing/` 中提供可复用的容器和 fixture 能力，用于支撑 Postgres、Redis 和测试数据场景。

#### Scenario: 集成测试需要依赖服务

- **WHEN** Go 测试需要真实 Postgres 或 Redis
- **THEN** 测试 MUST 优先使用 `common/testing/containers/` 启动依赖，避免每个模块重复实现容器生命周期

#### Scenario: 测试数据需要稳定生成

- **WHEN** 测试需要生成用户、角色、权限或其他输入数据
- **THEN** 测试 MUST 优先使用共享 fixture 或本 feature 内明确的 builder，避免随机数据破坏可重复性

### Requirement: 测试断言与失败处理风格

测试代码中的断言与失败处理 MUST 优先使用 `testify/require`，以提升测试可读性、减少手写失败判断样板代码、统一阻塞式失败处理方式，并提供一致、清晰的失败诊断信息。`common/contract`、`common/validation` 和 `common/testing` 的历史测试迁移或新增测试 MUST 将常见错误、相等性、集合、fixture 和容器测试断言表达为语义化 `require` 方法，除非该失败调用属于测试控制流、特殊诊断输出或测试框架边界。

#### Scenario: 常见阻塞式断言

- **WHEN** 测试需要检查错误返回值、前置条件、对象相等性、布尔条件、集合长度、错误类型或状态
- **THEN** 测试 MUST 使用语义化的 `require` 断言方法，例如 `require.NoError`、`require.Error`、`require.ErrorIs`、`require.Equal`、`require.True`、`require.False`、`require.Len`、`require.NotNil`
- **AND** 测试 SHOULD NOT 直接使用 `t.Fatal`、`t.Fatalf`、`t.Error` 或 `t.Errorf` 表达这些常见断言

#### Scenario: 共享基础包历史测试迁移

- **WHEN** `common/contract`、`common/validation` 或 `common/testing` 的 `_test.go` 文件迁移历史断言或新增常见阻塞式断言
- **THEN** 测试 MUST 优先使用 `require.NoError`、`require.ErrorIs`、`require.Equal`、`require.Len`、`require.Contains`、`require.NotNil` 或等价语义化 `require` 方法
- **AND** 目标模块 MUST 直接声明实际使用的 `github.com/stretchr/testify` 测试依赖
- **AND** 迁移 MUST NOT 改变对应生产包的公开 API、错误语义或运行时行为

#### Scenario: 避免机械 Fail 替换

- **WHEN** 测试迁移手写失败判断或新增失败处理
- **THEN** 测试 SHOULD NOT 将 `t.Fatal`、`t.Fatalf`、`t.Error` 或 `t.Errorf` 机械替换为 `require.FailNow`、`require.FailNowf`、`require.Fail`、`require.Failf`、`assert.Fail` 或 `assert.Failf`
- **AND** 当存在明确的语义化 `require` 或 `assert` 断言方法时，测试 MUST 优先使用对应断言方法

#### Scenario: 非阻塞式独立断言

- **WHEN** 单个测试用例需要在一次执行中继续收集多个相互独立的断言失败
- **THEN** 测试 MAY 使用 `testify/assert` 进行非阻塞式断言
- **AND** 初始化失败、前置条件失败或后续检查依赖当前结果的场景 MUST 使用 `testify/require` 立即终止当前测试

#### Scenario: 保留特殊失败控制流

- **WHEN** 测试存在无法通过现有 `require` 或 `assert` 语义化断言清晰表达的自定义测试控制流、特殊诊断输出，或测试辅助工具不适合依赖 `testify`
- **THEN** 测试 MAY 直接使用 `t.Fatal`、`t.Fatalf`、`t.Error`、`t.Errorf`、`require.FailNowf` 或 `assert.Failf`
- **AND** 此类用法 SHOULD 让保留原因在代码上下文中保持清晰
- **AND** 在 `common/contract`、`common/validation` 或 `common/testing` 迁移完成时，剩余命中 MUST 在实施任务记录中列明并确认符合例外规则

### Requirement: common 边界 mock 生成规范

系统 MUST 为 `common` 中适合 expectation 表达的边界 interface 测试 double 提供 package-local mockgen 生成入口。生成 mock MUST 仅用于对应 package 的测试，系统 MUST NOT 创建 `common/mocks`、全局 `mocks/`、`testmocks/` 或等价中央 mock 仓库。

#### Scenario: common 授权 enforcer 测试使用生成 mock

- **WHEN** `common/security/casbin` 测试需要模拟 `Enforcer` 的允许、拒绝、错误或调用参数
- **THEN** 测试 MUST 使用该 package 内的 `go.uber.org/mock/mockgen` 生成 mock 表达 expectation
- **AND** 测试 MUST NOT 保留被迁移的手写 recording double 兼容路径

#### Scenario: common HTTP middleware 测试使用生成 mock

- **WHEN** `common/http/middleware` 测试需要模拟 `CasbinAuthorizer` 或 `auth.TokenVersionValidator`
- **THEN** 测试 MUST 使用 `common/http/middleware` package-local 生成 mock 表达调用次数、入参和错误返回
- **AND** 生成 mock MUST NOT 作为跨 package 共享测试 API 暴露

#### Scenario: 保持 common 生产语义不变

- **WHEN** 测试 double 迁移为 generated mock
- **THEN** 系统 MUST 保持 Casbin 三元组授权、`ErrNotConfigured`、`ErrDenied`、JWT 解析、token version mismatch、HTTP 响应和日志语义不变
- **AND** 系统 MUST NOT 仅为测试新增业务无关生产接口、adapter、分支或 `NewXForTest` 入口

#### Scenario: 状态型测试 harness 不强制迁移

- **WHEN** `common` 测试对象需要复杂内存状态、并发协调、scheduler 生命周期或比 expectation mock 更清晰的测试夹具
- **THEN** 测试 MAY 保留 package-local 状态型 harness
- **AND** 该 harness MUST NOT 被迁移到中央 mock 仓库或导出为跨 package 测试依赖

### Requirement: common HTTP 测试断言规范

系统 MUST 在 `common/http/**/*_test.go` 中使用语义化 `testify` 断言验证共享 HTTP helper、binding、middleware、response、OpenAPI 和 pprof 相关行为。初始化失败、前置条件失败以及后续检查依赖当前结果的场景 MUST 使用 `testify/require`；只有需要在单次测试执行中收集多个相互独立响应字段失败时，系统 MAY 使用 `testify/assert`。

#### Scenario: 验证 HTTP status 和 header

- **WHEN** `common/http` 测试验证 HTTP 响应状态码、响应 header 或中间件写入结果
- **THEN** 测试 MUST 优先使用 `require.Equal`、`require.Contains`、`require.NotEmpty` 或等价语义化断言
- **AND** 测试 MUST NOT 将可语义化表达的检查迁移为 `require.Fail*`、`assert.Fail*`、`t.Fatal*` 或 `t.Error*`

#### Scenario: 验证 JSON envelope

- **WHEN** `common/http` 测试验证 JSON response envelope、错误详情或分页结构
- **THEN** 测试 MUST 优先使用 `require.JSONEq` 或在 `require.NoError` 解析后使用语义化字段断言
- **AND** 测试 MUST 验证当前稳定 envelope 结构，不得新增旧 envelope 兼容断言或双写断言

#### Scenario: 验证 binding 错误

- **WHEN** `common/http/binding` 测试验证请求绑定、校验失败或错误响应
- **THEN** 测试 MUST 使用 `require.NoError`、`require.Error`、`require.ErrorIs`、`require.Equal`、`require.Contains` 或等价语义化断言表达预期
- **AND** 只有无法通过现有语义化断言清晰表达的自定义测试控制流或特殊诊断输出 MAY 保留直接 `t.Fatal*`、`t.Error*` 或 `Fail*`

#### Scenario: 验证迁移完成度

- **WHEN** 断言迁移完成
- **THEN** `rg "t\\.Fatalf|t\\.Fatal\\(|t\\.Errorf|t\\.Error\\(|Failf?\\(" common/http --glob '*_test.go'` 的剩余命中 MUST 均符合 `docs/TESTING.md` 特殊例外规则
- **AND** `rg "github.com/stretchr/testify/(require|assert)" common/http --glob '*_test.go'` MUST 能定位到迁移后的实际使用点

### Requirement: common 测试断言统一迁移

`common/runtime` 和 `common/security` 的测试代码 MUST 优先使用 `testify/require` 表达可语义化的常见断言，包括错误返回、错误类型、对象和值相等性、nil、布尔条件、集合长度、字符串包含关系和状态检查。测试代码 MUST NOT 将历史手写失败判断机械替换为 `require.Fail`、`require.Failf`、`assert.Fail` 或 `assert.Failf`；当存在明确语义化断言方法时，MUST 使用对应的 `require` 或 `assert` 方法。

#### Scenario: 迁移常见断言

- **WHEN** `common/runtime` 或 `common/security` 的 `_test.go` 需要检查错误、对象状态、布尔条件、集合或字符串结果
- **THEN** 测试 MUST 使用 `require.NoError`、`require.Error`、`require.ErrorIs`、`require.Equal`、`require.True`、`require.False`、`require.Len`、`require.NotNil` 或等价语义化断言
- **AND** 测试 SHOULD NOT 使用 `t.Fatal`、`t.Fatalf`、`t.Error` 或 `t.Errorf` 表达这些常见断言

#### Scenario: 独立字段聚合诊断

- **WHEN** 单个测试需要在一次执行中收集多个相互独立字段、指标或统计值的失败信息
- **THEN** 测试 MAY 使用 `testify/assert` 进行非阻塞式断言
- **AND** 初始化失败、前置条件失败或后续检查依赖当前结果的场景 MUST 使用 `testify/require`

#### Scenario: 避免 Fail helper 机械替换

- **WHEN** 迁移历史手写失败判断
- **THEN** 测试 MUST NOT 使用 `require.Fail`、`require.Failf`、`require.FailNow`、`require.FailNowf`、`assert.Fail` 或 `assert.Failf` 替代可语义化表达的普通断言

#### Scenario: 特殊失败控制流例外

- **WHEN** 测试存在并发协调、panic/recovery、benchmark、goroutine 内控制流、测试框架边界或无法通过现有语义化断言清晰表达的特殊诊断
- **THEN** 测试 MAY 保留 `t.Fatal`、`t.Fatalf`、`t.Error`、`t.Errorf` 或 `Fail*` 用法
- **AND** 保留项 MUST 能通过代码上下文或实施任务清单说明其符合 `docs/TESTING.md` 的例外规则

#### Scenario: 不引入兼容 helper

- **WHEN** 统一 common 测试断言风格
- **THEN** 系统 MUST NOT 新增旧断言风格兼容 helper、双写断言 wrapper 或仅服务于断言迁移的生产代码

### Requirement: HTTP response helper wrapper 覆盖

系统 MUST 在 `common/http/response` 中为共享 HTTP response helper wrapper 保持直接单元测试覆盖，测试 MUST 锁定当前统一 response envelope、应用错误码、公开 message、HTTP status、`data` 或 `errors` 字段行为，并 MUST NOT 接受旧 envelope、旧错误消息格式、旧 helper alias 或旧 HTTP status 的兼容路径。

#### Scenario: 创建成功响应

- **WHEN** 调用方使用 `Created` 写入创建成功响应
- **THEN** 系统 MUST 返回 `201 Created`
- **AND** 响应 envelope MUST 为 `success=true`、`code=CodeOK`、`message=MessageCreated`
- **AND** 响应 envelope MUST 携带调用方传入的 `data`

#### Scenario: 无内容成功响应

- **WHEN** 调用方使用 `NoContent` 写入无内容成功响应
- **THEN** 系统 MUST 返回 `204 No Content`
- **AND** 响应 body MUST 为空

#### Scenario: 校验失败响应

- **WHEN** 调用方使用 `ValidationFailed` 写入字段语义校验失败响应
- **THEN** 系统 MUST 返回 `400 Bad Request`
- **AND** 响应 envelope MUST 为 `success=false`、`code=CodeValidationFailed`
- **AND** 响应 envelope MUST 使用调用方提供的公开 message

#### Scenario: 未认证响应

- **WHEN** 调用方使用 `Unauthenticated` 写入未认证响应
- **THEN** 系统 MUST 返回 `401 Unauthorized`
- **AND** 响应 envelope MUST 为 `success=false`、`code=CodeUnauthenticated`
- **AND** 响应 envelope MUST 使用调用方提供的公开 message

#### Scenario: 权限不足响应

- **WHEN** 调用方使用 `Forbidden` 写入权限不足响应
- **THEN** 系统 MUST 返回 `403 Forbidden`
- **AND** 响应 envelope MUST 为 `success=false`、`code=CodeForbidden`
- **AND** 响应 envelope MUST 使用调用方提供的公开 message

#### Scenario: 冲突响应

- **WHEN** 调用方使用 `Conflict` 写入领域冲突或资源状态冲突响应
- **THEN** 系统 MUST 返回 `409 Conflict`
- **AND** 响应 envelope MUST 为 `success=false`、`code=CodeConflict`
- **AND** 响应 envelope MUST 使用调用方提供的公开 message

#### Scenario: 未找到响应

- **WHEN** 调用方使用 `NotFound` 写入资源不存在响应
- **THEN** 系统 MUST 返回 `404 Not Found`
- **AND** 响应 envelope MUST 为 `success=false`、`code=CodeNotFound`
- **AND** 响应 envelope MUST 使用调用方提供的公开 message

### Requirement: 共享 CORS 默认入口覆盖

系统 MUST 在 `common/http/middleware` 中为共享 CORS 默认入口 `CORS()` 保持直接单元测试覆盖。测试 MUST 锁定当前默认策略：允许来源为 `*`，允许方法为 `GET,POST,PUT,PATCH,DELETE,OPTIONS`，允许请求头为 `Authorization,Content-Type`；测试 MUST 验证 `CORS()` 与 `CORSWithOptions(defaultCORSOptions)` 的外部响应行为一致，并 MUST NOT 接受旧 origin 反射默认值、旧 header、旧 wildcard+credentials 兼容行为或旧安全兼容开关。

#### Scenario: 默认响应头

- **WHEN** 普通 HTTP 请求经过 `CORS()` middleware
- **THEN** 响应 MUST 包含 `Access-Control-Allow-Origin=*`
- **AND** 响应 MUST 包含 `Access-Control-Allow-Methods=GET,POST,PUT,PATCH,DELETE,OPTIONS`
- **AND** 响应 MUST 包含 `Access-Control-Allow-Headers=Authorization,Content-Type`
- **AND** 默认响应 MUST NOT 包含 `Access-Control-Allow-Credentials`、`Access-Control-Max-Age`、`Access-Control-Expose-Headers` 或 `Vary: Origin`

#### Scenario: 默认预检短路

- **WHEN** `OPTIONS` 预检请求经过 `CORS()` middleware
- **THEN** 系统 MUST 返回 `204 No Content`
- **AND** 业务 handler MUST NOT 被继续调用
- **AND** 响应 MUST 继续包含当前默认 CORS 响应头

#### Scenario: 默认普通请求传递

- **WHEN** 非 `OPTIONS` 普通请求经过 `CORS()` middleware
- **THEN** 系统 MUST 继续调用后续业务 handler
- **AND** 业务 handler 写入的 HTTP status 和 body MUST 保持可见
- **AND** `CORS()` 的相关响应结果 MUST 与 `CORSWithOptions(defaultCORSOptions)` 一致

#### Scenario: CORS 测试断言风格

- **WHEN** 新增或修改 `common/http/middleware` 的 CORS 测试
- **THEN** 常见错误、状态、相等性、布尔条件、集合或字符串断言 MUST 使用语义化 `require` 或允许边界内的 `assert`
- **AND** 测试 MUST NOT 通过机械 `require.Fail`、`require.Failf`、`assert.Fail` 或 `assert.Failf` 替换常见断言

### Requirement: Go 测试断言依赖与例外规范

服务测试 MUST 可以直接使用标准 `testify/require` 与 `testify/assert` 断言库表达常见错误、对象、布尔、集合、字符串和诊断预期。系统 MUST NOT 为迁移历史测试断言新增跨服务兼容 helper、机械失败包装器或隐藏标准断言语义的共享抽象。

#### Scenario: 服务模块声明直接测试依赖

- **WHEN** 服务模块的测试代码直接导入 `github.com/stretchr/testify/require` 或 `github.com/stretchr/testify/assert`
- **THEN** 该 Go module MUST 在自身 `go.mod` 中直接声明 `github.com/stretchr/testify`
- **AND** `go mod tidy` 后依赖文件 MUST NOT 出现与本次测试断言迁移无关的漂移

#### Scenario: 优先使用语义化断言

- **WHEN** 测试需要验证错误、对象和值、布尔状态、集合长度、字符串内容或 nil 状态
- **THEN** 测试 MUST 优先使用 `require.NoError`、`require.Error`、`require.ErrorIs`、`require.Equal`、`require.NotEqual`、`require.Nil`、`require.NotNil`、`require.True`、`require.False`、`require.Len`、`require.Empty`、`require.NotEmpty` 或 `require.Contains` 等语义化断言
- **AND** 测试 MUST NOT 将普通手写失败判断机械替换为 `require.Fail`、`require.Failf`、`assert.Fail` 或 `assert.Failf`

#### Scenario: 保留直接 testing.T 失败调用的例外

- **WHEN** 测试保留 `t.Fatal`、`t.Fatalf`、`t.Error` 或 `t.Errorf`
- **THEN** 该调用 MUST 用于无法通过现有语义化断言清晰表达的自定义测试控制流、特殊诊断输出或不适合依赖 `testify` 的测试辅助工具
- **AND** 普通前置条件失败、错误返回值、相等性、包含关系、长度、空值或布尔状态断言 MUST 使用 `require` 或必要时 `assert`

### Requirement: 服务测试断言依赖与例外规范

服务测试 MUST 可以直接使用标准 `testify/require` 与 `testify/assert` 断言库表达常见错误、对象、布尔、集合、字符串和诊断预期。系统 MUST NOT 为迁移 user 与 shared identity 历史测试断言新增跨服务兼容 helper、机械失败包装器或隐藏标准断言语义的共享抽象。

#### Scenario: 服务模块声明直接测试依赖

- **WHEN** 服务模块的测试代码直接导入 `github.com/stretchr/testify/require` 或 `github.com/stretchr/testify/assert`
- **THEN** 该 Go module MUST 在自身 `go.mod` 中直接声明 `github.com/stretchr/testify`
- **AND** `go mod tidy` 后依赖文件 MUST NOT 出现与本次测试断言迁移无关的漂移

#### Scenario: 优先使用语义化断言

- **WHEN** 测试需要验证错误、对象和值、布尔状态、集合长度、字符串内容、类型、nil 状态、HTTP response 字段或 pagination 字段
- **THEN** 测试 MUST 优先使用 `require.NoError`、`require.Error`、`require.ErrorIs`、`require.Equal`、`require.NotEqual`、`require.Nil`、`require.NotNil`、`require.True`、`require.False`、`require.Len`、`require.Empty`、`require.NotEmpty`、`require.Contains` 或等价语义化断言
- **AND** 测试 MUST NOT 将普通手写失败判断机械替换为 `require.Fail`、`require.Failf`、`assert.Fail` 或 `assert.Failf`

#### Scenario: 多字段响应诊断可以使用 assert

- **WHEN** 单个 HTTP response、pagination 或 DTO 测试需要同时收集多个互不依赖字段的失败信息
- **THEN** 测试 MAY 使用 `testify/assert` 验证这些独立字段
- **AND** 任何会影响后续解码、类型断言或字段访问安全性的前置条件 MUST 使用 `require`

#### Scenario: 保留直接 testing.T 失败调用的例外

- **WHEN** 测试保留 `t.Fatal`、`t.Fatalf`、`t.Error` 或 `t.Errorf`
- **THEN** 该调用 MUST 用于无法通过现有语义化断言清晰表达的自定义测试控制流、特殊诊断输出或不适合依赖 `testify` 的测试辅助工具
- **AND** 普通前置条件失败、错误返回值、相等性、包含关系、长度、空值或布尔状态断言 MUST 使用 `require` 或必要时 `assert`

### Requirement: 服务装配边界测试断言依赖与例外治理

服务装配边界测试 MUST 可以直接使用标准 `testify/require` 与 `testify/assert` 表达常见错误、对象、数值范围、集合、字符串、JSON、正则、时间和 panic 断言。系统 MUST NOT 为迁移 router、provider 或 bootstrap 历史测试断言新增跨服务兼容 helper、机械失败包装器、共享断言 facade 或仅服务于测试的生产 API。

#### Scenario: 直接使用标准 testify 断言

- **WHEN** `user-service/internal/router`、`providers` 或 `bootstrap` 测试需要验证错误、对象和值、数值范围、集合长度、元素集合、字符串包含、JSON 等价、正则匹配、时间边界或 panic 行为
- **THEN** 测试 MUST 优先使用 `require.NoError`、`require.Error`、`require.ErrorContains`、`require.Equal`、`require.NotNil`、`require.Len`、`require.Greater`、`require.Less`、`require.ElementsMatch`、`require.JSONEq`、`require.Regexp`、`require.WithinDuration`、`require.Panics` 或等价语义化断言
- **AND** 测试 MUST NOT 使用 `require.True`、`require.False`、手写 `if` 或多个基础断言拼凑上述已有语义化断言可以清晰覆盖的检查

#### Scenario: 多个独立检查可使用 assert

- **WHEN** 单个测试需要在一次执行中收集多个互相独立的 route、provider 输出、metric family、label、日志字段或 health check 结果失败
- **THEN** 测试 MAY 使用 `testify/assert` 进行非阻塞式断言
- **AND** 初始化失败、前置条件失败或后续检查依赖当前结果时 MUST 使用 `testify/require`

#### Scenario: 禁止新增断言兼容层

- **WHEN** 迁移历史 `t.Fatal`、`t.Error` 或泛化布尔断言
- **THEN** 系统 MUST NOT 新增旧断言风格兼容 helper、共享 wrapper、机械 `Fail*` 替换、测试专用生产分支或仅为单元测试暴露的运行时 API
- **AND** 迁移 MUST 基于现有实现和合理的测试可读性完成

#### Scenario: testing.T 直接失败例外

- **WHEN** 目标测试保留直接 `testing.T` 失败方法或 `Fail*` 调用
- **THEN** 保留项 MUST 符合 `docs/TESTING.md` 中自定义测试控制流、特殊诊断输出或测试辅助工具不适合依赖 `testify` 的例外规则
- **AND** 普通错误、相等性、包含关系、长度、空值、数值范围、字符串、JSON、正则、时间或 panic 断言 MUST 使用语义化 `require` 或必要时 `assert`

### Requirement: cmd 与 Ent schema 测试断言依赖与例外治理

cmd 与 Ent schema 测试 MUST 可以直接使用标准 `testify/require` 与 `testify/assert` 表达常见错误、对象、数值范围、集合、字符串、JSON、正则、时间和 panic 断言。系统 MUST NOT 为迁移 CLI 或 Ent schema 历史测试断言新增跨服务兼容 helper、机械失败包装器、共享断言 facade 或仅服务于测试的生产 API。

#### Scenario: 直接使用标准 testify 断言

- **WHEN** `user-service/cmd` 或 `user-service/ent/schema` 测试需要验证错误、对象和值、数值范围、集合长度、元素集合、字符串包含、JSON 等价、正则匹配、时间边界或 panic 行为
- **THEN** 测试 MUST 优先使用 `require.NoError`、`require.Error`、`require.ErrorContains`、`require.Equal`、`require.NotNil`、`require.Len`、`require.Greater`、`require.Less`、`require.ElementsMatch`、`require.JSONEq`、`require.Regexp`、`require.WithinDuration`、`require.Panics` 或等价语义化断言
- **AND** 测试 MUST NOT 使用 `require.True`、`require.False`、手写 `if` 或多个基础断言拼凑上述已有语义化断言可以清晰覆盖的检查

#### Scenario: 多个独立检查可使用 assert

- **WHEN** 单个测试需要在一次执行中收集多个互相独立的 command property、flag metadata、schema field、schema edge、schema index、annotation 或 validator 结果失败
- **THEN** 测试 MAY 使用 `testify/assert` 进行非阻塞式断言
- **AND** 初始化失败、前置条件失败或后续检查依赖当前结果时 MUST 使用 `testify/require`

#### Scenario: 禁止新增断言兼容层

- **WHEN** 迁移历史 `t.Fatal`、`t.Error` 或泛化布尔断言
- **THEN** 系统 MUST NOT 新增旧断言风格兼容 helper、共享 wrapper、机械 `Fail*` 替换、测试专用生产分支或仅为单元测试暴露的运行时 API
- **AND** 迁移 MUST 基于现有实现和合理的测试可读性完成

#### Scenario: testing.T 直接失败例外

- **WHEN** 目标测试保留直接 `testing.T` 失败方法或 `Fail*` 调用
- **THEN** 保留项 MUST 符合 `docs/TESTING.md` 中自定义测试控制流、特殊诊断输出或测试辅助工具不适合依赖 `testify` 的例外规则
- **AND** 普通错误、相等性、包含关系、长度、空值、数值范围、字符串、JSON、正则、时间或 panic 断言 MUST 使用语义化 `require` 或必要时 `assert`

### Requirement: runtime primitive 内部默认值可追踪

系统 MUST 在 `common/runtime` 和 `common/testing` 中使用命名常量表达包内默认超时、轮询间隔和探测间隔，避免在核心执行路径或测试基础设施中保留难以追踪的内联时间魔法值。命名常量 MUST 保持私有，除非该值已经是明确的跨模块公开契约。

#### Scenario: scheduler 锁默认超时命名化

- **WHEN** scheduler 需要为锁释放或锁续租设置内部默认超时
- **THEN** 系统 MUST 通过 `common/runtime/scheduler` 包内私有命名常量表达该默认值
- **AND** `executor.go`、`renew.go` 和 `validation.go` MUST NOT 分别内联重复的 `5 * time.Second` 默认值

#### Scenario: 测试容器探测间隔命名化

- **WHEN** `common/testing/containers` 需要轮询 Docker mapped port 或依赖 readiness
- **THEN** 系统 MUST 通过测试 helper 包内私有命名常量表达探测或轮询间隔
- **AND** PostgreSQL 测试容器 helper MUST NOT 在端口探测循环中直接内联 `100 * time.Millisecond`

### Requirement: scheduler 任务执行流程可维护

系统 MUST 保持 `common/runtime/scheduler` 的任务执行流程职责清晰。`runJob()` MUST 作为一次任务触发的编排入口，核心子流程 MUST 通过私有函数承载执行权获取、分布式锁获取、任务上下文和续租准备、执行后 cleanup、执行结果记录，且 MUST 保持导出 API 和运行时行为不变。

#### Scenario: 拆分任务执行子流程

- **WHEN** scheduler 执行一次已注册任务
- **THEN** `runJob()` MUST 继续按本地 overlap gate、全局并发 gate、分布式锁、任务上下文、自动续租、任务执行和收尾记录的顺序编排
- **AND** 各子流程 MUST 由 `common/runtime/scheduler` 包内私有函数或私有类型承载

#### Scenario: 保持任务执行语义

- **WHEN** `runJob()` 被拆分为私有函数
- **THEN** 系统 MUST 保持任务触发、跳过原因、开始、完成、失败、panic recovery、锁释放、续租失败处理、gate 归还和 shutdown 语义不变
- **AND** 系统 MUST NOT 新增公开 executor 类型、公开接口或仅服务测试的生产适配层

### Requirement: common 模块依赖保持 tidy

系统 MUST 保持 `common` 模块依赖图与当前源码和工具入口一致。`common/go.mod` 和 `common/go.sum` MUST 通过 `GOWORK=off go mod tidy` 校验，不得手工保留当前模块不再需要的间接依赖残留。

#### Scenario: common 依赖清理

- **WHEN** `common` 模块完成 runtime primitive 或测试基础设施维护性变更
- **THEN** 系统 MUST 在 `common` 目录使用 `GOWORK=off go mod tidy` 整理依赖
- **AND** `common/go.mod` 和 `common/go.sum` MUST 只保留 Go 工具链按当前源码、测试和 tool 指令判定需要的模块项

#### Scenario: 不误删真实导入链依赖

- **WHEN** 某个间接依赖由 Gin、Swagger UI、Prometheus 或其他当前源码真实导入链带入
- **THEN** 系统 MUST 以 `go mod why -m` 和 `go mod tidy -diff` 结果为准判断是否清理
- **AND** 系统 MUST NOT 为降低依赖数量而手工删除 tidy 仍要求保留的模块项

### Requirement: 共享 runtime primitive 测试稳定性

`common/runtime` 中 localcache、workerpool、scheduler 和 timezone 等共享 runtime primitive 的测试 MUST 避免使用固定 `time.Sleep` 或手动 `os.Setenv` 恢复来表达异步进度、过期状态或全局环境隔离。测试 MUST 使用可观察条件、通道同步、testing 环境隔离或确定性输入表达预期。

#### Scenario: localcache 过期测试使用条件等待
- **WHEN** localcache 测试验证 TTL 过期后缓存未命中
- **THEN** 测试 MUST 使用 `require.Eventually` 或等价条件等待断言未命中状态
- **AND** 测试 MUST NOT 在固定 `time.Sleep` 后立即断言过期结果

#### Scenario: localcache 并发回源测试使用通道同步
- **WHEN** localcache 测试验证同 key 并发 miss 被 `singleflight` 合并
- **THEN** 测试 MUST 通过通道、atomic 计数或 wait group 明确确认 goroutine 已进入目标等待点
- **AND** 测试 MUST NOT 依赖固定 `time.Sleep` 猜测 goroutine 调度状态

#### Scenario: workerpool 状态等待使用条件断言
- **WHEN** workerpool 测试等待任务进入 running、waiting、completed、failed 或 stopped 状态
- **THEN** 测试 MUST 使用条件等待 helper、`require.Eventually` 或通道信号
- **AND** 测试 MUST NOT 使用固定 `time.Sleep` 表达后台状态已经变化

#### Scenario: scheduler 自动续租测试使用可观察条件
- **WHEN** scheduler 测试验证自动续租或任务取消行为
- **THEN** 测试 MUST 通过锁记录、任务通道或 eventually-style 条件断言观察续租结果
- **AND** 测试 MUST NOT 仅通过任务内部固定 sleep 制造续租窗口

#### Scenario: timezone 测试隔离全局环境
- **WHEN** timezone 测试修改 `TZ`、`time.Local` 或包级初始化状态
- **THEN** 测试 MUST 使用 `t.Setenv` 管理环境变量
- **AND** 测试 MUST 通过 `t.Cleanup` 恢复 `time.Local` 和包级状态
- **AND** 这些测试 MUST NOT 使用 `t.Parallel`

### Requirement: Password KDF busy 应用错误契约

`common/security/password` MUST 将密码 KDF 资源繁忙表达为业务中立的应用错误，使调用方可通过共享 `response.Fail` 直接渲染为服务不可用响应，同时保持 Argon2id 参数、哈希编码、并发上限、队列上限和 `errors.Is(err, password.ErrPasswordKDFBusy)` 语义不变。

#### Scenario: KDF busy 直接渲染为服务不可用

- **WHEN** `common/security/password` 的 KDF 服务实例因为执行中和等待中的请求数达到实例资源预算而返回 `password.ErrPasswordKDFBusy`
- **THEN** 该错误 MUST 携带 `KindServiceUnavailable`、稳定 `Reason` 值 `password_kdf_busy`、`CodeServiceUnavailable` 和不泄露资源预算的中文公开 message
- **AND** `common/http/response.Fail` MUST 能通过 `contracterrors.FromError` 直接渲染该错误为 `503 Service Unavailable`
- **AND** 该错误 MUST NOT 要求 user-service auth HTTP mapper 才能获得服务不可用语义

#### Scenario: KDF busy 保持 errors.Is 语义

- **WHEN** 调用方通过 `errors.Is(err, password.ErrPasswordKDFBusy)` 判断 KDF 资源繁忙
- **THEN** 直接返回的错误和被包装后的错误 MUST 继续支持正确匹配
- **AND** 该匹配语义 MUST NOT 依赖 HTTP transport 层的错误转换函数

#### Scenario: password primitive 保持业务中立

- **WHEN** `common/security/password` 定义 `password.ErrPasswordKDFBusy`
- **THEN** `common` MUST 只表达密码 KDF 资源预算繁忙这一业务中立语义
- **AND** `common` MUST NOT 承载 user-service 登录、用户名、认证会话、token、强制改密、撤销或认证公开消息以外的业务编排逻辑

#### Scenario: KDF 安全语义不变

- **WHEN** password KDF busy 错误迁移为应用错误
- **THEN** 系统 MUST NOT 改变 Argon2id 参数、哈希编码、队列上限、并发上限、常量时间校验或资源预算触发条件
- **AND** 测试 MUST 继续覆盖队列繁忙路径、哈希成功路径和密码校验失败路径

### Requirement: Feature-local auth domain 应用错误契约

系统 MUST 允许 user-service auth domain 为稳定认证业务错误定义应用错误，使这些错误同时携带 `Kind`、`Reason`、`Code`、`Message` 并可被共享 response helper 直接渲染。该契约 MUST 保持业务归属清晰，且不得把 user-service 的认证错误映射表上移到 `common`、`internal/shared` 或跨 feature 全局包。

#### Scenario: auth domain 定义可渲染应用错误

- **WHEN** `user-service/internal/features/auth/domain` 定义无效凭据、缺失会话、token 无效、refresh session 无效、强制改密 session 无效或撤销不完整错误
- **THEN** 错误 MUST 携带共享错误契约所需的 `Kind`、稳定 `Reason`、`Code` 和中文公开 `Message`
- **AND** `common/http/response.Fail` MUST 能通过 `contracterrors.FromError` 直接渲染该错误

#### Scenario: auth domain 保持业务判断语义

- **WHEN** 调用方通过 `errors.Is` 判断 auth domain 导出的稳定业务错误
- **THEN** 直接返回的错误和被包装后的错误 MUST 继续支持正确匹配
- **AND** 该匹配语义 MUST NOT 依赖 HTTP transport 层的错误转换函数

#### Scenario: common 保持认证业务中立

- **WHEN** auth domain 错误迁移为应用错误
- **THEN** `common/` MUST 只提供业务中立的错误契约、应用错误构造、password primitive 和 response 渲染 helper
- **AND** 系统 MUST NOT 在 `common` 新增 user-service 认证专用错误映射表、登录编排、session 语义、token version 语义或 Redis key schema

#### Scenario: 不新增跨模块认证错误注册表

- **WHEN** feature-local auth domain 应用错误需要被 HTTP controller 渲染
- **THEN** controller MUST 通过共享 `response.Fail` 和错误自身携带的契约信息完成渲染
- **AND** 系统 MUST NOT 新增跨模块认证错误映射注册表、compat mapper 或仅包装 `contracterrors.FromError` 的认证错误兼容函数

### Requirement: Feature-local domain 应用错误契约

系统 MUST 允许 user-service feature-local domain 为稳定业务错误定义应用错误，使这些错误同时携带 `Kind`、`Reason`、`Code`、`Message` 并可被共享 response helper 直接渲染。该契约 MUST 保持业务归属清晰，且不得把 user-service 的权限错误映射表上移到 `common`、`internal/shared` 或跨 feature 全局包。

#### Scenario: permission domain 定义可渲染应用错误

- **WHEN** `user-service/internal/features/permission/domain` 定义权限已存在、权限不存在、权限输入无效或系统权限保护错误
- **THEN** 错误 MUST 携带共享错误契约所需的 `Kind`、稳定 `Reason`、`Code` 和中文公开 `Message`
- **AND** `common/http/response.Fail` MUST 能通过 `contracterrors.FromError` 直接渲染该错误

#### Scenario: feature-local domain 保持业务判断语义

- **WHEN** 调用方通过 `errors.Is` 判断 permission domain 导出的稳定业务错误
- **THEN** 直接返回的错误和被包装后的错误 MUST 继续支持正确匹配
- **AND** 该匹配语义 MUST NOT 依赖 HTTP transport 层的错误转换函数

#### Scenario: common 保持业务中立

- **WHEN** 权限目录错误迁移为应用错误
- **THEN** `common/` MUST 只提供业务中立的错误契约、应用错误构造和 response 渲染 helper
- **AND** 系统 MUST NOT 在 `common` 新增权限目录专用 `Reason` 常量、公开消息、错误变量或权限错误到 HTTP 响应的全局映射表

#### Scenario: 不新增跨模块权限错误注册表

- **WHEN** feature-local domain 应用错误需要被 HTTP controller 渲染
- **THEN** controller MUST 通过共享 `response.Fail` 和错误自身携带的契约信息完成渲染
- **AND** 系统 MUST NOT 新增跨模块权限错误映射注册表、compat mapper 或仅包装 `contracterrors.FromError` 的权限错误兼容函数

### Requirement: Feature-local role 应用错误契约

系统 MUST 允许 user-service feature-local role domain 为稳定角色和 RBAC 绑定错误定义应用错误，使这些错误同时携带 `Kind`、`Reason`、`Code`、`Message` 并可被共享 response helper 直接渲染。该契约 MUST 保持业务归属清晰，且不得把 role、permission 或 identity 的错误映射表上移到 `common`、`internal/shared` 或跨 feature 全局包。

#### Scenario: role domain 定义可渲染应用错误

- **WHEN** `user-service/internal/features/role/domain` 定义角色目录、用户角色绑定或角色权限绑定错误
- **THEN** 错误 MUST 携带共享错误契约所需的 `Kind`、稳定 `Reason`、`Code` 和中文公开 `Message`
- **AND** `common/http/response.Fail` MUST 能通过 `contracterrors.FromError` 直接渲染该错误

#### Scenario: role domain 保持业务判断语义

- **WHEN** 调用方通过 `errors.Is` 判断 role domain 导出的稳定业务错误
- **THEN** 直接返回的错误和被包装后的错误 MUST 继续支持正确匹配
- **AND** 该匹配语义 MUST NOT 依赖 HTTP transport 层的错误转换函数

#### Scenario: 消费方 feature 透传应用错误

- **WHEN** role feature 接收到 `identity` 或 `permission` 边界拥有的应用错误
- **THEN** role feature MAY 返回或包装该错误并交给共享 response helper 渲染
- **AND** role HTTP transport MUST NOT 为这些跨 feature 错误维护重复 sentinel-to-HTTP 映射

#### Scenario: common 保持业务中立

- **WHEN** 角色与 RBAC 绑定错误迁移为应用错误
- **THEN** `common/` MUST 只提供业务中立的错误契约、应用错误构造和 response 渲染 helper
- **AND** 系统 MUST NOT 在 `common` 新增角色、用户角色绑定、角色权限绑定、permission 或 identity 专用 `Reason` 常量、公开消息、错误变量或错误到 HTTP 响应的全局映射表

#### Scenario: 不新增跨模块角色错误注册表

- **WHEN** feature-local role 应用错误或跨 feature 应用错误需要被 role HTTP controller 渲染
- **THEN** controller MUST 通过共享 `response.Fail` 和错误自身携带的契约信息完成渲染
- **AND** 系统 MUST NOT 新增跨模块角色错误映射注册表、compat mapper 或仅包装 `contracterrors.FromError` 的角色错误兼容函数

### Requirement: 服务内 shared identity 应用错误契约

系统 MUST 允许服务内 shared kernel 为稳定身份错误定义应用错误，使这些错误同时携带 `Kind`、`Reason`、`Code`、`Message` 并可被共享 response helper 直接渲染。该契约 MUST 保持服务内业务归属清晰，且不得把 user-service 的用户错误映射表上移到 `common` 或跨 feature 全局包。

#### Scenario: shared identity 定义可渲染应用错误

- **WHEN** `user-service/internal/shared/identity` 定义用户不存在或用户已存在错误
- **THEN** 错误 MUST 携带共享错误契约所需的 `Kind`、`Reason`、`Code` 和公开 `Message`
- **AND** `common/http/response.Fail` MUST 能通过 `contracterrors.FromError` 直接渲染该错误

#### Scenario: shared identity 保持业务判断语义

- **WHEN** 调用方通过 `errors.Is` 判断 `identity.ErrUserNotFound` 或 `identity.ErrUserAlreadyExists`
- **THEN** 直接返回的错误和被包装后的错误 MUST 继续支持正确匹配

#### Scenario: common 不承载用户错误映射

- **WHEN** 用户身份错误迁移为应用错误
- **THEN** `common/` MUST 只提供业务中立的错误契约、应用错误构造和 response 渲染 helper
- **AND** 系统 MUST NOT 在 `common` 或用户 feature 外新增用户错误到 HTTP 响应的全局映射表
