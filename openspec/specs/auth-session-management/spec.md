## Purpose

定义 user-service 的认证会话能力，覆盖登录、令牌签发、刷新、退出、改密、会话状态和 token version 校验。

## Requirements

### Requirement: 用户登录与令牌签发

系统 MUST 提供用户名密码登录能力，并在凭证、用户状态和会话策略校验通过后签发访问令牌与刷新令牌。系统 MUST 将密码 KDF 资源池繁忙视为临时服务不可用，而不是无效凭据。

#### Scenario: 登录成功

- **WHEN** 用户提供合法用户名和正确密码，且用户状态允许登录
- **THEN** 系统 MUST 创建会话、签发 access token 与 refresh token，并返回会话相关过期时间

#### Scenario: 凭证错误

- **WHEN** 用户名不存在或密码不匹配
- **THEN** 系统 MUST 拒绝登录并返回一致的认证错误，且 MUST NOT 泄露具体凭证匹配细节

#### Scenario: 用户状态禁止登录

- **WHEN** 用户存在但状态不允许登录
- **THEN** 系统 MUST 拒绝签发令牌并返回明确的状态相关错误

#### Scenario: 强制改密用户登录

- **WHEN** 用户凭据有效但账号状态要求强制修改密码
- **THEN** 系统 MUST 只签发 subject 为 `password_change` 的受限 token，不得创建普通 refresh session，也不得返回 refresh token

#### Scenario: 强制改密登录返回专用 code

- **WHEN** 用户凭据有效但账号状态要求强制修改密码
- **THEN** 登录响应 MUST 返回 HTTP `200 OK`
- **AND** 登录响应 envelope MUST 携带 `CodePasswordChangeRequired`
- **AND** 登录响应 envelope 的 `success` MUST 为 `false`
- **AND** 登录响应 MUST NOT 使用 `CodeOK` 表达该分支
- **AND** 登录响应 MUST 携带 subject 为 `password_change` 的受限 token 数据
- **AND** 登录响应 MUST NOT 携带 refresh token

#### Scenario: 普通登录仍返回成功 code

- **WHEN** 用户凭据有效且账号状态允许普通登录
- **THEN** 登录响应 envelope MUST 携带 `CodeOK`
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

### Requirement: 令牌刷新

系统 MUST 支持使用有效 refresh token 换取新的访问令牌，并校验会话状态、token version 和过期时间。

#### Scenario: 刷新成功

- **WHEN** 调用方提交有效且未过期的 refresh token
- **THEN** 系统 MUST 验证对应会话仍有效，并签发新的 access token

#### Scenario: refresh token 已撤销

- **WHEN** refresh token 对应会话已退出、被撤销或不存在
- **THEN** 系统 MUST 拒绝刷新并返回认证失败

#### Scenario: token version 不匹配

- **WHEN** token 中携带的 token version 与当前用户凭证版本不一致
- **THEN** 系统 MUST 拒绝刷新或受保护访问

#### Scenario: refresh session 与 token claims 一致性

- **WHEN** refresh token 已通过 JWT 解析
- **THEN** 系统 MUST 校验 Redis refresh session 存在，且 session 中的 `user_id`、`session_id`、`token_version` MUST 与 token claims 一致；任一不一致 MUST 拒绝续期

#### Scenario: refresh rotation 原子性

- **WHEN** refresh token rotation 已启用，且新 token 已签发但 Redis session 原子替换失败
- **THEN** 系统 MUST NOT 向客户端返回已签发的新 token，并 MUST 按无效 token 或会话错误处理

### Requirement: 会话与 token version 策略

系统 MUST 在 auth application 中拥有 token version 校验、refresh session 生命周期、每用户活跃 refresh session 上限和会话撤销语义。受保护路由的 token version 本地缓存 MUST 使用有容量上限的 `common/runtime/localcache` loading cache，并且 MUST 将 Redis token version 投影和 PostgreSQL 当前值作为回源路径。user-service auth/provider 边界 MUST 拥有 `auth_token_version` 缓存实例名，并 MUST 在缺少该配置实例时拒绝服务装配。`auth.token_version_cache_ttl` MUST 允许正数 duration 表示显式 Redis token version 投影 TTL，并 MUST 允许非正数 duration 表示使用服务默认 TTL；非正数配置 MUST NOT 创建永久 Redis token version 投影。auth application port MUST 将 PostgreSQL token version 持久化、Redis token version 投影和 refresh session 生命周期拆分为最小依赖接口，业务组件 MUST 只依赖自身所需的 port。

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

