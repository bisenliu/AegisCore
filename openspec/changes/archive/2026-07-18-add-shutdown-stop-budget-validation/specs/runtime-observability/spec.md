## MODIFIED Requirements

### Requirement: 监听故障与优雅关闭

系统 MUST 将 HTTP 或 pprof listener 的非预期退出转换为 Fx shutdown signal，并在统一的 `runtime.lifecycle.stop_timeout` 总预算内按逆序 lifecycle hook 完成优雅关闭。系统 MUST 通过配置校验和 lifecycle 测试保证总预算覆盖关键串行停止路径，避免前序 hook 消耗 deadline 后使后续资源清理无法获得新的完整预算。

#### Scenario: listener 非预期退出

- **WHEN** HTTP 或 pprof `Serve` 在未进入正常关闭阶段时返回错误
- **THEN** 系统 MUST 记录可诊断错误并触发非零内部 shutdown signal
- **WHEN** 正常关闭导致 `http.ErrServerClosed`
- **THEN** 系统 MUST NOT 将其视为内部故障

#### Scenario: 外部与内部退出共用预算

- **WHEN** 外部终止信号或内部故障触发关闭
- **THEN** 系统 MUST 使用同一未被取消的上游 context value 和 `runtime.lifecycle.stop_timeout` 总预算执行 `App.Stop`
- **AND** 局部 HTTP、gRPC、tracing 或 logger timeout MUST NOT 替代总预算

#### Scenario: 前序 hook 消耗时间

- **WHEN** 前序 `OnStop` hook 已消耗部分总预算
- **THEN** 后续 hook MUST 只使用剩余时间
- **AND** 总关闭耗时 MUST NOT 因每个组件重新创建完整预算而无界增长

#### Scenario: lifecycle timeout 同源

- **WHEN** App 和 CLI 构建启动或停止 context
- **THEN** 两者 MUST 使用同一已加载并校验的 lifecycle 配置
- **AND** `fx.New` 构造期 MUST NOT 被误算入 `StartTimeout`，也 MUST NOT 为满足 timeout 而隐式迁移现有资源构造语义

#### Scenario: 快速正常关闭

- **WHEN** 所有 hook 在预算内完成
- **THEN** App MUST 立即完成关闭，不得等待完整 timeout

#### Scenario: 组合停止预算覆盖关键路径

- **WHEN** user-service 加载 runtime lifecycle 配置
- **THEN** `runtime.lifecycle.stop_timeout` MUST 至少覆盖 HTTP graceful drain、feature worker drain、tracing flush allowance 和 shutdown safety margin 的组合最低预算
- **AND** 配置不足 MUST 在 App 构造、监听启动或资源初始化前失败

#### Scenario: Fx stop hook 错误不截断后续清理

- **WHEN** 某个 `OnStop` hook 返回普通 error
- **THEN** 其余已注册 stop hooks MUST 继续被 Fx 调用
- **AND** 关闭风险评估 MUST 聚焦共享 context deadline 是否已被耗尽，而不是假定普通 error 会直接截断后续清理

#### Scenario: 关键关闭顺序可验证

- **WHEN** user-service App 执行正常停止
- **THEN** lifecycle 测试 MUST 验证 HTTP 和 pprof 先停止接收请求
- **AND** RBAC watcher、feature worker 和 cache MUST 在 Ent、Redis、PostgreSQL、tracing 和 logger 关闭前完成自身停止或关闭
- **AND** tracing MUST 在 logger sync 前完成 flush 或 shutdown
