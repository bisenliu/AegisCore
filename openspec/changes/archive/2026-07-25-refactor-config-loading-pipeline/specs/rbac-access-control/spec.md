## ADDED Requirements

### Requirement: RBAC feature cache 配置与依赖边界

user-service MUST 私有拥有 user-role feature cache 的默认值、启用时校验和到通用 localcache 配置的集中映射。permission/RBAC 构造路径 MUST 只消费窄 RBAC settings，不得依赖完整 user-service 根配置；cache 禁用只能改变性能，授权必须继续 fail-closed。

#### Scenario: User-role cache 默认值与创建

- **WHEN** `rbac.user_role_cache` 未配置
- **THEN** user-service MUST 使用 `enabled=true`、`size=100000`、`ttl=5s` 和 `load_timeout=500ms` 的完整默认值
- **WHEN** `rbac.user_role_cache.enabled=true`
- **THEN** `size`、`ttl` 和 `load_timeout` MUST 为正值
- **AND** permission feature MUST 通过集中转换创建具名 `rbac_user_roles` loading cache，配置的 `size` MUST 映射为最大 item 数

#### Scenario: User-role cache 禁用

- **WHEN** `rbac.user_role_cache.enabled=false`
- **THEN** 系统 MUST 忽略 cache 的 `size`、`ttl` 和 `load_timeout`，不创建通用 loading cache，并逐次从 PostgreSQL 回源当前启用角色
- **AND** direct resolver MUST 返回独立角色 ID slice、记录 `LoadSuccess` 或 `LoadError`，并在回源错误或 context 取消时保持 fail-closed

#### Scenario: RBAC settings 依赖边界

- **WHEN** composition 构造用户角色 resolver、policy loader 或其他 RBAC runtime 资源
- **THEN** permission/RBAC provider MUST 接收只包含职责所需字段的 RBAC settings
- **AND** permission/RBAC feature MUST NOT 依赖完整 user-service 根配置或读取 auth、Ent、resources 等无关配置段
- **AND** feature cache 配置、必需缓存名和角色值复制语义 MUST 留在 user-service，不得进入 `common/runtime/localcache`
