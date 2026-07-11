## Purpose

定义 user-service 的认证会话能力，覆盖登录、令牌签发、刷新、退出、改密、会话状态和 token version 校验。
## Requirements
### Requirement: 用户登录与令牌签发

系统 MUST 提供用户名密码登录能力，并在凭证、用户状态和会话策略校验通过后签发访问令牌与刷新令牌。登录 use case MUST 使用登录专属结果字段表达是否需要强制改密；登录失败仍 MUST 通过错误返回。系统 MUST 将密码 KDF 资源池繁忙视为临时服务不可用，而不是无效凭据。

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

系统 MUST 在 auth application 中拥有 token version 校验、refresh session 生命周期、每用户活跃 refresh session 上限和会话撤销语义。受保护路由的 token version 本地缓存 MUST 使用有容量上限的 `common/runtime/localcache` loading cache，并且 MUST 将 Redis token version 投影和 PostgreSQL 当前值作为回源路径。user-service auth/provider 边界 MUST 拥有 `auth_token_version` 缓存实例名，并 MUST 在缺少该配置实例时拒绝服务装配。`auth.token_version_cache_ttl` MUST 允许正数 duration 表示显式 Redis token version 投影 TTL，并 MUST 允许非正数 duration 表示使用服务默认 TTL；非正数配置 MUST NOT 创建永久 Redis token version 投影。auth application port MUST 将 PostgreSQL token version 持久化、Redis token version 投影和 refresh session 生命周期拆分为最小依赖接口，业务组件 MUST 只依赖自身所需的 port。token version 本地缓存失效接口 MUST 返回失败错误；会话撤销流程 MUST 记录本地失效失败并将其纳入投影错误返回，MUST NOT 静默忽略本地 cache 删除失败。

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

#### Scenario: token version 本地缓存失效失败

- **WHEN** 用户执行全部会话退出或强制改密导致系统尝试删除本实例本地 token version cache，且本地 cache 删除返回错误
- **THEN** 系统 MUST 记录包含 `user_id` 和错误信息的日志
- **AND** 会话撤销流程 MUST 将该错误纳入投影错误返回
- **AND** 系统 MUST NOT 继续静默忽略本地 token version cache 删除失败

#### Scenario: session lifecycle 必需本地失效器

- **WHEN** auth application 构造 refresh session lifecycle
- **THEN** `TokenVersionLocalInvalidator` MUST 作为必需依赖提供
- **AND** 缺失该依赖时系统 MUST fail-fast 或拒绝装配，MUST NOT 静默跳过本地 token version cache 失效

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

系统 MUST 支持退出当前会话和退出全部会话，并保证退出后令牌无法继续访问受保护资源。全部会话退出 MUST 以 PostgreSQL token version 递增作为旧 access token 失效的主事实，并 MUST 明确表达本地 token version cache 失效、Redis token version 投影刷新和 refresh session 删除失败时的最终一致处理语义。

#### Scenario: 退出当前会话

- **WHEN** 已认证用户请求退出当前会话
- **THEN** 系统 MUST 撤销当前 refresh session，且 MUST NOT 递增用户 `token_version`

#### Scenario: 退出全部会话

- **WHEN** 已认证用户请求退出全部会话
- **THEN** 系统 MUST 递增用户 `token_version` 并撤销该用户的所有活跃 refresh session，使旧 token 无法继续刷新或访问
- **AND** PostgreSQL token version 递增成功后，旧 access token MUST 因 token version 不匹配而无法继续访问受保护资源
- **AND** 本地 token version cache 失效、Redis token version 投影刷新或 refresh session 删除失败时，系统 MUST 返回、记录或暴露可观察的投影失败信号，使调用方和测试能区分主事实成功与投影失败

#### Scenario: 全部会话退出投影失败返回撤销不完整

- **WHEN** 全部会话退出已完成 PostgreSQL token version 主事实递增，但本地 token version cache 失效、Redis token version 投影刷新或 refresh session 删除投影返回错误
- **THEN** logout all use case MUST 返回 `authdomain.ErrSessionRevocationIncomplete`，且 MUST NOT 返回普通成功结果
- **AND** 错误链 MUST 保留底层投影错误，供日志、metrics 和测试定位失败来源
- **AND** logout metrics MUST 将该结果记录为失败并能分类为撤销不完整

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

### Requirement: shared auth 和 password 测试断言迁移

