## ADDED Requirements

### Requirement: RBAC watcher stop 测试具备硬超时

RBAC policy watcher、policy sync 和启动失败回滚相关测试 MUST 验证 watcher stop 尊重调用方 context，并 MUST 通过测试级硬超时保护避免 watcher、订阅器或关闭回调阻塞测试进程。

#### Scenario: watcher Stop deadline 不阻塞测试

- **WHEN** 测试 `Watcher.Stop(ctx)` 在订阅循环尚未退出时的 deadline 行为
- **THEN** 测试 MUST 以 goroutine/select 或等价机制等待 Stop 返回
- **AND** 如果 Stop 未在测试级 guard 内返回，测试 MUST 失败并指出 watcher stop 阻塞

#### Scenario: watcher 重复停止仍可验证

- **WHEN** 第一次 `Watcher.Stop(ctx)` 因 context deadline 返回错误后释放阻塞订阅器
- **THEN** 后续重复 `Stop(context.Background())` MUST 被测试验证为幂等成功
- **AND** 测试 MUST 验证 watcher 不关闭调用方注入的 Redis、Ent 或 PostgreSQL 共享资源

#### Scenario: 启动失败回滚停止 watcher

- **WHEN** permission/RBAC Fx module 的后续 `OnStart` hook 失败
- **THEN** 测试 MUST 验证已启动 watcher 被停止并保持 Redis client 可用
- **AND** 启动失败路径 MUST 使用测试 logger 和带 deadline 的 Start context 保留 rollback 诊断信息

#### Scenario: RBAC lifecycle stop 聚合错误不阻塞

- **WHEN** 测试 `stopRBACLifecycle` 聚合 watcher stop 和 user-role cache close 错误
- **THEN** 测试 MUST 保留两个错误的 `errors.Is` 断言
- **AND** watcher stop 永不返回时测试 MUST 在测试级 guard 内失败
