## MODIFIED Requirements

### Requirement: Runtime 执行原语
系统 MUST 在 `common/runtime` 中提供业务中立的 ID、scheduler、workerpool、localcache、Redis key 和 timezone 原语，并 MUST 为后台执行提供明确的容量、并发、失败处理、观测和关闭语义。

#### Scenario: workerpool 生命周期和边界
- **WHEN** 调用方通过 `workerpool.New` 创建任务池并通过 `Stop(ctx)` 关闭
- **THEN** task pool MUST 作为不依赖 Fx 的普通 Go 资源创建，由拥有者显式关闭
- **AND** Stop MUST 停止接收新任务、等待已登记或已接受任务 drain，并允许重复调用共享同一 drain 状态
- **AND** Stop 超时 MUST 返回包装 `context.DeadlineExceeded` 的错误
- **AND** workerpool MUST NOT 承载 refresh session、token version、可靠消息、eventbus、outbox 或业务一致性语义

#### Scenario: scheduler 执行任务
- **WHEN** scheduler 触发已注册任务
- **THEN** 系统 MUST 按本地 overlap gate、全局并发 gate、可选分布式锁、任务 context、可选锁续租、任务执行和 cleanup 的顺序处理
- **AND** 系统 MUST 记录跳过、开始、完成、失败、拒绝和 panic，并在 shutdown 时优雅停止
- **AND** 多实例副作用任务 MUST 声明正数 TTL 的分布式锁策略，长任务 SHOULD 使用续租
- **AND** 即使任务未配置 timeout，scheduler MUST 创建可取消 context，并在自动续租失败时取消任务和记录失败

#### Scenario: 创建本地 loading cache
- **WHEN** 服务通过 `NewLoadingCache` 创建 loading cache
- **THEN** 配置 MUST 只包含非空名称、正数 `uint64` 容量、正数固定 TTL 和正数 load timeout，并 MUST 提供 loader
- **AND** 容量 MUST 表示最大 item 数，不得表示字节、自定义 cost 或 Ristretto admission 参数
- **AND** cache key MUST 保留调用方选择的 comparable 类型，common MUST NOT 要求业务 key 字符串编码
- **AND** 公开 API MUST NOT 暴露底层 `ttlcache` 配置、独立 `Get`、主动 `Set`、`CloneFunc` 或写入拒绝语义

#### Scenario: 本地缓存读取与回源
- **WHEN** `GetOrLoad` 命中未过期 item
- **THEN** cache MUST 返回该值并记录一次 hit，且读取 MUST NOT 延长该 item 的固定 TTL
- **WHEN** `GetOrLoad` 未命中
- **THEN** cache MUST 为每个调用记录一次 miss，并使用 `singleflight` 合并同一业务 key 的并发回源
- **AND** loader 成功 MUST 记录 `LoadSuccess` 并同步写入 bounded TTL cache，loader 失败 MUST 记录 `LoadError` 且不得缓存错误结果
- **AND** 内部 double-check MAY 避免重复回源，但 MUST NOT 计为业务 hit 或成为公开统计字段

#### Scenario: loader context 与 caller 取消
- **WHEN** 同 key 回源正在执行且任一 caller context 被取消
- **THEN** 该 caller MUST 返回其 context error，MUST NOT 因自身取消而终止其他等待者共享的 loader
- **AND** loader context MUST 保留发起请求的 context values、解除 caller cancellation，并受配置的 `LoadTimeout` 限制

#### Scenario: value ownership 与失效
- **WHEN** cache 存储或返回 slice、map、pointer 或包含引用字段的 value
- **THEN** common MUST NOT 执行业务 deep clone，消费 feature MUST 在 loader 写入和返回调用方的适当边界复制可变 value
- **WHEN** 调用方执行 `Delete` 或 `Clear`
- **THEN** 对应 item 或全部 item MUST 在方法成功返回时失效，显式失效 MUST NOT 计入自动 eviction

