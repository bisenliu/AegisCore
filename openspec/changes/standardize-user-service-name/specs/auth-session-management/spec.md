## MODIFIED Requirements

### Requirement: 认证架构、配置与资源生命周期

user-service auth feature MUST 私有拥有 token issuer、claims schema、subject 常量、TTL fallback 和认证策略配置；`common/security/auth` MUST 只提供通用验证原语。认证 application、adapter、controller 和 composition MUST 通过 framework-neutral constructor、消费侧最小 port 和窄 settings 表达依赖，并显式管理 auth 自有后台资源。默认 JWT issuer、auth Redis key prefix 和认证相关配置示例 MUST 使用 `aegiscore-user-service`，旧 `aegiscore-user-services` issuer 或 Redis key prefix MUST NOT 被兼容接受、读取或双写。

#### Scenario: 服务私有签发、配置和分层边界

- **WHEN** user-service 签发 token、装配认证流程或新增凭据、token、session、token version 行为
- **THEN** 系统 MUST 从服务私有配置读取 JWT TTL、password KDF 预算、refresh rotation、token version cache TTL 和每用户活跃 session 上限
- **AND** production-like 环境中的 `auth.jwt.secret` MUST 至少为 32 bytes，校验错误 MUST 明确定位该配置项
- **AND** 默认 `auth.jwt.issuer` MUST 为 `aegiscore-user-service`
- **AND** `common/runtime/config` MUST NOT 声明或校验这些业务策略
- **AND** 业务编排 MUST 位于 auth application 或 domain，Redis 和 PostgreSQL adapter MUST 只实现消费侧最小存储 port

#### Scenario: framework-neutral 构造和缺失依赖错误

- **WHEN** 构造 auth use case、PostgreSQL credential store、Redis session store、HTTP controller 或本地 cache/invalidator
- **THEN** constructor MUST 只接收职责所需的普通 Go collaborator 和窄 settings
- **AND** 生产 constructor 输入 MUST NOT 嵌入 `fx.In`、`fx.Out`、Dig tag 或服务级 named resource metadata
- **AND** 必需安全 collaborator 或无效窄 settings 缺失时 constructor MUST 返回明确 error 并拒绝装配，MUST NOT panic、静默降级或提供 no-op 安全替身

#### Scenario: auth 自有资源启动、停止和回滚

- **WHEN** auth session purge pool、token-version 本地缓存或其他主动资源被启用
- **THEN** user-service MUST 将其作为 auth 或服务 composition 拥有的生命周期资源注册，并在停止或启动回滚时关闭
- **AND** 资源关闭错误 MUST 可诊断，MUST NOT 被静默吞掉

#### Scenario: auth Redis key prefix

- **WHEN** auth Redis adapter 生成 refresh session、password-change session、token version projection 或 session purge key
- **THEN** key prefix MUST 来自当前 `app.name` 并归一化为 `aegiscore-user-service`
- **AND** adapter MUST NOT 查询、删除、迁移或双写旧 `aegiscore-user-services` prefix 下的 key
- **AND** 发布后旧 prefix 下的 token version projection 或 session 数据 MUST 不再影响认证结果