`common/security/auth` 和 `common/security/password` 的测试 MUST 使用统一断言规范验证 JWT、token subject、token version、password KDF、密码哈希和密码校验行为。断言迁移 MUST 保持 JWT claims 解析、token subject 校验、token version 校验、Argon2id 参数、KDF 资源预算、队列繁忙错误、密码哈希编码和常量时间校验语义不变。

#### Scenario: JWT 和 token 断言

- **WHEN** `common/security/auth` 测试验证 token 签发、解析、过期、subject、`jti`、token version 或错误路径
- **THEN** 测试 MUST 使用 `require` 表达错误、claims、时间、subject 和版本匹配断言
- **AND** 迁移 MUST NOT 改变 token 格式、claims 名称、过期校验或 subject 隔离语义

#### Scenario: password KDF 断言

- **WHEN** `common/security/password` 测试验证 Argon2id 哈希、校验、参数解析、资源预算或队列繁忙路径
- **THEN** 测试 MUST 使用 `require` 表达构造错误、哈希格式、校验结果、错误类型和资源边界断言
- **AND** 迁移 MUST NOT 改变 Argon2id 参数、哈希编码、队列上限、并发上限或 `ErrPasswordKDFBusy` 语义

#### Scenario: 安全失败路径不放宽

- **WHEN** auth 或 password 测试迁移历史 `t.Fatal`、`t.Error` 手写判断
- **THEN** 测试 MUST 保持原有安全失败路径覆盖
- **AND** 迁移 MUST NOT 通过兼容 helper、生产分支或弱化断言使非法 token、错误密码、过期 token 或资源繁忙路径被放行

### Requirement: auth 测试语义化断言规范

auth 范围内的 Go 测试 MUST 使用语义化断言验证认证会话、credential、refresh session、token、password change、HTTP response、Redis/PostgreSQL adapter、metrics 和 provider 行为。测试 MUST NOT 通过旧手写 if 断言、机械 `Fail` / `Failf` 替换或兼容 helper 隐藏失败信息。

#### Scenario: application 测试使用 require 表达安全路径断言

- **WHEN** auth application、credentials、sessions、tokens、validators 或 authctx 测试覆盖登录、刷新、强制改密、改密、退出、token version 或 client/session context 行为
- **THEN** 测试 MUST 优先使用 `testify/require` 的错误、对象、布尔、集合、字符串和类型断言表达预期
- **AND** 后续检查依赖当前结果时 MUST 使用阻塞式 `require` 避免级联失败

#### Scenario: HTTP controller 和 input 测试使用语义化断言

- **WHEN** auth HTTP transport 测试覆盖请求输入归一化、use case 调用、HTTP status、response envelope、强制改密响应、错误码或响应 data 字段
- **THEN** 测试 MUST 使用 `require` 或必要时 `assert` 验证状态码、envelope code、success 标记、data shape 和字段存在性
- **AND** 测试 MUST NOT 增加旧 auth HTTP 字段、旧错误码、旧 token 类型或旧状态兼容断言

#### Scenario: adapter 和 provider 测试使用语义化断言

- **WHEN** auth Redis/PostgreSQL infrastructure、metrics、Fx/provider 或 `user-service/internal/providers/auth_test.go` 测试覆盖 store、key schema、TTL、token version cache、credential update、metrics collector 或 provider 构造行为
- **THEN** 测试 MUST 使用 `require` 或必要时 `assert` 表达错误、相等性、包含关系、空值、非空值、长度和布尔预期
- **AND** 生产 Redis key、PostgreSQL schema、JWT claims、配置和 provider 装配语义 MUST 保持不变

#### Scenario: 剩余 testing.T 直接失败调用受限

- **WHEN** auth 目标范围内的 `_test.go` 文件保留 `t.Fatal`、`t.Fatalf`、`t.Error`、`t.Errorf`、`require.Fail`、`require.Failf`、`assert.Fail` 或 `assert.Failf`
- **THEN** 每个剩余命中 MUST 属于 `docs/TESTING.md` 允许的自定义测试控制流、特殊诊断输出或测试辅助工具场景
- **AND** change tasks MUST 列明剩余例外，证明其不是可由现有语义化断言清晰表达的普通断言

### Requirement: 认证领域状态判断测试覆盖

