## MODIFIED Requirements

### Requirement: 用户登录与令牌签发

系统 MUST 提供用户名密码登录能力，并在凭证、用户状态和会话策略校验通过后签发访问令牌与刷新令牌。登录 use case MUST 使用登录专属结果字段表达是否需要强制改密；登录失败仍 MUST 通过错误返回。系统 MUST 将密码 KDF 资源池繁忙视为临时服务不可用，而不是无效凭据。user-service auth feature MUST 私有拥有 access、refresh、password-change token 的 issuer、claims schema、subject 常量和 TTL fallback，MUST NOT 依赖 `common/security/auth` 提供 token 签发能力或 user-service 专属 claims。

#### Scenario: 登录成功

- **WHEN** 用户提供合法用户名和正确密码，且用户状态允许普通登录
- **THEN** 系统 MUST 创建普通 refresh session、签发 access token 与 refresh token
- **AND** 登录 use case MUST 返回 `PasswordChangeRequired=false`
- **AND** 登录响应 MUST 返回 HTTP `200 OK`
- **AND** 登录响应 envelope MUST 携带 `CodeOK`
- **AND** 登录响应 envelope 的 `success` MUST 为 `true`
- **AND** 登录响应 data MUST 携带 access token、refresh token、token type 和 access token 过期秒数
- **AND** 登录响应 data MUST NOT 携带登录状态枚举字段

#### Scenario: 凭证错误

- **WHEN** 用户名不存在或密码不匹配
- **THEN** 系统 MUST 拒绝登录并返回一致的认证错误，且 MUST NOT 泄露具体凭证匹配细节

#### Scenario: 未知用户登录执行 dummy 密码校验

- **WHEN** 登录用户名不存在
- **THEN** 系统 MUST 使用当前支持的密码 KDF 参数执行 dummy password verification，以降低用户存在性侧信道
- **AND** dummy verification 返回密码 KDF 繁忙时 MUST 返回 `password.ErrPasswordKDFBusy` 对应的服务不可用错误，MUST NOT 折叠为无效凭据
- **AND** 日志、错误和响应 MUST NOT 泄露用户名是否存在

#### Scenario: 用户状态禁止登录

- **WHEN** 用户存在但状态不允许登录，且该状态不是强制改密状态
- **THEN** 系统 MUST 拒绝签发令牌并返回明确的状态相关错误

#### Scenario: 强制改密用户登录

- **WHEN** 用户凭据有效但账号状态要求强制修改密码
- **THEN** 系统 MUST 只签发 subject 为 `password_change` 的受限 token，不得创建普通 refresh session，也不得返回 refresh token
- **AND** 登录 use case MUST 返回 `PasswordChangeRequired=true`，而不是通过 error 表达该分支

#### Scenario: 强制改密登录返回业务码 envelope

- **WHEN** 用户凭据有效但账号状态要求强制修改密码
- **THEN** 登录响应 MUST 返回 HTTP `200 OK`
- **AND** 登录响应 envelope MUST 携带 `CodePasswordChangeRequired`
- **AND** 登录响应 envelope 的 `code` MUST 为 `20006`
- **AND** 登录响应 envelope 的 `success` MUST 为 `false`
- **AND** 登录响应 envelope 的 `message` MUST 使用强制改密用户提示
- **AND** 登录响应 data MUST 携带 subject 为 `password_change` 的受限 token 数据
- **AND** 登录响应 MUST NOT 携带 refresh token
- **AND** 登录响应 data MUST NOT 携带 `status`、`authenticated` 或 `password_change_required` 枚举字段

#### Scenario: 普通登录仍返回成功 code

- **WHEN** 用户凭据有效且账号状态允许普通登录
- **THEN** 登录响应 envelope MUST 携带 `CodeOK`
- **AND** 登录响应 envelope 的 `success` MUST 为 `true`
- **AND** 登录响应 MUST 携带 access token 与 refresh token
- **AND** 登录响应 MUST NOT 携带 `CodePasswordChangeRequired`

#### Scenario: 强制改密分支不创建普通会话

- **WHEN** 用户凭据有效但账号状态要求强制修改密码
- **THEN** 系统 MUST 只签发 subject 为 `password_change` 的受限 token
- **AND** 系统 MUST NOT 创建普通 refresh session
- **AND** 系统 MUST NOT 签发 refresh token

#### Scenario: 密码 KDF 资源繁忙

