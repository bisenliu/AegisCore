# 认证会话规格

## 需求

### 需求：认证生命周期
认证功能必须支持登录、refresh token 轮换、强制改密、退出当前会话和退出全部会话。

#### 场景：登录签发 token
Given 活跃用户提供有效凭据
When 登录成功
Then 服务必须签发包含必需 `jti` 的 access token、refresh token 和会话元数据。

#### 场景：强制改密用户登录
Given 用户凭据有效但账号要求强制改密
When 登录成功
Then 认证功能必须只签发 subject 为 `password_change` 的受限 token，不得创建普通 refresh session，也不得返回 refresh token。

#### 场景：缺失 JTI 的 token
Given access token 或 refresh token 缺少 `jti`
When token 被解析或校验
Then token 必须被拒绝。

#### 场景：token subject 不匹配
Given token subject 为 `access`、`refresh` 或 `password_change`
When token 被用于不匹配的认证流程
Then token 必须被拒绝，不得在 access token、refresh token 和强制改密 token 之间兼容复用。

#### 场景：refresh 校验会话一致性
Given refresh 请求携带有效 refresh token
When 处理 refresh
Then 认证功能必须在签发替换 token 前校验会话存在性、token version、轮换状态和用户可用性。

#### 场景：refresh session 与 token claims 一致性
Given refresh token 已通过 JWT 解析
When 执行 refresh
Then 认证功能必须校验 Redis refresh session 存在，且 session 中的 `user_id`、`session_id`、`token_version` 必须与 token claims 一致；任一不一致必须拒绝续期。

#### 场景：refresh rotation 原子性
Given refresh token rotation 已启用
When 新 token 已签发但 Redis session 原子替换失败
Then 认证功能不得向客户端返回已签发的新 token，必须按无效 token 或会话错误处理。

### 需求：会话与 token version 策略
认证 application 必须拥有 token version 校验、refresh session 生命周期、每用户活跃 refresh session 上限和会话撤销语义。

#### 场景：活跃 session 上限
Given 用户超过配置的活跃 refresh session 上限
When 创建新的 refresh session
Then Redis 中最旧的活跃会话必须作为安全敏感操作的一部分被同步裁剪。

#### 场景：token version 校验链路
Given access token 已通过 JWT 解析且未过期
When 受保护路由校验 token version
Then 认证功能必须按本地短 TTL 缓存、Redis token version 投影、PostgreSQL 当前值回源的顺序解析当前版本；Redis miss 后允许回源数据库并回填 Redis，但不得缓存错误结果。

#### 场景：token version 投影刷新
Given 用户执行全部设备登出或强制改密
When 用户当前 `token_version` 已变化
Then 认证功能必须使本实例本地 token version 缓存失效，并刷新 Redis token version 投影；旧版本不得覆盖 Redis 中已存在的较新版本。

#### 场景：退出当前会话
Given 已认证用户执行当前设备登出
When 当前 refresh session 被删除
Then 认证功能不得递增用户 `token_version`；既有 access token 仅受自身过期时间和后续 token version 变化约束。

#### 场景：退出全部会话
Given 用户退出全部设备
When 会话被撤销
Then 撤销必须立即具备安全效果，后台物理清理可以提交到专用 workerpool。

#### 场景：全部会话后台清理
Given 用户执行全部设备登出
When token version 已递增并写入当前投影
Then 安全失效不得依赖后台 workerpool；Redis refresh session key 的批量物理删除可以通过 auth 专用 purge workerpool 异步执行。

### 需求：认证包组织
认证 application 必须按 `command`、`authctx`、`credentials`、`tokens`、`sessions`、`validators` 和 `ports.go` 组织职责。

#### 场景：新增凭据行为
Given 行为需要校验或更新密码凭据
When 添加 auth application 代码
Then 代码必须位于 `application/credentials` 或 command 编排中，不得放入 transport 或 Redis 适配器。
