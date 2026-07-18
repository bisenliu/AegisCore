## ADDED Requirements

### Requirement: RBAC watcher 和缓存生命周期启动安全

系统 MUST 在 permission/RBAC Fx composition 中显式编排 policy initial load、Redis policy watcher 和 user-role cache 的生命周期。watcher constructor MUST 只创建对象，不得启动 goroutine、订阅 Redis 或执行版本补偿循环；`Start` 创建的后台循环 MUST 在启动失败或停止时通过 `Stop(ctx)` 关闭。

#### Scenario: Watcher 后续启动失败回滚
- **WHEN** Redis policy watcher 的 `Start` 已成功启动后台循环
- **AND** 后续 Fx hook 失败导致 App 启动失败
- **THEN** watcher MUST 收到停止信号并在调用方 context 限制内退出
- **AND** watcher MUST NOT 关闭共享 Redis、Ent 或 PostgreSQL 资源

#### Scenario: User-role cache 启动失败关闭
- **WHEN** user-role cache 或 resolver 在启动阶段创建了 localcache 资源
- **AND** App 启动失败或服务停止
- **THEN** cache MUST 执行幂等 `Close`
- **AND** 关闭后授权语义 MUST 继续 fail-closed，不得产生允许结果

### Requirement: RBAC 生命周期调整不改变授权与同步语义

系统 MUST 在调整 permission/RBAC 资源生命周期时保持权限目录、角色、绑定、Casbin policy、授权 fail-closed、policy reload、Redis policy version、Pub/Sub 和周期性补偿语义不变。

#### Scenario: 显式 lifecycle 编排保持行为
- **WHEN** permission Fx composition 将资源创建从 constructor 移到 `OnStart` 或补齐部分失败清理
- **THEN** 在线 RBAC 写操作成功后的本实例 reload、版本发布、通知和其他副本收敛语义 MUST 保持不变
- **AND** 初始加载失败、watcher 停止超时和同步失败仍 MUST 按既有错误契约传播或 fail-closed