- **WHEN** 登录凭据校验进入密码 KDF 但实例内 Argon2 执行和等待队列已达资源上限
- **THEN** 系统 MUST 拒绝本次登录并返回 `503 Service Unavailable`
- **AND** 系统 MUST NOT 将该错误映射为无效凭据
- **AND** 系统 MUST NOT 签发 access token、refresh token 或 password change token
- **AND** 系统 MUST NOT 泄露用户名存在性、密码匹配状态、队列长度或 Argon2 并发配置

#### Scenario: token 缺少 jti

- **WHEN** access token、refresh token 或 password change token 缺少标准 `jti`
- **THEN** token MUST 被拒绝

#### Scenario: token subject 不匹配

- **WHEN** subject 为 `access`、`refresh` 或 `password_change` 的 token 被用于不匹配的认证流程
- **THEN** 系统 MUST 拒绝该 token，且 MUST NOT 在三类 token 之间兼容复用

#### Scenario: issuer 私有化

- **WHEN** user-service 需要签发 access token、refresh token 或 password change token
- **THEN** 签发逻辑 MUST 位于 user-service auth feature 私有边界
- **AND** 签发逻辑 MUST 使用 user-service 私有 JWT 配置和 user-service 私有 claims schema
- **AND** `common/security/auth` MUST NOT 提供这些 token 的签发入口

### Requirement: 会话与 token version 策略

系统 MUST 在 auth application 中拥有 token version 校验、refresh session 生命周期、每用户活跃 refresh session 上限和会话撤销语义。受保护路由的 token version 本地缓存 MUST 使用有容量上限的 `common/runtime/localcache` loading cache，并且 MUST 将 Redis token version 投影和 PostgreSQL 当前值作为回源路径。user-service auth/provider 边界 MUST 拥有 `auth_token_version` 缓存实例名，并 MUST 在缺少该配置实例时拒绝服务装配。`auth.token_version_cache_ttl` MUST 允许正数 duration 表示显式 Redis token version 投影 TTL，并 MUST 允许非正数 duration 表示使用服务默认 TTL；非正数配置 MUST NOT 创建永久 Redis token version 投影。user-service 私有配置 MUST 拥有 `auth.token_version_cache_ttl`、`auth.refresh_token_rotation`、`auth.max_active_sessions_per_user`、JWT TTL 和 password KDF 配置校验，`common/runtime/config` MUST NOT 声明或校验这些认证策略。auth application port MUST 将 PostgreSQL token version 持久化、Redis token version 投影和 refresh session 生命周期拆分为最小依赖接口，业务组件 MUST 只依赖自身所需的 port。token version 本地缓存失效接口 MUST 返回失败错误；会话撤销流程 MUST 记录本地失效失败并将其纳入投影错误返回，MUST NOT 静默忽略本地 cache 删除失败。

#### Scenario: 活跃 session 上限

- **WHEN** 用户超过配置的活跃 refresh session 上限
- **THEN** Redis 中最旧的活跃会话 MUST 作为安全敏感操作的一部分被同步裁剪

#### Scenario: token version 校验链路

- **WHEN** access token 已通过 JWT 解析且未过期
- **THEN** 受保护路由 MUST 按有界本地缓存、Redis token version 投影、PostgreSQL 当前值回源的顺序解析当前版本
- **AND** Redis miss 后 MAY 回源数据库并回填 Redis
- **AND** 系统 MUST NOT 缓存错误结果
- **AND** token version validator MUST NOT 依赖 refresh session 创建、轮换、查询或批量删除 port

#### Scenario: token version 本地缓存容量

- **WHEN** 不同用户的 access token version 在同一实例内被校验
- **THEN** 系统 MUST 通过 `auth_token_version` 本地缓存容量限制控制进程内条目预算
- **AND** 系统 MUST 在容量淘汰、准入拒绝或 TTL 过期后通过 Redis 或 PostgreSQL 回源恢复校验能力

#### Scenario: token version 必需缓存配置

- **WHEN** user-service 装配 auth token version validator
- **THEN** auth/provider MUST 使用本服务常量读取 `local_cache.auth_token_version`
- **AND** 缺少该配置实例时 MUST 返回明确错误并拒绝继续装配 token version 本地缓存

#### Scenario: 认证策略配置私有化

- **WHEN** user-service 装配 auth issuer、password KDF、refresh use case 或 session lifecycle
- **THEN** 系统 MUST 从 user-service 私有配置读取 JWT TTL、password KDF 预算、refresh token rotation、token version cache TTL 和每用户活跃 session 上限
- **AND** `common/runtime/config.Config` MUST NOT 暴露 `Auth` 字段
- **AND** user-service feature 或 provider MUST NOT 为读取这些策略而依赖 common 的 auth config 类型

#### Scenario: token version 投影 TTL 默认值

