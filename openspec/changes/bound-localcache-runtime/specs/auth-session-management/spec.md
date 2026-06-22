## MODIFIED Requirements

### Requirement: 会话与 token version 策略

系统 MUST 在 auth application 中拥有 token version 校验、refresh session 生命周期、每用户活跃 refresh session 上限和会话撤销语义。受保护路由的 token version 本地缓存 MUST 使用有容量上限的 `common/runtime/localcache` loading cache，并且 MUST 将 Redis token version 投影和 PostgreSQL 当前值作为回源路径。

#### Scenario: 活跃 session 上限

- **WHEN** 用户超过配置的活跃 refresh session 上限
- **THEN** Redis 中最旧的活跃会话 MUST 作为安全敏感操作的一部分被同步裁剪

#### Scenario: token version 校验链路

- **WHEN** access token 已通过 JWT 解析且未过期
- **THEN** 受保护路由 MUST 按有界本地缓存、Redis token version 投影、PostgreSQL 当前值回源的顺序解析当前版本
- **AND** Redis miss 后 MAY 回源数据库并回填 Redis
- **AND** 系统 MUST NOT 缓存错误结果

#### Scenario: token version 本地缓存容量

- **WHEN** 不同用户的 access token version 在同一实例内被校验
- **THEN** 系统 MUST 通过 `auth_token_version` 本地缓存容量限制控制进程内条目预算
- **AND** 系统 MUST 在容量淘汰、准入拒绝或 TTL 过期后通过 Redis 或 PostgreSQL 回源恢复校验能力

#### Scenario: token version 投影刷新

- **WHEN** 用户执行全部会话退出或强制改密导致当前 `token_version` 变化
- **THEN** 系统 MUST 使本实例本地 token version 缓存失效，并刷新 Redis token version 投影
- **AND** 旧版本 MUST NOT 覆盖 Redis 中已存在的较新版本
