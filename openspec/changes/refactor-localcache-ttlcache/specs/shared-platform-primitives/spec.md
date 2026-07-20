## MODIFIED Requirements

### Requirement: Runtime 配置加载与服务配置边界

系统 MUST 在 `common/runtime/config` 中维护跨服务 runtime 配置、默认值和通用校验。服务私有业务配置、必需资源名、业务用途和配置 map 到真实资源的选择 MUST 由消费服务拥有。

#### Scenario: 严格加载通用配置

- **WHEN** 服务通过配置文件启动
- **THEN** 共享 loader MUST 解析 runtime、HTTP、gRPC、metrics、tracing、pprof、logger 和通用 `local_cache` 配置
- **AND** 系统 MUST 使用 `github.com/go-viper/mapstructure/v2` 的 decode 能力解析 duration、slice 和具名配置
- **AND** 未声明字段 MUST 在启动前失败并报告完整路径，不得使用旧字段别名或 fallback

#### Scenario: 通用 runtime 字段和安全校验

- **WHEN** 服务加载 runtime 配置
- **THEN** 共享 runtime config MUST 声明并校验 `runtime.gin.mode`、server、logger、metrics、tracing、pprof、lifecycle 和通用 local cache 配置
- **AND** `runtime.gin.mode` 默认值 MUST 为 `release`，环境变量覆盖 MUST 使用 `AEGISCORE_RUNTIME_GIN_MODE`，合法值 MUST 仅为 `debug`、`release` 或 `test`
- **AND** `observability.pprof.enabled` 和 `observability.pprof.addr` 默认值 MUST 分别为 `false` 和 `127.0.0.1:6060`
- **AND** production-like 环境启用 pprof 时 `observability.pprof.addr` MUST 使用 loopback host
- **AND** 至少一个 HTTP 或 gRPC server MUST 启用

#### Scenario: 服务私有配置留在服务边界

- **WHEN** 服务需要 `auth`、`ent`、JWT TTL、refresh session、token version、RBAC 或 production-like secret 校验
- **THEN** 服务私有 loader MUST 负责解析和校验这些配置
- **AND** `common/runtime/config` MUST NOT 声明或校验这些业务配置
- **AND** 服务私有配置 MUST NOT 声明、读取或兼容旧 `auth.password_kdf` 配置

#### Scenario: 通用具名本地缓存配置

- **WHEN** 配置包含 `local_cache.<name>`
- **THEN** loader MUST 保留 `<name>` 并解析为通用缓存实例配置
- **AND** validation MUST 校验 `capacity > 0`、`ttl > 0` 和 `load_timeout > 0`，错误 MUST 包含完整字段路径
- **AND** 配置契约 MUST NOT 暴露 Ristretto 的 `num_counters`、`buffer_items`、admission 或 write buffer 选项
- **AND** 必需缓存名及其业务含义 MUST 留在消费服务

#### Scenario: Runtime lifecycle 停止预算校验

- **WHEN** 配置中的 `runtime.lifecycle.stop_timeout` 小于 HTTP shutdown timeout、worker drain allowance、tracing flush allowance 和 shutdown safety margin 的组合最低预算
- **THEN** 配置校验 MUST 失败并指出 `runtime.lifecycle.stop_timeout` 以及最低所需预算
- **WHEN** 配置中的 `runtime.lifecycle.stop_timeout` 大于或等于组合最低预算
- **THEN** 共享 runtime 配置校验 MUST 继续通过，且业务停止策略 MUST 由 owning feature 或服务组合层表达

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
