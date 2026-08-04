## MODIFIED Requirements

### Requirement: Token version 校验与会话撤销

系统 MUST 以 PostgreSQL 当前 `token_version` 为主事实，并通过有界本地 loading cache 和 Redis 投影加速校验。缓存未命中、关闭、过期、驱逐或显式禁用只能影响性能，MUST NOT 改变认证与撤销语义。退出全部会话和密码变更 MUST 递增主事实，使旧 access token 失效并撤销相应 refresh sessions。token-version feature cache MUST 由 user-service 私有配置提供完整默认值、启用时校验和到通用 localcache 配置的集中映射，auth 构造路径 MUST 只消费窄 auth settings。递增 `token_version` 或更新凭证并返回新 `token_version` MUST 是单一确定的 PostgreSQL 结果，成功路径 MUST NOT 在提交后通过第二条 `SELECT` 才获取撤销版本。

#### Scenario: 受保护访问与本地缓存

- **WHEN** access token 已通过签名、subject 和过期校验
- **THEN** 系统 MUST 按有界本地缓存、Redis 投影和 PostgreSQL 当前值的顺序解析 token version，版本不一致时 MUST 拒绝访问，Redis miss 后 MAY 回源并回填
- **AND** 系统 MUST NOT 缓存错误结果；容量驱逐或 TTL 过期后 MUST 可通过同 key 合并回源恢复校验，不得依赖异步写入可见性
- **WHEN** `auth.token_version_cache` 未配置
- **THEN** user-service MUST 使用 `enabled=true`、`size=100000`、`ttl=1s` 和 `load_timeout=300ms` 的完整默认值
- **WHEN** `auth.token_version_cache.enabled` 为 true
- **THEN** 具名 `auth_token_version` cache 的 `size`、`ttl` 和 `load_timeout` MUST 为正值，`size` MUST 表示最大 item 数
- **AND** auth feature MUST 直接提供 string key，MUST NOT 配置 common key encoder；`int64` token version MUST 直接缓存，MUST NOT 配置 clone callback
- **WHEN** cache 被禁用
- **THEN** 系统 MUST 忽略 cache 的 `size`、`ttl` 和 `load_timeout`，不创建通用 loading cache，并保持校验和撤销正确
- **AND** direct stats source MUST 使用 `LoadSuccess` 与 `LoadError` 表达逐次回源结果

#### Scenario: Redis 投影更新

- **WHEN** 系统写入 Redis token version 投影
- **THEN** 正数 `auth.token_version_cache_ttl` MUST 作为显式 TTL，零值或负值 MUST 使用服务默认 TTL，MUST NOT 创建永久投影
- **WHEN** PostgreSQL `token_version` 增加
- **THEN** 系统 MUST 失效本实例本地缓存并刷新 Redis 投影，旧版本 MUST NOT 覆盖较新版本
- **AND** Redis 刷新失败时 MUST 尝试删除投影，使后续校验回源 PostgreSQL；本地失效或投影失败 MUST 被记录并作为可观察错误返回

#### Scenario: 退出当前会话

- **WHEN** 已认证用户退出当前会话，或重复退出已撤销或不存在的会话
- **THEN** 系统 MUST 只撤销目标 refresh session，MUST NOT 递增 `token_version` 或恢复会话，并 MUST 返回稳定结果或明确错误

#### Scenario: 全部退出与已认证改密

- **WHEN** 已认证用户请求退出全部会话
- **THEN** 系统 MUST 递增 PostgreSQL `token_version` 并在同一数据库结果中返回新版本，然后撤销全部活跃 refresh sessions，使旧 access 和 refresh token 无效
- **AND** 安全失效 MUST NOT 依赖后台 workerpool，Redis key 物理清理 MAY 异步执行
- **WHEN** 已认证用户提供正确旧密码和满足策略的新密码
- **THEN** 系统 MUST 原子更新密码哈希并递增 `token_version`，在同一数据库结果中返回新版本，再执行本地缓存失效、Redis 投影刷新和 refresh session 撤销
- **WHEN** 旧密码错误或新密码不满足策略
- **THEN** 系统 MUST 拒绝修改并保持密码、状态和 `token_version` 不变

#### Scenario: 撤销版本原子返回

