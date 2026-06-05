## MODIFIED Requirements

### Requirement: Verify password-change credentials before loading user state

系统 SHALL 在修改密码流程中先验证受限改密凭据并解析外部用户 UUID，再执行凭证更新和会话吊销编排。受限改密凭据验证 MUST 复用 `common/security/auth.StripBearerPrefix` 支持剥离可选 `Bearer ` 前缀，MUST 解析 password-change token，MUST 校验服务端当前 `token_version` 与 token claims 一致，并 MUST 将 claims 中的 `user_id` 解析为 UUID。用户存在性检查、用户仍处于 `status=300` 的校验、新密码 hash、凭证更新和用户状态恢复 MUST 由认证凭证组件负责。修改密码成功后的用户级 token/session 失效 MUST 由认证会话组件负责。

#### Scenario: Password-change token validation rejects invalid token
- **Given** 调用方提交空白、格式非法、签名无效或 subject 非改密凭据的 token
- **When** 系统验证受限改密凭据
- **Then** 系统 MUST 返回 token 无效响应
- **Then** 系统 MUST NOT 更新用户凭证

#### Scenario: Password-change token validation accepts optional bearer prefix
- **Given** 调用方提交 `Bearer <password-change-token>`
- **When** 系统验证受限改密凭据
- **Then** 系统 MUST 通过 `common/security/auth.StripBearerPrefix` 剥离可选 Bearer 前缀
- **Then** 系统 MUST 按剥离后的 password-change token 执行后续校验

#### Scenario: Password-change token validation rejects changed token version
- **Given** 受限改密凭据签名有效且未过期
- **Given** token claims 中的 `token_version` 与服务端当前版本不一致
- **When** 调用方请求修改密码
- **Then** 系统 MUST 返回 token 无效响应
- **Then** 系统 MUST NOT 查询后续状态或更新用户凭证

#### Scenario: Credential component owns password-change persistence
- **Given** 受限改密凭据通过 token 校验并解析出 UUID `user_id`
- **When** 系统继续处理修改密码请求
- **Then** 修改密码流程 MUST 调用认证凭证组件执行凭证更新
- **Then** 认证凭证组件 MUST 使用该 UUID 查询用户并校验用户仍处于 `status=300`
- **Then** 只有状态校验通过后认证凭证组件 MUST 更新 `password_hash` 并将状态更新为 `100`
- **Then** Auth Service MUST NOT 直接生成密码 hash 或直接调用用户 repository 更新凭证

#### Scenario: Session component owns password-change revocation
- **Given** 受限改密凭据通过 token 校验
- **Given** 认证凭证组件完成新密码持久化
- **When** 系统完成修改密码流程
- **Then** 修改密码流程 MUST 调用认证会话组件执行用户级会话吊销
- **Then** 认证会话组件 MUST 使旧受限改密凭据和既有认证会话失效
- **Then** Auth Service MUST NOT 直接删除 token version 缓存或直接删除 Redis Refresh Token 会话记录

### Requirement: Separate auth orchestration from credential token and session strategies

用户会话控制能力 SHALL 将认证用例编排与凭证校验、凭证更新、token 签发解析和会话管理策略分离。`AuthService` MUST 继续作为登录、修改密码、刷新 token、退出当前设备和退出全部设备的统一入口，并 MUST 保持现有 HTTP 契约、响应信封、错误映射、token claims、Redis 会话行为和 token_version 行为不变。凭证校验和凭证更新 MUST 由认证凭证组件承载，token 签发和解析 MUST 由认证 token 组件承载，Refresh Token 会话生命周期和用户级会话吊销 MUST 由认证会话组件承载，而不是持续堆叠在 `AuthService` 的用例方法中。

#### Scenario: Auth service orchestrates login without owning credential and token strategies
- **Given** 用户提交登录请求
- **When** `AuthService` 处理登录流程
- **Then** 系统 MUST 通过独立凭证组件读取用户认证资料并校验密码
- **Then** `AuthService` MUST 根据用户状态编排普通 token pair 签发或受限改密凭据签发
- **Then** token TTL 兜底、JWT 签发和 Redis Refresh Token 会话创建 MUST 由独立 token 或 session 组件执行
- **Then** 登录成功、无效凭证、禁用用户和必须改密用户的外部行为 MUST 与拆分前保持一致

#### Scenario: Auth service refreshes tokens through token and session components
- **Given** 调用方提交 Refresh Token
- **When** `AuthService` 处理刷新流程
- **Then** 系统 MUST 通过独立 token 组件解析和校验 Refresh Token claims
- **Then** 系统 MUST 通过独立 session 组件校验 Redis 会话存在性、会话 claims 一致性和当前 token_version
- **Then** `AuthService` MUST 继续按配置编排 Refresh Token rotation
- **Then** 新 token 签发、旧会话删除、新会话创建和失败响应语义 MUST 与拆分前保持一致