`auth-session-management` 的 auth domain 测试 MUST 直接覆盖 `UserCredential` 对用户状态的领域判断。测试 MUST 固定普通登录、强制改密登录和受限改密流程当前使用的状态语义，并 MUST NOT 通过旧状态别名、旧 token 类型复用、旧错误码、旧字段或兼容 helper 表达预期。

#### Scenario: 普通状态允许普通登录

- **WHEN** `UserCredential.Status` 为 `identity.UserStatusNormal`
- **THEN** `CanLogin` MUST 返回 `true`
- **AND** `RequiresPasswordChange` MUST 返回 `false`
- **AND** `CanChangePassword` MUST 返回 `false`

#### Scenario: 强制改密状态只允许受限改密流程

- **WHEN** `UserCredential.Status` 为 `identity.UserStatusMustChangePassword`
- **THEN** `CanLogin` MUST 返回 `false`
- **AND** `RequiresPasswordChange` MUST 返回 `true`
- **AND** `CanChangePassword` MUST 返回 `true`

#### Scenario: 不可登录状态拒绝认证流程

- **WHEN** `UserCredential.Status` 为 `identity.UserStatusDisabled` 或未知状态值
- **THEN** `CanLogin` MUST 返回 `false`
- **AND** `RequiresPasswordChange` MUST 返回 `false`
- **AND** `CanChangePassword` MUST 返回 `false`

#### Scenario: auth domain 测试使用语义化断言

- **WHEN** auth domain 测试覆盖 `UserCredential` 状态判断
- **THEN** 测试 MUST 使用 `testify/require` 或等价语义化断言表达布尔和值预期
- **AND** 测试 MUST NOT 使用机械 `Fail` / `Failf` 替换或旧手写断言兼容 helper

### Requirement: 认证路由注册测试覆盖
系统 MUST 使用 router 包测试覆盖认证公开路由和认证保护路由在 user-service 聚合路由中的注册结果，确保认证入口仅存在于当前 `/api/v1/auth` 路由图中。

#### Scenario: 认证公开路由注册
- **WHEN** `registerV1Routes` 注册当前 `/api/v1` 路由组
- **THEN** 测试 MUST 验证登录、refresh 和强制改密入口注册在 `/api/v1/auth` 下
- **AND** 测试 MUST 验证这些公开认证路由不经过普通 access token 认证中间件

#### Scenario: 认证保护路由注册
- **WHEN** `registerV1Routes` 注册当前 `/api/v1` 路由组
- **THEN** 测试 MUST 验证退出当前会话和退出全部会话入口注册在 `/api/v1/auth` 下
- **AND** 测试 MUST 验证这些路由进入当前认证中间件链
- **AND** 测试 MUST NOT 为旧认证绕过路径或 `/api`、`/v1` 旧别名保留兼容断言

### Requirement: 认证会话 E2E flow 断言规范

系统 MUST 使用语义化断言覆盖 user-service E2E HTTP flow 中的认证会话行为，包括普通登录、强制改密登录、修改密码、旧密码登录失败、登出当前会话和 refresh token 失效。断言迁移 MUST 保持当前认证会话、token、错误码和 response envelope 语义不变，且 MUST 以 envelope `CodePasswordChangeRequired` 作为强制改密登录分支的当前语义。

#### Scenario: 普通登录 token 断言

- **WHEN** E2E flow 使用合法用户名和密码完成普通登录
- **THEN** 测试 MUST 使用 `require.NotEmpty`、`require.Equal`、`require.Greater` 或必要 `assert` 验证 access token、refresh token、token type 和 expires_in
- **AND** 测试 MUST NOT 接受缺失 refresh token、旧 token type、旧错误码或旧响应字段兼容分支

#### Scenario: 强制改密登录断言

- **WHEN** E2E flow 使用强制改密用户凭据登录
- **THEN** 测试 MUST 使用语义化断言验证 HTTP `200 OK`、`success=false`、`CodePasswordChangeRequired`、受限 access token metadata 和空 refresh token
- **AND** 测试 MUST NOT 接受 `success=true`、`CodeOK`、响应 data 状态枚举或旧 `password_change_required` 兼容字段

#### Scenario: 改密、登出和 refresh 失败断言

- **WHEN** E2E flow 完成改密、使用旧密码重试登录、登出当前会话并使用旧 refresh token 刷新
- **THEN** 测试 MUST 使用语义化断言验证改密成功、旧密码认证失败、登出成功和 refresh token 失效的当前 HTTP status 与应用错误码
- **AND** 迁移 MUST NOT 改变 refresh session、token version、password change token 或 logout 运行时语义

