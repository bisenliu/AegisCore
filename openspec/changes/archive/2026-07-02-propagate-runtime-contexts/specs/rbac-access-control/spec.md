## ADDED Requirements

### Requirement: Casbin 初始 policy 加载上下文传播

系统 MUST 在 user-service 启动 lifecycle 中执行 Casbin Engine 初始 policy 加载，并将 Fx `OnStart` 提供的启动 context 传播到 policy loader 及其持久化查询。Casbin Engine 构造器 MUST NOT 在 provider 构造阶段执行不可取消的初始 policy reload。

#### Scenario: 启动 context 传播到初始加载
- **WHEN** user-service 通过 Fx 启动 permission/RBAC 模块并初始化 Casbin Engine
- **THEN** 初始 policy reload MUST 使用 Fx `OnStart` 传入的 context 调用 `Loader.LoadPolicies(ctx)`
- **AND** 启动 context 取消或超时时，底层 policy loader MUST 能观察到对应取消信号

#### Scenario: 初始加载失败保持 fail-closed
- **WHEN** Casbin 初始 policy 加载失败或因启动 context 取消而未构造可用 enforcer
- **THEN** Engine MUST 记录最近错误并更新 policy reload 失败指标
- **AND** 后续授权判断 MUST fail-closed，不得放行缺少可用 policy 的请求
- **AND** 服务装配行为 MUST 保持既有语义，不因本场景自动改为启动失败

#### Scenario: 手动 reload 继续使用调用方 context
- **WHEN** 在线 RBAC 写操作或 watcher 触发 Casbin policy reload
- **THEN** `Reload(ctx)` MUST 继续使用调用方传入的 context 执行 policy loader
- **AND** 本次变更不得改变 policy 权威来源、用户角色缓存失效或 Redis policy sync 语义