#### Scenario: Password change delegates credential update and revocation
- **Given** 调用方持有受限改密凭据并提交新密码
- **When** `AuthService` 处理修改密码流程
- **Then** 系统 MUST 通过独立 token 组件解析受限改密凭据
- **Then** 系统 MUST 通过独立 session 组件校验服务端当前 `token_version` 与 token claims 一致
- **Then** `AuthService` MUST 调用独立凭证组件完成用户状态校验、密码 hash、凭证更新和状态恢复
- **Then** `AuthService` MUST 调用独立 session 组件完成用户级 token/session 吊销
- **Then** `AuthService` MUST NOT 直接读取用户状态、hash 新密码、调用用户 repository 更新凭证、删除 token version 缓存或删除 Redis 会话
- **Then** 用户状态校验、凭证更新和受限改密凭据失效语义 MUST 与拆分前保持一致

#### Scenario: Logout flows keep session semantics unchanged
- **Given** 请求已通过 Access Token 认证
- **When** `AuthService` 处理退出当前设备或退出全部设备流程
- **Then** 系统 MUST 继续在 service 边界校验认证上下文中的 `user_id` 和 `session_id`
- **Then** 退出当前设备 MUST 通过独立 session 组件删除当前 Redis Refresh Token 会话
- **Then** 退出全部设备 MUST 通过独立 session 组件先递增 PostgreSQL `token_version`，再删除 token version 缓存和所有 Redis Refresh Token 会话
- **Then** `AuthService` MUST NOT 直接调用用户 repository 递增 `token_version` 或直接删除 Redis token version 缓存和会话记录
- **Then** 当前设备退出和全部设备退出的外部行为 MUST 与拆分前保持一致

#### Scenario: Components remain inside service layer boundaries
- **Given** 认证能力需要拆分凭证、token 和 session 策略
- **When** 实现新增组件或领域服务
- **Then** 组件 MUST 位于 `user-services/internal/service` 或等价 service 层边界内
- **Then** 组件 MUST 依赖 `repository.UserRepository`、`repository.AuthSessionRepository`、`common/security/auth`、`common/security/password` 和配置等现有抽象
- **Then** 组件 MUST NOT 直接依赖 Ent 生成模型、Redis client、controller、router 或 HTTP response writer
- **Then** repository 层 MUST 继续只负责数据访问，controller 层 MUST 继续只负责 HTTP 请求解析和响应输出

#### Scenario: Auth service stores only orchestration dependencies
- **Given** `AuthService` 由 Fx 构造函数创建
- **When** 开发者检查 `authService` 结构体字段
- **Then** `authService` MUST 只保存认证凭证组件、认证 token 组件、认证会话组件和必要的高层编排策略
- **Then** `authService` MUST NOT 保存原始 JWT service 作为字段
- **Then** `authService` MUST NOT 保存用户 repository 作为字段用于凭证更新或 token version 递增

### Requirement: Logout all devices through token version increment

系统 SHALL 支持退出全部设备。退出全部设备时，系统 MUST 通过认证会话组件在 PostgreSQL 中原子递增该用户 `token_version`，更新成功后删除 Redis 中该用户的版本缓存、全部活跃会话记录和会话索引。`AuthService` MUST 仅从认证上下文提取当前用户身份并调用认证会话组件，不得直接执行用户 repository 写入或 Redis 会话清理。

#### Scenario: Logout all sessions invalidates tokens
- **Given** 请求已通过 Access Token 认证
- **When** 调用方请求退出全部设备
- **Then** 系统 MUST 先在 PostgreSQL 中递增该用户 `token_version`
- **Then** 系统 MUST 删除 Redis 中该用户的 token version 缓存
- **Then** 系统 MUST 删除该用户所有 Redis Refresh Token 会话记录
- **Then** 系统 MUST 清空该用户活跃会话索引
- **Then** 旧 Access Token MUST 因版本不一致而失效

#### Scenario: Auth session manager owns logout all writes
- **Given** 请求已通过 Access Token 认证
- **When** `AuthService` 处理退出全部设备流程
- **Then** `AuthService` MUST 从认证上下文提取并校验当前 `user_id`
- **Then** `AuthService` MUST 调用认证会话组件执行全部会话吊销
- **Then** 认证会话组件 MUST 调用用户 repository 原子递增 `token_version`
- **Then** 认证会话组件 MUST 调用 auth session repository 清理 token version 缓存和全部 Refresh Token 会话记录
- **Then** `AuthService` MUST NOT 直接持有用户 repository 来完成退出全部设备写操作