### Requirement: 认证会话测试时间确定性

认证会话、refresh session 和 token version validator 测试 MUST 避免使用固定 `time.Sleep` 作为 Redis session 排序、本地缓存过期或异步状态变化的唯一依据。测试 MUST 使用确定性 score/clock、可观察条件或真实 cache 的 eventually-style 断言表达预期。

#### Scenario: refresh session 上限裁剪测试使用确定性顺序
- **WHEN** Redis refresh session store 测试验证超过每用户活跃 session 上限时裁剪最旧 session
- **THEN** 测试 MUST 使用确定性 Redis score、可注入时间输入或可观察排序条件建立 session 顺序
- **AND** 测试 MUST NOT 依赖循环中的固定 `time.Sleep` 制造不同创建时间

#### Scenario: token version 本地缓存过期测试使用条件等待
- **WHEN** token version validator 测试验证本地缓存 TTL 过期后重新回源
- **THEN** 测试 MUST 使用 `require.Eventually` 或等价条件等待直到重新回源发生
- **AND** 测试 MUST 保留真实 `localcache` 实例验证缓存行为
- **AND** 测试 MUST NOT 在固定 `time.Sleep` 后直接断言回源调用次数

#### Scenario: 认证测试不引入测试专用生产 API
- **WHEN** 认证测试需要控制时间、顺序或异步状态
- **THEN** 测试 MUST 优先使用现有可观测存储状态、测试数据构造、gomock expectation、通道或局部 helper
- **AND** 正式代码 MUST NOT 仅为了测试新增无运行时职责的全局 clock、test hook 或兼容分支

### Requirement: 强制改密一次性会话

系统 MUST 为强制改密流程创建服务端一次性 password-change session，并将 `password_change` token 的 `jti`、`session_id`、`user_id` 和 `token_version` 与 Redis 状态绑定。password-change session MUST 使用独立短 TTL，并 MUST NOT 复用 refresh session 存储语义、refresh session 上限裁剪或 refresh token TTL。

#### Scenario: 强制改密登录创建一次性会话
- **WHEN** 用户凭据有效但账号状态要求强制修改密码
- **THEN** 系统 MUST 签发 subject 为 `password_change` 的受限 token
- **AND** 系统 MUST 创建与该 token `jti`、`session_id`、`user_id` 和 `token_version` 一致的 Redis password-change session
- **AND** 系统 MUST NOT 创建普通 refresh session
- **AND** 系统 MUST NOT 返回 refresh token

#### Scenario: 一次性会话创建失败
- **WHEN** 系统无法创建 Redis password-change session
- **THEN** 登录 MUST 失败
- **AND** 系统 MUST NOT 向客户端返回已签发的 password-change token

#### Scenario: 独立改密 token TTL
- **WHEN** 系统签发 password-change token 或创建 password-change session
- **THEN** token 和 session MUST 使用 `auth.jwt.password_change_token_ttl`
- **AND** 系统 MUST NOT 使用 `auth.jwt.access_token_ttl` 或 `auth.jwt.refresh_token_ttl` 作为 password-change token TTL

#### Scenario: 非正数改密 token TTL
- **WHEN** `auth.jwt.password_change_token_ttl` 未配置、配置为 `0` 或配置为负数
- **THEN** 系统 MUST 使用 5 分钟作为默认 password-change token TTL
- **AND** 系统 MUST NOT 创建无过期时间的 password-change token 或 password-change session

### Requirement: 强制改密 token 原子消费

系统 MUST 在更新密码前原子消费 password-change session。消费 MUST 同时校验 `jti`、`session_id`、`user_id` 和 `token_version`，任一不匹配、会话不存在、会话过期、会话已撤销或会话已被消费时，系统 MUST 返回统一无效凭据错误，且 MUST NOT 泄露具体失败原因。

#### Scenario: 首次消费成功
- **WHEN** 调用方提交有效且未过期的 password-change token，且 Redis password-change session 与 token claims 完全一致
- **THEN** 系统 MUST 原子删除该 password-change session
- **AND** 系统 MAY 继续执行密码更新

#### Scenario: 重复消费被拒绝
- **WHEN** 同一个 password-change token 已被成功消费后再次用于改密
- **THEN** 系统 MUST 拒绝改密并返回统一无效凭据
- **AND** 系统 MUST NOT 再次更新密码或递增 `token_version`