- **WHEN** 系统执行退出全部会话所需的 `token_version` 递增，或执行强制改密所需的密码哈希、状态和 `token_version` 条件更新
- **THEN** PostgreSQL mutation 与新 `token_version` 返回 MUST 来自单条 `UPDATE ... RETURNING token_version` 或等价事务内更新返回
- **AND** 成功更新路径 MUST NOT 在更新提交后执行第二条 `SELECT` 才获取新版本
- **AND** 故障注入 MUST 只能产生“未更新且返回失败”或“已更新、已拿到新版本并进入撤销编排”两种 application 可观察状态

#### Scenario: 条件凭证更新拒绝

- **WHEN** 强制改密条件更新时用户不存在
- **THEN** 系统 MUST 返回用户不存在错误，并保持密码、状态和 `token_version` 不变
- **WHEN** 用户状态或当前 `token_version` 与强制改密 token/session 绑定条件不匹配
- **THEN** 系统 MUST 返回统一无效凭据，并保持密码、状态和 `token_version` 不变

#### Scenario: 撤销投影不完整

- **WHEN** PostgreSQL 主事实已更新并已返回新 `token_version`，但本地缓存失效、Redis 投影刷新或 refresh session 删除失败
- **THEN** use case MUST 返回 `authdomain.ErrSessionRevocationIncomplete`，MUST NOT 返回普通成功结果
- **AND** 错误链 MUST 保留底层原因，metrics MUST 将结果记录为撤销不完整失败

#### Scenario: Auth settings 依赖边界

- **WHEN** composition 构造 token issuer、session 策略、token-version localcache 或 validator
- **THEN** auth provider MUST 接收只包含 JWT、session 和 token-version cache 所需字段的 auth settings
- **AND** auth feature MUST NOT 依赖完整 user-service 根配置或读取 RBAC、Ent、resources 等无关配置段

### Requirement: 强制改密一次性流程

系统 MUST 为强制改密 token 创建服务端一次性 password-change session，并在更新密码前原子消费。token 与 session MUST 使用独立短 TTL，并绑定 `jti`、`session_id`、`user_id` 和 `token_version`；MUST NOT 复用 refresh session 的 TTL、存储语义或上限裁剪策略。RBAC bootstrap 用户 MUST 通过同一流程完成首次改密，bootstrap CLI MUST NOT 直接实现认证撤销逻辑。

#### Scenario: 创建会话与 bootstrap 首次登录

- **WHEN** 强制改密登录签发 password-change token
- **THEN** 系统 MUST 创建 claims 完全一致的 Redis session，token 与 session MUST 使用 `auth.jwt.password_change_token_ttl`；配置未设置或非正数时 MUST 使用 5 分钟默认 TTL，MUST NOT 永不过期
- **AND** session 创建失败时登录 MUST 失败，已签发 token MUST NOT 返回客户端
- **WHEN** RBAC bootstrap 创建的固定超级管理员以临时密码首次登录
- **THEN** 用户状态 MUST 为 `identity.UserStatusMustChangePassword`，只能获得受限 token；改密完成后 MUST 转为 normal，随后才能普通登录并使用超级管理员权限
- **AND** bootstrap CLI MUST NOT 直接执行条件凭据更新、token version 更新、投影刷新、缓存失效或 session 撤销

#### Scenario: 原子消费、并发与无效凭据

- **WHEN** token 有效且 Redis session 的绑定 claims 全部匹配
- **THEN** 系统 MUST 原子删除一次性 session，并 MAY 继续执行条件凭据更新；并发请求最多一个能消费 session、更新密码并递增一次 `token_version`
- **WHEN** token 或 session 过期、不存在、已撤销、已消费、绑定不一致，或并发请求未成功消费
- **THEN** 系统 MUST 返回统一无效凭据，MUST NOT 泄露原因或更新密码、状态与 `token_version`

#### Scenario: 条件更新与撤销

- **WHEN** session 已消费，用户仍为 `UserStatusMustChangePassword` 且 PostgreSQL 当前 `token_version` 等于 token 中旧版本
- **THEN** 系统 MUST 更新密码哈希、将状态改为 `UserStatusNormal` 并递增 `token_version`，且 MUST 在同一数据库结果中返回新 `token_version`
- **AND** 状态或版本不匹配时 MUST 返回统一无效凭据且不得更新任何字段
- **AND** 更新成功并拿到新 `token_version` 后 MUST 失效本地缓存、刷新 Redis 投影并删除该用户 refresh sessions；任一步失败 MUST 返回可观察的安全撤销未完成错误，MUST NOT 返回普通成功结果
- **AND** 成功更新路径 MUST NOT 在更新提交后执行第二条 `SELECT` 才获取新版本
