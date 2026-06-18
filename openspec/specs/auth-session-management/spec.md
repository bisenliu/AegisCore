## Purpose

定义 user-service 的认证会话能力，覆盖登录、令牌签发、刷新、退出、改密、会话状态和 token version 校验。

## Requirements

### Requirement: 用户登录与令牌签发

系统 MUST 提供用户名密码登录能力，并在凭证、用户状态和会话策略校验通过后签发访问令牌与刷新令牌。

#### Scenario: 登录成功

- **WHEN** 用户提供合法用户名和正确密码，且用户状态允许登录
- **THEN** 系统 MUST 创建会话、签发 access token 与 refresh token，并返回会话相关过期时间

#### Scenario: 凭证错误

- **WHEN** 用户名不存在或密码不匹配
- **THEN** 系统 MUST 拒绝登录并返回一致的认证错误，且 MUST NOT 泄露具体凭证匹配细节

#### Scenario: 用户状态禁止登录

- **WHEN** 用户存在但状态不允许登录
- **THEN** 系统 MUST 拒绝签发令牌并返回明确的状态相关错误

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

### Requirement: 会话退出

系统 MUST 支持退出当前会话和退出全部会话，并保证退出后令牌无法继续访问受保护资源。

#### Scenario: 退出当前会话

- **WHEN** 已认证用户请求退出当前会话
- **THEN** 系统 MUST 撤销当前会话，并使当前 refresh token 失效

#### Scenario: 退出全部会话

- **WHEN** 已认证用户请求退出全部会话
- **THEN** 系统 MUST 撤销该用户的所有活跃会话，并使旧 token 无法继续刷新或访问

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

系统 MUST 将公开认证路由和受保护认证路由分开挂载，并通过共享认证中间件保护需要 bearer token 的接口。

#### Scenario: 公开登录路由

- **WHEN** 调用方访问登录或刷新等公开认证入口
- **THEN** 系统 MUST 允许请求进入认证 controller 并在业务层完成凭证校验

#### Scenario: 受保护认证路由

- **WHEN** 调用方访问退出、修改密码或其他受保护认证入口
- **THEN** 系统 MUST 先通过 JWT、auth config 和 token version validator 校验

#### Scenario: 无效 bearer token

- **WHEN** 受保护认证路由收到缺失、过期、格式错误或签名无效的 bearer token
- **THEN** 系统 MUST 在进入业务处理前拒绝请求
