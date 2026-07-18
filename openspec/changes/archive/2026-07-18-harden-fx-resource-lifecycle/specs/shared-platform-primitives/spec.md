## ADDED Requirements

### Requirement: Fx adapter 不把 stop hook 当作 constructor rollback

系统 MUST 在 `common/runtime` 的 Fx adapter 中区分构造、启动和停止职责。带网络连接、后台 goroutine、定时清理或批处理行为的共享 runtime 资源 MUST 优先在 `OnStart` 创建并在 `OnStop` 关闭；constructor 阶段若已创建需要关闭的部分资源，后续失败时 MUST 立即清理。

#### Scenario: OnStart 创建的资源随启动失败关闭
- **WHEN** 某个共享 runtime Fx adapter 的 `OnStart` 已创建 Redis client、workerpool、localcache 或 tracing 相关运行资源
- **AND** 后续 `OnStart` hook 返回错误导致 App 启动失败
- **THEN** 已启动资源 MUST 通过 Fx 停止流程关闭
- **AND** 资源关闭 MUST 不依赖 constructor 中登记但从未进入启动状态的 rollback 假设

#### Scenario: constructor 部分失败即时清理
- **WHEN** 共享 runtime constructor 必须在构造阶段创建一个或多个需要关闭的资源
- **AND** 后续配置投影、探测或包装步骤失败
- **THEN** constructor MUST 在返回错误前关闭已经创建的资源
- **AND** 错误链 MUST 保留构造失败和关闭失败

### Requirement: Redis Fx 生命周期启动安全

系统 MUST 在 Redis Fx adapter 中保持单资源创建语义，并保证启动探测失败、后续启动失败和正常停止路径均关闭同一个 Redis client。普通 Go constructor MUST 继续保持框架无关。

#### Scenario: Redis 启动探测失败关闭 client
- **WHEN** Redis Fx adapter 创建 client 后执行启动 PING 失败
- **THEN** adapter MUST 关闭该 client
- **AND** 返回错误 MUST 同时保留 PING 失败和关闭失败信息

#### Scenario: Redis client 不被 feature 自有资源关闭
- **WHEN** auth、permission 或其他 feature 关闭自身 workerpool、watcher、cache 或 store
- **THEN** feature 自有资源 MUST NOT 关闭共享 Redis client
- **AND** 共享 Redis client MUST 只由服务资源层生命周期关闭

### Requirement: 主动后台 primitive 显式关闭

系统 MUST 为 workerpool、scheduler、localcache 等主动后台 primitive 提供拥有者显式关闭契约，并要求 Fx composition 只关闭本组件拥有的资源。

#### Scenario: workerpool 和 localcache 关闭幂等
- **WHEN** 调用方重复关闭 workerpool 或 localcache
- **THEN** 关闭操作 MUST 幂等且不得 panic
- **AND** 关闭操作 MUST NOT 关闭调用方注入的 Redis、PostgreSQL、Ent 或其他共享资源