#### Scenario: 过期会话被拒绝
- **WHEN** password-change token 未通过 JWT 过期校验，或对应 Redis password-change session 已过期
- **THEN** 系统 MUST 拒绝改密并返回统一无效凭据
- **AND** 系统 MUST NOT 更新密码或递增 `token_version`

#### Scenario: 撤销会话被拒绝
- **WHEN** password-change session 已被服务端撤销或删除
- **THEN** 系统 MUST 拒绝改密并返回统一无效凭据
- **AND** 系统 MUST NOT 更新密码或递增 `token_version`

#### Scenario: claims 不一致被拒绝
- **WHEN** password-change token 中的 `jti`、`session_id`、`user_id` 或 `token_version` 与 Redis password-change session 不一致
- **THEN** 系统 MUST 拒绝改密并返回统一无效凭据
- **AND** 系统 MUST NOT 更新密码或递增 `token_version`

#### Scenario: 并发消费只有一个成功
- **WHEN** 多个请求并发使用同一个有效 password-change token 执行改密
- **THEN** 系统 MUST 最多允许一个请求成功消费 password-change session
- **AND** 其他请求 MUST 返回统一无效凭据
- **AND** 系统 MUST 最多执行一次密码更新和一次 `token_version` 递增

### Requirement: 强制改密凭据条件更新

系统 MUST 在强制改密更新凭据时校验用户仍处于强制改密状态，且当前 PostgreSQL `token_version` 与 password-change token claims 中的旧版本一致。条件不满足时，系统 MUST 返回统一无效凭据，并 MUST NOT 更新密码、状态或 `token_version`。

#### Scenario: 状态和版本匹配时更新
- **WHEN** password-change session 已成功消费，用户仍处于强制改密状态，且 PostgreSQL `token_version` 等于 token claims 中的旧版本
- **THEN** 系统 MUST 更新密码哈希
- **AND** 系统 MUST 将用户状态恢复为正常
- **AND** 系统 MUST 递增 `token_version`

#### Scenario: 状态不再要求改密
- **WHEN** password-change session 已成功消费，但用户状态不再要求强制改密
- **THEN** 系统 MUST 拒绝改密并返回统一无效凭据
- **AND** 系统 MUST NOT 更新密码或递增 `token_version`

#### Scenario: 旧 token version 不匹配
- **WHEN** password-change session 已成功消费，但 PostgreSQL 当前 `token_version` 不等于 token claims 中的旧版本
- **THEN** 系统 MUST 拒绝改密并返回统一无效凭据
- **AND** 系统 MUST NOT 更新密码或递增 `token_version`

### Requirement: 强制改密后安全撤销结果

系统 MUST 在强制改密成功更新凭据后刷新 token version 投影、失效本地 token version cache 并删除该用户 refresh sessions。任一撤销投影步骤失败时，系统 MUST 返回可观察的安全撤销未完成错误，MUST NOT 返回普通 `Changed: true` 成功结果。

#### Scenario: 撤销全部成功
- **WHEN** 强制改密凭据更新成功，且本地 token version cache 失效、Redis token version 投影刷新和 refresh session 删除均成功
- **THEN** 系统 MUST 返回改密成功结果
- **AND** 旧 access token MUST 因 `token_version` 不匹配而无法继续访问受保护资源
- **AND** 旧 refresh token MUST 无法继续刷新

#### Scenario: token version 投影失败
- **WHEN** 强制改密凭据更新成功，但 Redis token version 投影刷新失败
- **THEN** 系统 MUST 尝试删除 Redis token version 投影
- **AND** 系统 MUST 返回安全撤销未完成错误
- **AND** 系统 MUST NOT 返回普通 `Changed: true` 成功结果

#### Scenario: refresh session 删除失败
- **WHEN** 强制改密凭据更新成功，但删除该用户 refresh sessions 失败
- **THEN** 系统 MUST 返回安全撤销未完成错误
- **AND** 系统 MUST NOT 返回普通 `Changed: true` 成功结果

#### Scenario: 本地 token version cache 失效失败
- **WHEN** 强制改密凭据更新成功，但本实例本地 token version cache 失效失败
- **THEN** 系统 MUST 返回安全撤销未完成错误
- **AND** 系统 MUST NOT 返回普通 `Changed: true` 成功结果

