## ADDED Requirements

### Requirement: user-service Fx lifecycle timeout 同源与作用边界

user-service composition root MUST 使用同一份已解析 service config 的 `runtime.lifecycle.start_timeout` 和 `runtime.lifecycle.stop_timeout` 设置 App 顶层 `fx.StartTimeout` 与 `fx.StopTimeout`。`serve` 命令手动调用 `App.Start` 和 `App.Stop` 时 MUST 使用同一配置值创建显式 context；这些 context MUST 作为当前 CLI lifecycle hook 的实际 deadline，Fx App 顶层 timeout 与显式 context MUST NOT 被解释为可累加的两段预算。

`fx.StartTimeout` MUST NOT 被描述或实现为配置加载或 `fx.New` 同步构造阶段的 deadline。配置加载 MUST 在 `fx.New` 之前完成；对构造期 provider、invoke 或资源 I/O 的 timeout 与 lifecycle 迁移 MUST 由其自身 context 或后续独立 change 定义。

#### Scenario: App 与 CLI 使用相同 lifecycle 配置

- **WHEN** CLI 使用已解析 service config 创建正式 Fx App
- **THEN** App 的 Start/Stop timeout MUST 分别等于该配置的 `runtime.lifecycle.start_timeout` 和 `runtime.lifecycle.stop_timeout`
- **AND** CLI 传给 `App.Start` 与 `App.Stop` 的 context MUST 分别使用相同的两个配置值

#### Scenario: 显式 Start context 是实际启动边界

- **WHEN** `serve` 命令手动调用 `App.Start(startCtx)`
- **THEN** lifecycle `OnStart` hook MUST 接收受 `startCtx` 限制的启动预算
- **AND** App 顶层 `fx.StartTimeout` MUST NOT 在该 context 之外增加或串联第二段启动预算

#### Scenario: 显式 Stop context 是实际停止边界

- **WHEN** 外部 context 或内部 Fx shutdown signal 触发 `serve` 停止
- **THEN** CLI MUST 使用未被取消的上游 context value 和配置化 `stop_timeout` 调用一次 `App.Stop(stopCtx)`
- **AND** App 顶层 `fx.StopTimeout` MUST NOT 在该 context 之外增加或串联第二段停止预算

#### Scenario: fx.New 不受 StartTimeout 限制

- **WHEN** user-service 在 `fx.New` 中同步构建依赖图、执行 invoke 或解析其 constructor 依赖
- **THEN** 系统 MUST NOT 声称 `fx.StartTimeout` 会中断或限制该阶段
- **AND** 文档与配置注释 MUST 将 `start_timeout` 描述为配置加载后 `App.Start` lifecycle 阶段的预算

#### Scenario: 不隐式迁移构造期资源

- **WHEN** 本 change 设置 App 顶层 lifecycle timeout 并统一配置来源
- **THEN** 系统 MUST NOT 因此声称全部 provider constructor 或 invoke 已具备可取消的构造期 deadline
- **AND** 任何把构造期资源工作迁移到 `OnStart` 的行为 MUST 另行评估依赖顺序、回滚和测试