#### Scenario: 本地缓存关闭
- **WHEN** 调用方一次或多次执行 `Close`
- **THEN** cache MUST 幂等停止 wrapper 拥有的 TTL 清理 goroutine 并阻止新的底层访问
- **AND** 关闭后的 `GetOrLoad`、`Delete` 和 `Clear` MUST 返回 `ErrClosed`
- **AND** 已开始的 loader MAY 在 load timeout 内完成，但关闭后 MUST NOT 再写入底层 cache
- **AND** `Name` 和 `Stats` MUST 在关闭后继续返回稳定名称与最后累计快照

#### Scenario: Redis key 和 timezone 归属
- **WHEN** feature 需要 refresh session、token version、RBAC 或其他业务 Redis key
- **THEN** feature infrastructure MUST 拥有业务 key schema，并只能复用 `common/runtime/rediskey` 的通用构造规则
- **WHEN** runtime 初始化进程时区
- **THEN** timezone primitive MUST 优先使用平台 `TZ` 环境变量并在缺省时使用稳定默认值
- **AND** timezone primitive MUST NOT 依赖核心 Config 或服务业务配置
- **AND** 如果 timezone 初始化通过 Fx 执行，拥有进程启动语义的服务 composition root MUST 显式绑定初始化调用或服务级 runtime 初始化函数，common MUST NOT 仅为了包装 `Init` 暴露无额外运行时职责的 Fx provider

### Requirement: Logger 与共享 Fx 装配边界
系统 MUST 在 `common/runtime` 中提供业务中立的 logger、Fx provider 和依赖图原语。构造函数、provider 和 Fx graph helper MUST 只消费真实运行时依赖或调用方显式提供的无副作用 Fx option，MUST NOT 为测试便利暴露生产 API 或读取服务私有配置。公开 provider 名称 MUST 表达其提供的 runtime 能力或资源职责，MUST NOT 仅用模糊的 DI framework 术语隐藏能力语义。

#### Scenario: logger 构造无全局副作用
- **WHEN** 调用方通过 `logger.New`、`NewWithConfig` 或 Fx provider `NewLogger` 创建 logger
- **THEN** 系统 MUST 返回由调用方拥有的 logger，Fx provider MUST 注册既有 Sync 关闭 hook
- **AND** 构造过程 MUST NOT 隐式安装、覆盖或恢复进程级默认 logger
- **AND** 默认 logger 只能通过显式 `SetDefault` 修改，并 MAY 作为未注入 logger 时的兜底

#### Scenario: 共享 provider 和 fxgraph 边界
- **WHEN** 共享 provider 暴露依赖
- **THEN** provider MUST 只消费跨服务配置和 primitive，不得导入服务私有配置
- **AND** provider 的公开命名 MUST 能从调用点区分 logger、metrics、tracing、datastore 或其他具体 runtime 能力，不得在多个 common 包中重复使用缺少能力语义的通用名称作为主要入口
- **WHEN** 服务将 Fx option 或 module 传入 `common/runtime/fxgraph`
- **THEN** helper MUST 输出稳定排序的 provider、invoke 和依赖关系图文本
- **AND** helper MUST 只处理调用方显式传入的 graph-safe Fx option，MUST NOT 构造或要求服务私有配置、feature provider、Ent、Redis、PostgreSQL、OTLP 或 HTTP server 输入
- **AND** helper MUST NOT 通过服务完整 runtime module 间接执行生产 runtime `fx.Invoke`

#### Scenario: 公开 API 具有运行时职责
- **WHEN** `common/runtime` 新增公开 constructor、method、option 或 hook
- **THEN** 入口 MUST 具有真实运行时职责或已定义的稳定共享契约
- **AND** 仅测试消费、暴露内部状态或绕过正常 lifecycle 的能力 MUST 留在包内、`_test.go` fixture 或 `common/testing`
- **AND** 仅包装另一个无参初始化函数且不提供额外资源、配置、错误处理、顺序控制或 lifecycle 语义的 Fx provider MUST NOT 作为 common 公开 API 新增或保留
