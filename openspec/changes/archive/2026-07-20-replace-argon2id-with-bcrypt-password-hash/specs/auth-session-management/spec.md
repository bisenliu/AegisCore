## MODIFIED Requirements

### Requirement: 用户登录与用途隔离令牌

系统 MUST 在凭证、用户状态和会话策略校验通过后签发用途隔离的 access、refresh 或 password-change token。所有 token MUST 包含标准 `jti`，并 MUST 通过 `access`、`refresh` 和 `password_change` subject 限定使用流程；任一用途、subject 或必要 claims 不匹配时 MUST 拒绝 token。

#### Scenario: 普通登录成功

- **WHEN** 用户提供合法用户名和正确密码，且状态允许普通登录
- **THEN** 系统 MUST 创建普通 refresh session 并签发 access token 与 refresh token
- **AND** 登录 use case MUST 返回 `PasswordChangeRequired=false`
- **AND** HTTP 响应 MUST 为 `200 OK`、`CodeOK` 和 `success=true`
- **AND** data MUST 包含 access token、refresh token、token type 和 access token 过期秒数，MUST NOT 包含登录状态枚举字段

#### Scenario: 凭证、状态和侧信道防护

- **WHEN** 用户名不存在、密码不匹配，或用户状态不允许登录且不属于强制改密状态
- **THEN** 系统 MUST 拒绝签发任何 token 和创建会话
- **AND** 公开错误 MUST NOT 泄露用户是否存在、密码匹配结果或具体用户状态
- **AND** 用户名不存在时系统 MUST 使用当前 bcrypt dummy hash 执行 dummy password verification
- **AND** 旧 Argon2id、未知算法或格式非法的存储哈希 MUST 被视为无效凭据，MUST NOT 触发旧哈希验证、迁移、fallback 或 rehash

#### Scenario: 强制改密登录

- **WHEN** 凭据有效且用户状态要求强制修改密码
- **THEN** 登录 use case MUST 返回 `PasswordChangeRequired=true` 和 subject 为 `password_change` 的受限 token，而不是返回业务错误
- **AND** 系统 MUST 创建与该 token 绑定的一次性 password-change session，MUST NOT 创建普通 refresh session 或签发 refresh token
- **AND** HTTP 响应 MUST 为 `200 OK`、`CodePasswordChangeRequired`、code `20006` 和 `success=false`
- **AND** data MUST 只包含受限 access token、token type 和过期秒数，MUST NOT 包含 refresh token、`status`、`authenticated` 或 `password_change_required` 枚举字段

### Requirement: 认证 HTTP 与错误契约

系统 MUST 仅通过 `/api/v1/auth` 暴露认证入口，将登录、refresh 和强制改密作为公开认证路由，将退出当前会话、退出全部会话和普通改密作为 access token 保护路由。HTTP transport MUST 使用共享 response helper 渲染业务错误，并 MUST NOT 维护认证专用 sentinel-to-HTTP mapper。

#### Scenario: 公开与受保护路由

- **WHEN** 调用方访问登录、refresh 或强制改密入口
- **THEN** 系统 MUST 允许请求进入 controller，并在业务层校验相应凭据或 token
- **AND** 退出和普通改密入口 MUST 在进入业务处理前校验 bearer token、user-service access claims 和 token version
- **AND** 系统 MUST NOT 暴露旧认证路径别名或认证绕过路径

#### Scenario: 共享 middleware 最小能力

- **WHEN** user-service 将 access token verifier 注入共享认证 middleware
- **THEN** 注入对象 MUST 只暴露 access token 验证能力
- **AND** 共享 middleware MUST NOT 获得 refresh token、password-change token 或任何 token 签发能力

#### Scenario: 认证失败和临时不可用响应