### Requirement: 认证 command use case 最小依赖边界

认证 command use case MUST 通过自身 constructor 声明最小依赖，并且结构体 MUST 只保存该 use case 实际需要的 collaborator。系统 MUST NOT 通过跨多个 command use case 的共享依赖容器向单个 use case 暴露无关的 credential、token、session、metrics 或配置依赖。

#### Scenario: 退出当前会话不能访问无关凭证依赖

- **WHEN** 实现或维护退出当前会话 use case
- **THEN** 该 use case MUST 只注入撤销当前 refresh session 和记录退出指标所需的依赖
- **AND** 该 use case MUST NOT 通过共享依赖容器访问 credential verifier、token issuer、refresh token rotation 配置或其他无关 collaborator

#### Scenario: 登录与刷新复用签发逻辑不扩大依赖面

- **WHEN** 登录或刷新 use case 复用 access token、refresh token 和 refresh session 创建逻辑
- **THEN** 复用逻辑 MUST 以显式参数或窄 helper 表达所需的 token issuer 与 session lifecycle
- **AND** 复用逻辑 MUST NOT 要求调用方持有覆盖其他 use case 的公共依赖容器

#### Scenario: Fx 装配表达 use case 真实依赖

- **WHEN** user-service 装配 auth command use case
- **THEN** Fx provider MUST 直接提供各 use case constructor 所需的最小参数结构
- **AND** 系统 MUST NOT 继续 provide 或消费 `UseCaseDeps` 作为 auth command use case 的公共装配入口

#### Scenario: 测试 fixture 不隐藏依赖边界

- **WHEN** command 包测试构造登录、刷新、改密、退出当前会话或退出全部会话 use case
- **THEN** 测试 MUST 按被测 use case 的最小 constructor 参数直接提供 mock collaborator
- **AND** 测试 MUST NOT 通过公共 `UseCaseDeps` fixture 隐藏单个 use case 的真实依赖面

### Requirement: 会话退出

系统 MUST 支持退出当前会话和退出全部会话，并保证退出后令牌无法继续访问受保护资源。全部会话退出 MUST 以 PostgreSQL token version 递增作为旧 access token 失效的主事实，并 MUST 明确表达 Redis token version 投影刷新和 refresh session 删除失败时的最终一致处理语义。

#### Scenario: 退出当前会话

- **WHEN** 已认证用户请求退出当前会话
- **THEN** 系统 MUST 撤销当前 refresh session，且 MUST NOT 递增用户 `token_version`

#### Scenario: 退出全部会话

- **WHEN** 已认证用户请求退出全部会话
- **THEN** 系统 MUST 递增用户 `token_version` 并撤销该用户的所有活跃 refresh session，使旧 token 无法继续刷新或访问
- **AND** PostgreSQL token version 递增成功后，旧 access token MUST 因 token version 不匹配而无法继续访问受保护资源
- **AND** Redis token version 投影刷新或 refresh session 删除失败时，系统 MUST 返回、记录或暴露可观察的投影失败信号，使调用方和测试能区分主事实成功与投影失败

#### Scenario: 全部会话后台清理

- **WHEN** 用户执行全部会话退出
- **THEN** 安全失效 MUST NOT 依赖后台 workerpool；Redis refresh session key 的批量物理删除 MAY 通过 auth 专用 purge workerpool 异步执行

#### Scenario: 重复退出

- **WHEN** 用户对已撤销或不存在的会话重复执行退出操作
- **THEN** 系统 MUST 返回稳定结果或明确错误，并 MUST NOT 恢复已撤销会话

### Requirement: 密码变更

系统 MUST 支持已认证用户修改密码，并在密码变更后更新凭证和 token version 以失效旧令牌。

#### Scenario: 修改密码成功

- **WHEN** 已认证用户提供正确旧密码和满足策略的新密码
- **THEN** 系统 MUST 更新密码哈希、提升 token version，并使旧令牌失效