#### Scenario: HTTP 错误映射
- **WHEN** 强制改密返回安全撤销未完成错误
- **THEN** 认证 HTTP 边界 MUST 返回 `503 Service Unavailable`
- **AND** 响应 MUST 表达认证服务暂时无法完成安全撤销
- **AND** 响应 MUST NOT 泄露 Redis key、session ID、jti、SQL、stacktrace 或内部错误文本

### Requirement: 认证错误应用错误渲染

系统 MUST 将认证、会话、token 和撤销相关稳定错误表达为可由共享 response helper 直接渲染的应用错误，并保持 auth HTTP boundary 无专用 sentinel-to-HTTP 兼容映射。认证错误 MUST 携带稳定 `Kind`、`Reason`、`Code` 和中文公开 `Message`，且 MUST 保持 `errors.Is` 或应用错误 `Reason` 可供登录、refresh 和 logout metrics 分类。

#### Scenario: 无效凭据渲染为未认证响应

- **WHEN** 登录凭据校验返回 `authdomain.ErrInvalidCredentials`
- **THEN** 认证 HTTP 边界 MUST 通过 `response.Fail(c, err)` 渲染失败响应
- **AND** 响应 MUST 为 `401 Unauthorized` 和 `CodeUnauthenticated`
- **AND** 响应 message MUST 使用当前无效凭据中文公开文案
- **AND** 该错误 MUST 使用稳定 `Reason` 值 `invalid_credentials`
- **AND** 系统 MUST NOT 泄露用户名不存在、密码不匹配或用户状态拒绝的具体细节

#### Scenario: 用户状态拒绝保持无效凭据公开语义

- **WHEN** 登录凭据有效但用户状态不允许普通登录，且错误链包含 `authdomain.ErrUserStatusRejected`
- **THEN** 认证 HTTP 边界 MUST 返回 `401 Unauthorized` 和 `CodeUnauthenticated`
- **AND** 响应 message MUST 继续使用当前无效凭据中文公开文案
- **AND** metrics MUST 能通过 `errors.Is(err, authdomain.ErrUserStatusRejected)` 或稳定 `Reason` 值 `user_status_rejected` 分类该失败
- **AND** 系统 MUST NOT 向客户端暴露具体用户状态

#### Scenario: 缺失认证会话渲染为未认证响应

- **WHEN** 受保护认证 use case 返回 `authdomain.ErrMissingSession`
- **THEN** 认证 HTTP 边界 MUST 通过 `response.Fail(c, err)` 渲染失败响应
- **AND** 响应 MUST 为 `401 Unauthorized` 和 `CodeUnauthenticated`
- **AND** 响应 message MUST 使用当前登录状态失效中文公开文案
- **AND** 该错误 MUST 使用稳定 `Reason` 值 `missing_session`

#### Scenario: token 无效渲染为 token invalid 响应

- **WHEN** token 解析、password change token 校验或受保护认证流程返回 `authdomain.ErrTokenInvalid`
- **THEN** 认证 HTTP 边界 MUST 通过 `response.Fail(c, err)` 渲染失败响应
- **AND** 响应 MUST 为 `401 Unauthorized` 和 `CodeTokenInvalid`
- **AND** 响应 message MUST 使用当前登录状态失效中文公开文案
- **AND** 该错误 MUST 使用稳定 `Reason` 值 `auth_token_invalid`

#### Scenario: refresh session 不存在渲染为 token invalid 响应

- **WHEN** refresh token 对应会话不存在、已退出或已过期，且流程返回 `authdomain.ErrAuthSessionNotFound`
- **THEN** 认证 HTTP 边界 MUST 通过 `response.Fail(c, err)` 渲染失败响应
- **AND** 响应 MUST 为 `401 Unauthorized` 和 `CodeTokenInvalid`
- **AND** 响应 message MUST 使用当前登录状态失效中文公开文案
- **AND** 该错误 MUST 使用稳定 `Reason` 值 `auth_session_not_found`

#### Scenario: refresh session mismatch 渲染为 token invalid 响应