- **WHEN** 凭据无效、用户状态拒绝、缺失认证会话、token 无效、refresh session 无效或 password-change session 无效
- **THEN** 系统 MUST 返回稳定的 `401 Unauthorized` 认证错误，并保持统一公开文案或 token invalid 文案
- **WHEN** 认证流程返回 `authdomain.ErrSessionRevocationIncomplete`
- **THEN** 系统 MUST 返回 `503 Service Unavailable` 和 `CodeServiceUnavailable`
- **AND** 响应 MUST 使用对应稳定公开消息，MUST NOT 泄露 Redis key、session ID、jti、SQL、stacktrace 或内部错误文本

#### Scenario: 错误分类与统一出口

- **WHEN** auth controller 收到 use case 错误
- **THEN** controller MUST 直接调用 `response.Fail(c, err)`
- **AND** 认证错误 MUST 携带稳定 `Kind`、`Reason`、`Code` 和中文公开 `Message`
- **AND** 直接或包装后的错误 MUST 保持 `errors.Is` 或稳定 `Reason` 分类能力，供登录、refresh 和 logout metrics 使用

### Requirement: 认证架构、配置与资源生命周期

user-service auth feature MUST 私有拥有 token issuer、claims schema、subject 常量、TTL fallback 和认证策略配置；`common/security/auth` MUST 只提供通用验证原语。认证 application、adapter、controller 和 composition MUST 通过 framework-neutral constructor、消费侧最小 port 和窄 settings 表达依赖，并显式管理 auth 自有后台资源。默认 JWT issuer、auth Redis key prefix 和认证相关配置示例 MUST 使用 `aegiscore-user-service`，旧 `aegiscore-user-services` issuer 或 Redis key prefix MUST NOT 被兼容接受、读取或双写。

#### Scenario: 服务私有签发、配置和分层边界

- **WHEN** user-service 签发 token、装配认证流程或新增凭据、token、session、token version 行为
- **THEN** 系统 MUST 从服务私有配置读取 JWT TTL、refresh rotation、token version cache TTL 和每用户活跃 session 上限
- **AND** 系统 MUST NOT 从服务私有配置读取 password KDF、Argon2 或 bcrypt cost 预算
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
- **THEN** composition MUST 显式创建、启动和关闭这些 auth 自有资源
- **AND** 服务停止或启动失败时 MUST 停止已启动的 purge pool 并关闭已创建的本地缓存
- **AND** constructor 或启动阶段部分失败时 MUST 立即清理已创建且归 auth 拥有的资源，并保留原始失败和清理失败
- **AND** 停止和关闭 MUST 幂等、受 context/deadline 约束，disabled 或 direct 模式 MUST 提供一致 no-op close 语义

#### Scenario: 共享资源所有权和 fail-closed

- **WHEN** auth 资源停止、关闭或尚未启动
- **THEN** auth 组件 MUST NOT 关闭共享 Redis client、Redis token version 投影存储或 PostgreSQL 用户存储
- **AND** auth 自有资源 MUST 先于共享 Redis client 关闭
- **AND** 受保护访问和会话撤销流程 MUST 返回明确错误或保持 fail-closed
- **AND** 系统 MUST NOT 因 holder 中资源为空而允许旧 token、无效 refresh session 或撤销不完整结果通过

#### Scenario: 显式日志依赖

- **WHEN** auth application 或关键 Redis/PostgreSQL infrastructure 记录正式业务日志
- **THEN** logger MUST 由 constructor 显式注入或从 request context 获取，MUST NOT 依赖可变 package-level 默认 logger
- **AND** 撤销失败日志 MUST 保留可用的 `user_id`、错误分类、`request_id`、`trace_id` 和 `span_id`
- **AND** 日志 MUST NOT 暴露 token、jti、session ID、Redis key、SQL、密码或敏感原始错误

#### Scenario: auth Redis key prefix

- **WHEN** auth Redis adapter 生成 refresh session、password-change session、token version projection 或 session purge key
- **THEN** key prefix MUST 来自当前 `app.name` 并归一化为 `aegiscore-user-service`
- **AND** adapter MUST NOT 查询、删除、迁移或双写旧 `aegiscore-user-services` prefix 下的 key
- **AND** 发布后旧 prefix 下的 token version projection 或 session 数据 MUST 不再影响认证结果