#### Scenario: 旧密码错误

- **WHEN** 用户修改密码时提供的旧密码不正确
- **THEN** 系统 MUST 拒绝修改并保持原密码和 token version 不变

#### Scenario: 新密码不合规

- **WHEN** 新密码不满足密码策略
- **THEN** 系统 MUST 拒绝修改并返回校验错误

### Requirement: 认证 HTTP 边界

系统 MUST 将公开认证路由和受保护认证路由分开挂载，并通过共享认证中间件保护需要 bearer token 的接口。认证 HTTP 边界 MUST 区分凭据认证失败和认证服务临时不可用。认证 HTTP controller 测试 MUST 使用 feature-local `gomock` 生成 mock 表达 use case 调用契约，不得保留手写 `stubAuthUseCases` 兼容入口。

#### Scenario: 公开登录路由

- **WHEN** 调用方访问登录或刷新等公开认证入口
- **THEN** 系统 MUST 允许请求进入认证 controller 并在业务层完成凭证校验

#### Scenario: 受保护认证路由

- **WHEN** 调用方访问退出、修改密码或其他受保护认证入口
- **THEN** 系统 MUST 先通过 JWT、auth config 和 token version validator 校验

#### Scenario: 无效 bearer token

- **WHEN** 受保护认证路由收到缺失、过期、格式错误或签名无效的 bearer token
- **THEN** 系统 MUST 在进入业务处理前拒绝请求

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

### Requirement: 认证 command 测试协作者契约

认证 command use case 测试 MUST 使用该包已有 `mockgen` 生成物表达 credential、refresh session、token version、token issuer/verifier、metrics 和 lifecycle 等外部协作者契约。测试 MUST NOT 通过手写 collaborator double 兼容或隐藏这些 port 的调用、失败路径、调用顺序或指标记录。

#### Scenario: 登录测试使用生成 mock

- **WHEN** command 包测试覆盖登录成功、凭证失败、用户状态失败、强制改密或 token 签发失败路径
- **THEN** 测试 MUST 通过生成 mock 的 expectation 表达 credential store、password verifier、token issuer、refresh session store 和 metrics 调用
- **AND** 测试 MUST NOT 使用手写 credential、session、token issuer 或 metrics collaborator double 承载这些外部依赖

#### Scenario: 刷新测试使用生成 mock

- **WHEN** command 包测试覆盖 refresh token 解析、session 查询、token version 校验、rotation 或 rotation 失败路径
- **THEN** 测试 MUST 通过生成 mock 的 expectation、`gomock.InOrder`、matcher 或 `DoAndReturn` 表达 token verifier/issuer、refresh session store、token version store/cache 和 metrics 调用
- **AND** refresh session 与 token claims 一致性、rotation 原子失败和指标失败 reason MUST 由 expectation 或 matcher 明确断言

#### Scenario: 改密与退出测试使用生成 mock

- **WHEN** command 包测试覆盖修改密码、退出当前会话或退出全部会话的成功与失败路径
- **THEN** 测试 MUST 通过生成 mock 表达 credential store、token version store/cache、refresh session store、session lifecycle 和 metrics 调用
- **AND** token version 递增、本地缓存失效、Redis 投影刷新、refresh session 删除和 purge 提交失败等安全相关行为 MUST 通过 expectation 或 `DoAndReturn` 明确断言

#### Scenario: 保留纯构造 helper

- **WHEN** command 包测试需要复用用户凭据、token claims、auth session、密码 verifier 或 use case 构造逻辑
- **THEN** 测试 MAY 保留不实现外部 collaborator port 的纯构造 helper 或真实轻量纯函数依赖
- **AND** 这些 helper MUST NOT 替代 `mockgen` 生成物记录 collaborator 调用或隐藏失败注入

### Requirement: session lifecycle 测试使用生成 mock 表达端口契约

`auth-session-management` 的 `user-service/internal/features/auth/application/sessions` 包测试 MUST 使用已有 gomock 生成物或真实领域值验证 session lifecycle 端口交互，不得在 `lifecycle_test.go` 中继续保留 `sessionUserTestStore`、`authSessionTestStore` 或 `tokenVersionRecordingInvalidator` 手写兼容路径。测试 MUST 通过 expectation 明确表达 token version 回源、refresh session 旋转、全量撤销和本地 token version cache 失效语义。

