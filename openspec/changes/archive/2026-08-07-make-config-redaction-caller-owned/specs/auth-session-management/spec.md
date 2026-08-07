## MODIFIED Requirements

### Requirement: 认证架构、配置与 Redis 资源生命周期

user-service auth feature MUST 私有拥有 token issuer、claims schema、subject、TTL fallback 和认证策略配置；`common/security/auth` MUST 只提供通用验证原语。application、adapter、controller 和 composition MUST 通过 framework-neutral constructor、消费侧最小 port 和窄 settings 表达依赖，并显式管理 auth 自有后台资源。auth Redis adapter MUST 使用 Cluster-capable client 与同用户 hash tag 保证多 key 原子操作，且不得关闭共享 Redis client。user-service 的配置渲染边界 MUST 私有拥有 JWT、Redis、PostgreSQL 及服务私有敏感字段路径策略，并显式传入共享脱敏原语。

#### Scenario: 服务私有配置与分层
- **WHEN** user-service 签发 token、装配认证流程或新增认证行为
- **THEN** 系统 MUST 从服务私有配置读取 JWT TTL、refresh rotation、token version cache TTL 和活跃 session 上限，`common/runtime/config` MUST NOT 声明或校验这些策略
- **AND** 服务私有配置 MUST NOT 承载 password KDF、Argon2 或 bcrypt cost 预算；production-like 环境的 `auth.jwt.secret` MUST 至少 32 bytes，错误 MUST 定位配置项；默认 `auth.jwt.issuer` MUST 为 `aegiscore-user-service`
- **AND** 业务编排 MUST 位于 auth application 或 domain，Redis 与 PostgreSQL adapter MUST 只实现消费侧最小存储 port

#### Scenario: 服务私有敏感路径策略
- **WHEN** user-service CLI 或测试渲染 effective settings
- **THEN** user-service MUST 在自身 config 或 CLI 边界集中声明 `auth.jwt.secret`、Redis password、PostgreSQL password 及其他服务私有敏感路径
- **AND** 调用 `common/runtime/config` redaction primitive 时 MUST 显式传入这些路径，不得依赖 common 默认识别 auth、JWT、RBAC、Ent 或具名资源业务语义
- **AND** render 输出 MUST NOT 包含 JWT secret、Redis 凭据或 PostgreSQL 凭据，且原 settings map MUST 保持不变

#### Scenario: Framework-neutral 构造与安全依赖
- **WHEN** 构造 auth use case、store、controller 或本地 cache/invalidator
- **THEN** constructor MUST 只接收职责所需的普通 Go collaborator 和窄 settings，MUST NOT 嵌入 `fx.In`、`fx.Out`、Dig tag 或服务级 named resource metadata
- **AND** 必需安全 collaborator 或 settings 缺失时 MUST 返回明确 error 并拒绝装配，MUST NOT panic、静默降级或提供 no-op 安全替身

#### Scenario: 自有资源生命周期与共享资源所有权
- **WHEN** auth session purge pool 或其他主动资源启用、停止或启动失败
- **THEN** composition MUST 显式创建、启动和幂等关闭 auth 自有主动资源；部分失败时 MUST 立即清理并保留原始失败与清理失败，关闭 MUST 受 context/deadline 约束
- **WHEN** token-version localcache 启用或禁用
- **THEN** composition MUST 提供 cache 或 direct validator 所需的稳定读取、失效和统计视图，MUST NOT 为 localcache 创建 `Close`、`ErrClosed`、resource closed 状态或 no-op close 生命周期
- **AND** auth MUST NOT 关闭共享 Redis client、Redis 投影存储或 PostgreSQL 用户存储，且 auth 自有主动资源 MUST 先于共享 Redis client 关闭
- **AND** 资源不可用时受保护访问和撤销 MUST 明确报错或 fail-closed，MUST NOT 因 holder 为空而放行旧 token、无效 session 或撤销不完整结果

#### Scenario: 日志与 Redis key 命名
- **WHEN** auth application 或关键 adapter 记录业务日志
- **THEN** logger MUST 显式注入或来自 request context，MUST NOT 依赖可变 package-level 默认 logger
- **AND** 撤销失败日志 MUST 保留 `user_id`、错误分类、`request_id`、`trace_id` 和 `span_id`，MUST NOT 暴露 token、jti、session ID、Redis key、SQL、密码或敏感原始错误
- **WHEN** auth Redis adapter 生成 session、token version projection 或 purge key
- **THEN** prefix MUST 来自当前 `app.name` 并归一化为 `aegiscore-user-service`，MUST NOT 查询、删除、迁移或双写旧 prefix；旧 prefix 数据 MUST NOT 再影响认证结果

#### Scenario: refresh session 多 key 原子操作
- **WHEN** auth 创建、轮换、撤销或裁剪同一用户的 refresh sessions
- **THEN** 同一 Lua 或事务性操作涉及的 Redis key MUST 使用同一用户 hash tag
- **AND** Redis Cluster MUST NOT 因 CROSSSLOT 拒绝同一用户的 refresh session 原子操作

#### Scenario: 强制改密和 token version key schema
- **WHEN** auth 创建或消费 password-change session，或读写 token version projection
- **THEN** 相关 Redis key MUST 使用与用户一致的 hash tag 规则
- **AND** token version projection 刷新失败时的删除补偿 MUST 继续保持 Cluster 兼容

#### Scenario: Cluster client 生命周期边界
- **WHEN** auth store、purge pool、本地 cache 或 invalidator 停止
- **THEN** auth MUST NOT 关闭共享 Redis Cluster client
- **AND** Redis Cluster MOVED/ASK、slot 初始化或 CROSSSLOT 错误 MUST 作为可诊断错误暴露，不得被吞掉或降级为认证成功
