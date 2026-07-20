## MODIFIED Requirements

### Requirement: RBAC 策略加载、缓存与多副本同步

系统 MUST 以 PostgreSQL 关系数据作为业务权威来源，以本地 Casbin policy 和用户角色 loading cache 作为授权投影。系统 MUST 在启动时显式加载 policy，在线 RBAC 写操作成功后刷新本实例状态，并通过 Redis policy version、Pub/Sub 和周期性版本补偿同步其他副本。授权热路径 MUST 使用本地 enforcer 和本地用户角色解析结果，MUST NOT 每请求读取 Redis version。

#### Scenario: 初始加载和恢复

- **WHEN** user-service 启动 permission/RBAC 模块
- **THEN** composition 层 MUST 显式调用初始 policy 加载入口，并将可取消或带超时的启动 context 传给 policy loader
- **WHEN** 初始 policy 加载失败或被取消
- **THEN** engine MUST 记录最近错误和 reload 失败指标，后续授权 MUST fail-closed，`app.Start` MUST 保持成功
- **AND** reload 状态和 readiness/startup 可观测性 MUST 保留失败信息并拒绝接入业务流量
- **WHEN** 后续 Pub/Sub、版本补偿或显式 reload 成功加载当前 policy
- **THEN** engine MUST 替换为最新可用 policy、清除最近 reload 错误并恢复 readiness/startup

#### Scenario: 用户角色缓存 key 与容量

- **WHEN** RBAC user-role cache 被启用
- **THEN** permission feature MUST 使用 `uuid.UUID` 作为真实业务 key，并将配置的正数 size 映射为最大 item 数
- **AND** common MUST NOT 字符串化 UUID、接收 key encoder 或暴露底层 cache option

#### Scenario: 用户角色缓存命中与 value 隔离

- **WHEN** 用户角色缓存命中
- **THEN** 授权 MUST 使用缓存中角色 ID 的防御性副本，调用方对返回 slice 的修改 MUST NOT 污染缓存内部值或后续读取
- **AND** permission feature MUST 在 loader 写入缓存前及 `RolesForUser` 返回调用方前复制 `[]uuid.UUID`
- **AND** `common/runtime/localcache` MUST NOT 承担角色 ID clone 语义

#### Scenario: 用户角色缓存未命中与关闭

- **WHEN** 用户角色缓存未命中
- **THEN** 系统 MUST 合并同一 `uuid.UUID` 用户的并发回源并查询 PostgreSQL 中的当前启用角色，loader 错误 MUST NOT 写入缓存
- **WHEN** user-role cache 已关闭或回源失败
- **THEN** 授权 MUST fail-closed，MUST NOT 因 cache 不可用产生允许结果
- **WHEN** `rbac.user_role_cache.enabled` 为 false
- **THEN** 系统 MUST 直接回源、返回独立角色 ID slice 并保持正确的 fail-closed 授权语义
- **AND** direct stats source MUST 使用 `LoadSuccess` 与 `LoadError` 表达逐次回源结果

#### Scenario: 在线写后同步

- **WHEN** 权限、角色状态、角色权限绑定或用户角色绑定通过在线 API 持久化成功
- **THEN** 本实例 MUST reload policy 或失效相关用户缓存，并发布新的 Redis policy version 和 Pub/Sub 通知
- **AND** 持久化成功后的 reload、缓存失效、version 发布或通知失败 MUST 向调用方返回同步错误，成功响应 MUST NOT 掩盖该错误
- **AND** `PolicyChangeNotifier` MUST 是正式 command service 的必需依赖

#### Scenario: reload、发布和副本收敛

- **WHEN** policy refresh coordinator 执行本地 reload 和 version 发布
- **THEN** 本地 reload 失败后系统 MUST 仍尝试发布 version
- **AND** 两者同时失败时返回错误 MUST 保留两项失败，只有两者均成功时系统才 MUST 标记本实例已应用该 version
- **WHEN** watcher 通过 Pub/Sub 或周期性版本检查发现远端 policy version 更新
- **THEN** 系统 MUST reload policy 或失效用户角色缓存
- **AND** Pub/Sub 丢失时周期性版本补偿 MUST 使副本最终收敛