#### Scenario: token version 回源测试

- **WHEN** session lifecycle 测试覆盖 Redis token version 投影 miss 后回源 PostgreSQL 并回填投影的路径
- **THEN** 测试 MUST 使用生成的 `UserTokenVersionStore` 和 `TokenVersionCache` mock 表达读取、回源和回填 expectation
- **AND** 测试 MUST NOT 使用手写 store 状态字段替代这些端口调用断言

#### Scenario: refresh session 旋转测试

- **WHEN** session lifecycle 测试覆盖 refresh session rotation 创建新 session 并替换旧 session 的路径
- **THEN** 测试 MUST 使用生成的 `RefreshSessionStore` mock 表达 session 参数、rotation 结果和失败分支 expectation
- **AND** 测试 MUST 使用 matcher 或 `DoAndReturn` 捕获需要校验的领域值

#### Scenario: 全量撤销和本地缓存失效测试

- **WHEN** session lifecycle 测试覆盖全部会话退出、token version 提升、Redis token version 投影刷新和本地 cache 失效
- **THEN** 测试 MUST 使用生成的 store mock 和 `TokenVersionLocalInvalidator` mock 表达调用顺序、失败信号和 cache invalidation expectation
- **AND** 测试 MUST NOT 通过 `tokenVersionRecordingInvalidator` 记录字段表达本地 cache 失效

### Requirement: token version validator 测试替身一致性

token version validator 的单元测试 MUST 使用本包已有 gomock 生成物表达 `UserTokenVersionStore` 与 `TokenVersionCache` 依赖交互，并 MUST 保留真实 `localcache` 实例验证本地缓存行为。测试 MUST NOT 保留 `tokenVersionUserTestStore` 或 `tokenVersionSessionTestStore` 手写兼容替身。

#### Scenario: Redis miss 后回源并回填

- **WHEN** token version validator 在本地缓存未命中且 Redis token version 投影未命中时执行校验
- **THEN** 测试 MUST 通过 gomock expectation 表达 Redis miss、PostgreSQL 当前值回源和 Redis 投影回填
- **AND** 测试 MUST 继续使用真实 `localcache` 验证后续本地缓存命中

#### Scenario: singleflight 合并并发回源

- **WHEN** 同一用户的多个并发 token version 校验同时触发回源路径
- **THEN** 测试 MUST 通过 `DoAndReturn`、channel、mutex 或 atomic 计数表达并发控制
- **AND** 测试 MUST 断言 PostgreSQL 当前值回源被 singleflight 合并

#### Scenario: 按用户隔离并发校验

- **WHEN** 不同用户的并发 token version 校验同时触发回源路径
- **THEN** 测试 MUST 表达不同用户之间不共享 singleflight 结果
- **AND** 每个用户的依赖调用 MUST 通过 gomock expectation 独立断言

#### Scenario: 失效后重新加载

- **WHEN** token version validator 的本地 token version 缓存被失效后再次校验同一用户
- **THEN** 测试 MUST 通过 gomock expectation 表达重新读取 Redis 或 PostgreSQL 当前值
- **AND** 旧本地缓存值 MUST NOT 继续作为校验依据

### Requirement: 认证包组织

系统 MUST 将认证 application 职责按 `command`、`authctx`、`credentials`、`tokens`、`sessions`、`validators` 和 `ports.go` 组织，避免 transport 或 Redis adapter 承载认证业务编排。

#### Scenario: 新增凭据行为

- **WHEN** 新行为需要校验或更新密码凭据
- **THEN** 代码 MUST 位于 `application/credentials` 或 command 编排中，不得放入 HTTP transport 或 Redis adapter

#### Scenario: 新增 token 或会话行为

- **WHEN** 新行为涉及 JWT 签发解析、refresh session 生命周期、token version fallback 或会话撤销
- **THEN** 业务语义 MUST 位于 `application/tokens`、`application/sessions`、`application/validators` 或 command 编排中，Redis adapter 只实现存储契约