- **WHEN** refresh session 中的 `user_id`、`session_id` 或 `token_version` 与 token claims 不一致，且流程返回 `authdomain.ErrAuthSessionMismatch`
- **THEN** 认证 HTTP 边界 MUST 通过 `response.Fail(c, err)` 渲染失败响应
- **AND** 响应 MUST 为 `401 Unauthorized` 和 `CodeTokenInvalid`
- **AND** 响应 message MUST 使用当前登录状态失效中文公开文案
- **AND** 该错误 MUST 使用稳定 `Reason` 值 `auth_session_mismatch`
- **AND** refresh metrics MUST 继续能分类为 refresh session mismatch

#### Scenario: 强制改密一次性会话无效渲染为 token invalid 响应

- **WHEN** 强制改密流程返回 `authdomain.ErrPasswordChangeSessionNotFound` 或 `authdomain.ErrPasswordChangeSessionMismatch`
- **THEN** 认证 HTTP 边界 MUST 通过 `response.Fail(c, err)` 渲染失败响应
- **AND** 响应 MUST 为 `401 Unauthorized` 和 `CodeTokenInvalid`
- **AND** 响应 message MUST 使用当前登录状态失效中文公开文案
- **AND** 两类错误 MUST 分别使用稳定 `Reason` 值 `password_change_session_not_found` 和 `password_change_session_mismatch`

#### Scenario: 撤销不完整渲染为服务不可用响应

- **WHEN** 退出当前会话、退出全部会话或安全敏感撤销流程返回 `authdomain.ErrSessionRevocationIncomplete`
- **THEN** 认证 HTTP 边界 MUST 通过 `response.Fail(c, err)` 渲染失败响应
- **AND** 响应 MUST 为 `503 Service Unavailable` 和 `CodeServiceUnavailable`
- **AND** 响应 message MUST 使用当前退出登录尚未完全生效中文公开文案
- **AND** 该错误 MUST 使用稳定 `Reason` 值 `session_revocation_incomplete`
- **AND** logout metrics MUST 继续能分类撤销不完整失败

#### Scenario: 密码 KDF 繁忙直接渲染为服务不可用响应

- **WHEN** 登录凭据校验返回 `password.ErrPasswordKDFBusy`
- **THEN** 认证 HTTP 边界 MUST 通过 `response.Fail(c, err)` 渲染失败响应
- **AND** 响应 MUST 为 `503 Service Unavailable` 和 `CodeServiceUnavailable`
- **AND** 响应 message MUST 使用当前认证服务繁忙中文公开文案
- **AND** 该错误 MUST 使用稳定 `Reason` 值 `password_kdf_busy`
- **AND** 登录 metrics MUST 继续能通过 `errors.Is(err, password.ErrPasswordKDFBusy)` 或稳定 `Reason` 分类为 password KDF busy

#### Scenario: 认证业务错误保留 errors.Is 语义

- **WHEN** auth feature 或测试需要判断认证、会话、token、撤销或 KDF busy 错误
- **THEN** `errors.Is` 对直接返回的应用错误和被包装后的应用错误 MUST 继续支持正确匹配
- **AND** 该匹配语义 MUST NOT 依赖 HTTP transport 层的错误转换函数

### Requirement: 认证 HTTP transport 统一错误出口

auth HTTP transport MUST 对业务 use case 返回错误使用共享 `response.Fail` 入口，避免在 transport 层重复维护 auth domain、identity 或 password 错误到 HTTP 响应的映射。强制改密登录成功分支 MAY 继续使用现有专用 envelope 映射以携带受限 token data，但该路径 MUST NOT 作为错误 mapper 兼容入口。

#### Scenario: 登录业务错误

- **WHEN** `Login` controller 调用登录 use case 返回错误
- **THEN** controller MUST 直接调用 `response.Fail(c, err)`
- **AND** controller MUST NOT 先调用认证专用错误 mapper
- **AND** controller MUST 保持强制改密成功分支的现有 HTTP `200 OK`、`success=false` 和受限 token data 响应结构

#### Scenario: refresh 业务错误

- **WHEN** `Refresh` controller 调用 refresh use case 返回错误
- **THEN** controller MUST 直接调用 `response.Fail(c, err)`
- **AND** controller MUST NOT 先调用认证专用错误 mapper

#### Scenario: 改密业务错误

- **WHEN** `ChangePassword` controller 调用改密 use case 返回错误
- **THEN** controller MUST 直接调用 `response.Fail(c, err)`
- **AND** controller MUST NOT 先调用认证专用错误 mapper

#### Scenario: 退出当前会话业务错误

