## ADDED Requirements

### Requirement: RBAC policy sync 兼容 Redis Cluster

RBAC policy sync MUST 使用 Redis Cluster client 维护 policy version 和 policy refresh 通知。policy version key 与 policy refresh channel MUST 使用固定 hash tag 表达同一同步域；Pub/Sub MUST 只作为快速路径，周期性 version check MUST 作为漏消息、重启和 Cluster 通知差异的最终收敛兜底。

#### Scenario: 发布 policy 变更

- **WHEN** 角色状态、角色权限或用户角色绑定通过在线 API 持久化成功
- **THEN** 系统 MUST 在 Redis Cluster 中递增 policy version 并发布 policy refresh 通知
- **AND** version key 与 refresh channel MUST 使用固定 hash tag，且不得包含用户、角色、权限或请求级高基数标识

#### Scenario: Pub/Sub 漏消息后的版本补偿

- **WHEN** watcher 未收到 Pub/Sub 消息、Pub/Sub channel 中断、实例重启或 Redis Cluster 通知只作为 best-effort 送达
- **THEN** 周期性 version check MUST 能发现远端 policy version 更新并触发 reload 或用户角色缓存失效
- **AND** 授权热路径 MUST 继续使用本地 enforcer 和本地用户角色解析结果，MUST NOT 每请求读取 Redis version

#### Scenario: Redis Cluster 不可用时 fail-closed

- **WHEN** Redis Cluster policy version 读取、发布或订阅发生错误
- **THEN** 系统 MUST 保留可观察错误，并按现有 policy sync 失败、readiness/startup 和授权 fail-closed 语义处理
- **AND** 成功响应 MUST NOT 掩盖 version 发布或通知失败