- **WHEN** `auth.token_version_cache_ttl` 配置为 `0` 或负数，且系统写入 Redis token version 投影
- **THEN** 系统 MUST 使用服务默认 TTL 写入 Redis token version 投影
- **AND** 系统 MUST NOT 写入无过期时间的 token version 投影

#### Scenario: token version 投影 TTL 显式值

- **WHEN** `auth.token_version_cache_ttl` 配置为正数 duration，且系统写入 Redis token version 投影
- **THEN** 系统 MUST 使用该显式 TTL 写入 Redis token version 投影

#### Scenario: token version 投影刷新

- **WHEN** 用户执行全部会话退出或强制改密导致当前 `token_version` 变化
- **THEN** 系统 MUST 使本实例本地 token version 缓存失效，并刷新 Redis token version 投影
- **AND** 旧版本 MUST NOT 覆盖 Redis 中已存在的较新版本
- **AND** Redis token version 投影刷新失败时，系统 MUST 尝试删除 Redis 投影，使后续校验能够回源 PostgreSQL
- **AND** 投影刷新失败 MUST 被记录并可测试，不得被静默忽略

#### Scenario: token version 本地缓存失效失败

- **WHEN** 用户执行全部会话退出或强制改密导致系统尝试删除本实例本地 token version cache，且本地 cache 删除返回错误
- **THEN** 系统 MUST 记录包含 `user_id` 和错误信息的日志
- **AND** 会话撤销流程 MUST 将该错误纳入投影错误返回
- **AND** 系统 MUST NOT 继续静默忽略本地 token version cache 删除失败

#### Scenario: session lifecycle 必需本地失效器

- **WHEN** auth application 构造 refresh session lifecycle
- **THEN** `TokenVersionLocalInvalidator` MUST 作为必需依赖提供
- **AND** 缺失该依赖时系统 MUST fail-fast 或拒绝装配，MUST NOT 静默跳过本地 token version cache 失效

### Requirement: 认证 HTTP 边界

系统 MUST 将公开认证路由和受保护认证路由分开挂载，并通过共享认证中间件保护需要 bearer token 的接口。认证 HTTP 边界 MUST 区分凭据认证失败和认证服务临时不可用。认证 HTTP controller 测试 MUST 使用 feature-local `gomock` 生成 mock 表达 use case 调用契约，不得保留手写 `stubAuthUseCases` 兼容入口。user-service MUST 通过服务私有 access token verifier adapter 将 user-service claims 和 subject 校验接入共享认证中间件，MUST NOT 让共享 middleware 依赖具备签发能力的 JWT concrete service。

#### Scenario: 公开登录路由

- **WHEN** 调用方访问登录或刷新等公开认证入口
- **THEN** 系统 MUST 允许请求进入认证 controller 并在业务层完成凭证校验

#### Scenario: 受保护认证路由

- **WHEN** 调用方访问退出、修改密码或其他受保护认证入口
- **THEN** 系统 MUST 先通过 JWT、user-service 私有 auth 配置和 token version validator 校验

#### Scenario: 无效 bearer token

- **WHEN** 受保护认证路由收到缺失、过期、格式错误或签名无效的 bearer token
- **THEN** 系统 MUST 在进入业务处理前拒绝请求

#### Scenario: 共享 middleware 不持有签发能力

- **WHEN** user-service 将认证 verifier 注入共享 HTTP middleware
- **THEN** 注入对象 MUST 只暴露访问令牌验证能力
- **AND** 注入对象 MUST NOT 暴露 refresh token、password-change token 或任意 token 签发能力
- **AND** middleware MUST 通过 verifier 返回的认证上下文执行后续 token version 校验和请求上下文注入

#### Scenario: 登录 KDF busy HTTP 响应

- **WHEN** 登录 use case 返回 `password.ErrPasswordKDFBusy`
- **THEN** 认证 HTTP 边界 MUST 返回 `503 Service Unavailable`
- **AND** 响应 envelope MUST 使用服务不可用错误分类和认证服务繁忙消息
- **AND** OpenAPI MUST 声明登录接口可能返回 503

#### Scenario: controller 测试验证 use case 调用契约

- **WHEN** 认证 HTTP controller 测试覆盖登录、刷新、改密、退出当前会话或退出全部会话流程
- **THEN** 测试 MUST 使用 `auth/transport/http` 测试包内的 `gomock` 生成 mock 设置 use case expectation
- **AND** 测试 MUST 通过 expectation、matcher 或 `DoAndReturn` 验证命令归一化、client context 注入和错误映射
- **AND** 测试 MUST NOT 通过手写 `stubAuthUseCases` 或只服务于该 stub 的状态字段表达调用契约