- **WHEN** `LogoutCurrentSession` controller 调用退出当前会话 use case 返回错误
- **THEN** controller MUST 直接调用 `response.Fail(c, err)`
- **AND** controller MUST NOT 先调用认证专用错误 mapper

#### Scenario: 退出全部会话业务错误

- **WHEN** `LogoutAllSessions` controller 调用退出全部会话 use case 返回错误
- **THEN** controller MUST 直接调用 `response.Fail(c, err)`
- **AND** controller MUST NOT 先调用认证专用错误 mapper

#### Scenario: 不保留认证错误兼容 mapper

- **WHEN** auth HTTP transport 完成本次迁移
- **THEN** 系统 MUST NOT 保留 `toAuthHTTPError`
- **AND** 系统 MUST NOT 新增等价的 sentinel-to-HTTP 兼容函数、跨模块认证错误映射注册表或仅包装 `contracterrors.FromError` 的认证错误函数

### Requirement: 登录结果分支模型

Auth application MUST 使用登录 use case 专属结果表达普通登录和强制改密登录分支。`TokenResult` MUST 只表达 token 载荷本身；登录业务分支 MUST 位于 `LoginResult` 或等价登录 use case 专属结果类型中，避免 token issuer 或 transport 通过 token 载荷推断业务分支。

#### Scenario: 普通登录结果

- **WHEN** 登录 use case 完成普通登录并创建 refresh session
- **THEN** 返回结果 MUST 包含 `PasswordChangeRequired=false` 和普通 token 载荷
- **AND** token 载荷 MUST 包含 access token、refresh token、token type 和 access token 过期秒数

#### Scenario: 强制改密登录结果

- **WHEN** 登录 use case 完成强制改密登录并创建一次性 password change session
- **THEN** 返回结果 MUST 包含 `PasswordChangeRequired=true` 和受限 password change token 载荷
- **AND** token 载荷 MUST NOT 包含 refresh token
- **AND** token 载荷 MUST NOT 通过 `PasswordChangeRequired` 或等价字段表达业务分支
- **AND** 返回结果 MUST NOT 暴露 `authenticated` 或 `password_change_required` 字符串枚举

#### Scenario: token issuer 保持载荷职责

- **WHEN** token issuer 签发普通 token pair 或 password change token
- **THEN** token issuer MUST 返回 transport-neutral token 载荷
- **AND** token issuer MUST NOT 决定登录 HTTP 响应 envelope、登录业务状态 code 或强制改密响应 shape

### Requirement: 强制改密登录 HTTP 响应

Auth HTTP transport MUST 将强制改密登录表达为业务码 envelope，并使用受限 token 载荷作为 data。controller MUST 使用专用 mapper 生成该 envelope，普通登录 MUST 继续使用普通成功响应。

#### Scenario: controller 映射普通登录

- **WHEN** 登录 use case 返回普通登录结果
- **THEN** controller MUST 返回 HTTP `200 OK`、`CodeOK`、`success=true` 和普通登录响应 DTO
- **AND** 响应 DTO MUST 包含 access token、refresh token、token type 和 expires_in
- **AND** 响应 DTO MUST NOT 包含 `status` 字段

#### Scenario: controller 映射强制改密登录

- **WHEN** 登录 use case 返回强制改密结果
- **THEN** controller MUST 返回 HTTP `200 OK`、`CodePasswordChangeRequired`、`success=false` 和强制改密 envelope
- **AND** envelope data MUST 包含 access token、token type 和 expires_in
- **AND** envelope data MUST NOT 包含 refresh token
- **AND** controller MUST 调用 `toPasswordChangeRequiredEnvelope` 或等价专用 mapper

#### Scenario: 不保留响应枚举

- **WHEN** auth HTTP transport 完成本次重构
- **THEN** 系统 MUST NOT 在登录响应 data 中返回 `status` 字段
- **AND** 系统 MUST NOT 暴露 `authenticated` 或 `password_change_required` 响应枚举
- **AND** 系统 MUST NOT 为登录 token 载荷保留与 `TokenResponse` 字段完全重复的 `LoginResponse` DTO

#### Scenario: 登录失败仍走错误出口

- **WHEN** 登录 use case 返回凭证错误、用户状态拒绝、KDF busy 或系统错误
- **THEN** controller MUST 继续通过 `response.Fail(c, err)` 渲染失败响应
- **AND** 系统 MUST NOT 用登录结果字段表达失败分支
