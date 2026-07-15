## ADDED Requirements

### Requirement: user-service CLI 统一协调外部与内部退出

`aegiscore-user-services serve` MUST 在 Fx App 成功启动后同时消费外部 context 取消和 `App.Wait()` 返回的 `fx.ShutdownSignal`。任一退出来源就绪后，命令 MUST 使用配置化进程停止预算调用且仅调用一次 `App.Stop()`，并把内部失败或停止失败转换为 Cobra error；命令内部 MUST NOT 调用 `os.Exit`。

#### Scenario: 外部终止信号正常退出

- **WHEN** `SIGINT`、`SIGTERM` 或上游 context 取消触发 serve 命令退出，且 `App.Stop()` 成功
- **THEN** 命令 MUST 使用未被取消的上游 context value 和 `runtime.lifecycle.stop_timeout` 预算调用一次 `App.Stop()`
- **AND** 命令 MUST 正常返回且不产生非零 Cobra error

#### Scenario: 内部零 exit code 请求关闭

- **WHEN** `App.Wait()` 返回 exit code 为 `0` 的内部 `fx.ShutdownSignal`，且 `App.Stop()` 成功
- **THEN** 命令 MUST 立即调用一次带预算的 `App.Stop()`
- **AND** 命令 MUST 正常返回

#### Scenario: 内部非零 exit code 请求关闭

- **WHEN** `App.Wait()` 返回非零 exit code 的 `fx.ShutdownSignal`
- **THEN** 命令 MUST 在一次带预算的 `App.Stop()` 完成后返回包含该 exit code 的 Cobra error
- **AND** 现有 main 入口 MUST 将该 error 转换为非零进程退出码

#### Scenario: App Stop 失败

- **WHEN** 任一退出来源触发停止且 `App.Stop()` 返回错误
- **THEN** 命令 MUST 返回可诊断的 Cobra error
- **AND** 若内部 shutdown signal 同时携带非零 exit code，返回错误 MUST 同时保留内部 exit code 与 Stop error，不能以其中一项覆盖另一项

#### Scenario: 多个退出来源并发竞争

- **WHEN** 外部 context 取消与一个或多个内部 shutdown signal 并发到达
- **THEN** 命令 MUST 只调用一次 `App.Stop()`
- **AND** 命令 MUST 在停止预算内完成或返回停止错误，不得重复 Stop 或死锁

#### Scenario: 保持手动生命周期可测试性

- **WHEN** 命令层测试构造 serve App 替身
- **THEN** 最小 App 接口 MUST 只暴露 `Start`、`Wait` 和 `Stop` 所需生命周期能力
- **AND** 测试 MUST 继续通过命令实例或函数调用范围内的局部 factory 注入替身，不得引入 package-level 可变测试 hook
