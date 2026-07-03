## ADDED Requirements

### Requirement: RBAC policy watcher 生命周期契约

RBAC Redis policy watcher MUST 使用无参数 `Start()` 表达启动动作，并 MUST 通过内部可取消 context 驱动长期后台循环；Fx `OnStart` context MUST NOT 作为 watcher 后台循环的生命周期控制信号。watcher 停止 MUST 由 `Stop(ctx)` 触发内部 cancel 并等待后台循环退出，`Stop(ctx)` 的 context 仅用于限制停止等待时间。

#### Scenario: 启动 watcher 不消费 Fx 启动 context

- **WHEN** user-service 通过 Fx `OnStart` 启动 RBAC Redis policy watcher
- **THEN** lifecycle hook MUST 调用无参数 `Watcher.Start()`
- **AND** watcher 后台循环 MUST NOT 依赖 Fx `OnStart` context 的取消信号来退出

#### Scenario: Stop 关闭 watcher 后台循环

- **WHEN** user-service 通过 Fx `OnStop` 停止 RBAC Redis policy watcher
- **THEN** `Watcher.Stop(ctx)` MUST 取消 watcher 内部 context 并等待后台循环退出
- **AND** `Stop(ctx)` MUST 在传入 context 取消或超时时返回对应错误

#### Scenario: 策略同步语义保持不变

- **WHEN** watcher 通过 Redis Pub/Sub 或周期性版本补偿感知 RBAC policy version 变化
- **THEN** 系统 MUST 继续按既有 policy sync 语义执行 policy reload 或用户角色缓存失效
- **AND** 本生命周期契约变更 MUST NOT 改变 Redis policy version、Pub/Sub payload、补偿检查间隔、Casbin reload 或授权判断语义
