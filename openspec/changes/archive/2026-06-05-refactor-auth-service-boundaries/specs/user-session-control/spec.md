## ADDED Requirements

### Requirement: Separate auth orchestration from credential token and session strategies

用户会话控制能力 SHALL 将认证用例编排与凭证校验、token 签发、会话管理策略分离。`AuthService` MUST 继续作为登录、修改密码、刷新 token、退出当前设备和退出全部设备的统一入口，并 MUST 保持现有 HTTP 契约、响应信封、错误映射、token claims、Redis 会话行为和 token_version 行为不变。凭证校验、token 签发或解析、Refresh Token 会话生命周期管理 MUST 由清晰边界的 service 内组件或领域服务承载，而不是持续堆叠在 `AuthService` 的用例方法中。

#### Scenario: Auth service orchestrates login without owning credential and token strategies
- **Given** 用户提交登录请求
- **When** `AuthService` 处理登录流程
- **Then** 系统 MUST 通过独立凭证校验组件读取用户认证资料并校验密码
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

#### Scenario: Password change keeps credential update orchestration compatible
- **Given** 调用方持有受限改密凭据并提交新密码
- **When** `AuthService` 处理修改密码流程
- **Then** 系统 MUST 通过独立 token 组件解析受限改密凭据
- **Then** 系统 MUST 通过独立 session 组件校验服务端当前 token_version 与 token claims 一致
- **Then** `AuthService` MUST 继续读取用户状态、hash 新密码、通过用户 repository 更新凭证并失效 token_version 缓存
- **Then** 用户状态校验、凭证更新和受限改密凭据失效语义 MUST 与拆分前保持一致

#### Scenario: Logout flows keep session semantics unchanged
- **Given** 请求已通过 Access Token 认证
- **When** `AuthService` 处理退出当前设备或退出全部设备流程
- **Then** 系统 MUST 继续在 service 边界校验认证上下文中的 `user_id` 和 `session_id`
- **Then** 退出当前设备 MUST 通过独立 session 组件删除当前 Redis Refresh Token 会话
- **Then** 退出全部设备 MUST 继续先递增 PostgreSQL `token_version`，再通过独立 session 组件删除 token version 缓存和所有 Redis Refresh Token 会话
- **Then** 当前设备退出和全部设备退出的外部行为 MUST 与拆分前保持一致

#### Scenario: Components remain inside service layer boundaries
- **Given** 认证能力需要拆分凭证、token 和 session 策略
- **When** 实现新增组件或领域服务
- **Then** 组件 MUST 位于 `user-services/internal/service` 或等价 service 层边界内
- **Then** 组件 MUST 依赖 `repository.UserRepository`、`repository.AuthSessionRepository`、`common/security/auth`、`common/security/password` 和配置等现有抽象
- **Then** 组件 MUST NOT 直接依赖 Ent 生成模型、Redis client、controller、router 或 HTTP response writer
- **Then** repository 层 MUST 继续只负责数据访问，controller 层 MUST 继续只负责 HTTP 请求解析和响应输出
