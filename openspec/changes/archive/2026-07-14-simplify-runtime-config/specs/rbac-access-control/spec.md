## MODIFIED Requirements

### Requirement: 授权热路径用户角色本地缓存

系统 MUST 在 RBAC 授权热路径中使用有容量上限的本地 loading cache 缓存用户当前启用角色 ID 集合，并通过主动失效和全量清空保证在线 RBAC 变更后不依赖 TTL 长期收敛。user-service MUST 从服务自有 `resources.redis` 和 `resources.postgres` 读取 RBAC 依赖，并使用 `rbac.user_role_cache` 配置本地缓存，MUST NOT 使用共享核心 LocalCache、Redis 或 PostgreSQL 配置字段。

#### Scenario: 用户角色缓存命中

- **WHEN** 业务请求进入 RBAC 授权中间件且用户角色本地缓存命中
- **THEN** 授权判断 MUST 使用缓存中的角色 ID 副本
- **AND** 调用方对返回 slice 的修改 MUST NOT 污染缓存内部值

#### Scenario: 用户角色缓存 miss

- **WHEN** 业务请求进入 RBAC 授权中间件且用户角色本地缓存 miss
- **THEN** 系统 MUST 通过 `singleflight` 合并同用户并发回源
- **AND** 回源 MUST 查询 PostgreSQL 中该用户当前绑定的启用角色
- **AND** loader 错误 MUST NOT 写入本地缓存

#### Scenario: 用户角色缓存容量边界

- **WHEN** 单实例处理大量不同用户的 RBAC 授权请求
- **THEN** `rbac.user_role_cache.size` MUST 限制进程内条目预算
- **AND** 容量淘汰、准入拒绝或 TTL 过期后 MUST 能通过 PostgreSQL 回源恢复授权判断

#### Scenario: RBAC 资源和缓存缺省值

- **WHEN** user-service 装配 RBAC 用户角色 resolver
- **THEN** Redis 和 PostgreSQL MUST 从 `resources.redis` 和 `resources.postgres` 的必需具名资源读取
- **AND** 未显式配置 `rbac.user_role_cache` 时 MUST 使用 `enabled=true`、`size=100000`、`ttl=5s` 和 `load_timeout=500ms`
- **AND** `rbac.user_role_cache` MUST NOT 暴露 `num_counters` 或 `buffer_items`

#### Scenario: 校验启用的 user role cache

- **WHEN** `rbac.user_role_cache.enabled` 为 true
- **THEN** `size`、`ttl` 和 `load_timeout` MUST 为正值

#### Scenario: 关闭 RBAC cache

- **WHEN** `rbac.user_role_cache.enabled` 为 false
- **THEN** 授权结果和策略一致性 MUST 保持正确
- **AND** 关闭缓存 MAY 只造成性能下降
- **AND** `size`、`ttl` 和 `load_timeout` MAY 为零值

#### Scenario: 在线用户角色变更失效缓存

- **WHEN** 用户角色绑定通过在线 HTTP API 添加、替换或移除成功
- **THEN** 本实例 MUST 删除对应用户角色本地缓存或清空相关缓存
- **AND** 其他副本 MUST 通过既有 policy sync 机制感知变更并失效本地缓存

#### Scenario: policy reload 全量失效缓存

- **WHEN** RBAC policy reload 或全量策略刷新完成
- **THEN** 系统 MUST 清空本实例用户角色本地缓存
- **AND** 后续授权请求 MUST 通过回源重新建立本地投影
