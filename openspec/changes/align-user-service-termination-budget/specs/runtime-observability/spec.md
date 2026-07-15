## ADDED Requirements

### Requirement: user-service 优雅关闭总预算边界

user-service MUST 将 `runtime.lifecycle.stop_timeout` 视为 `app.Stop()` 和全部 Fx `OnStop` hook 的进程级总预算。Fx MUST 保持逆注册顺序串行停止组件；每个 hook MUST 使用同一 Stop context 或其派生的更短 context，单个 HTTP、workerpool、exporter 或 datastore 关闭 timeout MUST NOT 被解释为全部 hook 的总预算，也不得通过并行执行 hook 绕开资源关闭顺序。

默认关闭链路 MUST 在 120 秒 Fx 总预算内依次为 HTTP 请求排空、auth session purge workerpool、RBAC policy watcher、pprof、tracing、Ent/PostgreSQL/Redis 和 logger 同步提供关闭机会；具有 25 秒 HTTP 子预算和 30 秒 workerpool 子预算的 hook MUST 继续受 Fx 剩余 deadline 约束，没有独立子预算的 hook MUST 继续受同一 Fx Stop context 约束。

#### Scenario: 外部终止信号进入总预算关闭链路

- **WHEN** user-service 收到 `SIGINT` 或 `SIGTERM`
- **THEN** 进程 MUST 使用默认 120 秒 Stop context 调用一次 `app.Stop()`
- **AND** Fx MUST 在该 context 内按逆注册顺序串行执行已注册的 `OnStop` hook

#### Scenario: 内部故障进入同一关闭链路

- **WHEN** HTTP 或 pprof server 的非预期退出产生 Fx shutdown signal
- **THEN** 进程 MUST 使用与外部终止相同的 Fx Stop 总预算和组件关闭语义
- **AND** 部署 grace MUST 为 tracing flush、datastore 关闭和 logger sync 等后序工作保留到总预算结束后的平台余量

#### Scenario: 局部组件 timeout 不替代总预算

- **WHEN** HTTP shutdown timeout 为 25 秒、auth session purge workerpool StopTimeout 为 30 秒或 OTLP exporter I/O timeout 为 5 秒
- **THEN** 这些值 MUST 仅限制各自组件或 I/O 操作
- **AND** 运维和自动校验 MUST NOT 使用任一局部 timeout 作为 Kubernetes 或 Helm termination grace 的应用预算来源

#### Scenario: 前序 hook 消耗关闭时间

- **WHEN** 一个前序 `OnStop` hook 在 Fx 逆序串行关闭中消耗部分 Stop 时间
- **THEN** 后序 hook MUST 观察同一全局 deadline 的剩余时间
- **AND** 系统 MUST NOT 为每个 hook 重新创建完整 120 秒父预算

#### Scenario: 正常快速关闭

- **WHEN** HTTP 已无活跃请求、workerpool 已无任务、watcher 已退出且外部资源可立即关闭
- **THEN** 各 `OnStop` hook MUST 在完成自身关闭后立即返回
- **AND** 进程 MUST NOT 为耗尽 Fx Stop budget 或 Kubernetes termination grace 而主动等待
