## ADDED Requirements

### Requirement: Auth service uses domain user model
用户会话控制能力 SHALL 通过用户领域实体读取认证流程所需的用户状态、密码哈希、外部用户 ID 和 token version。`AuthService` MUST NOT 直接依赖 Ent 用户模型，登录、改密、刷新、退出当前设备和退出全部设备的安全语义 MUST 保持不变。

#### Scenario: Login authenticates domain user
- **Given** 登录流程按用户名读取未软删除用户
- **When** 用户 Repository 返回用户领域实体
- **Then** `AuthService` MUST 使用领域实体中的密码哈希执行共享密码校验
- **Then** `AuthService` MUST 使用领域实体或 `domain.UserStatus` 方法判断普通登录、禁用状态或必须改密状态
- **Then** `AuthService` MUST NOT 为登录流程导入 Ent 用户模型

#### Scenario: Must-change-password issuance remains unchanged
- **Given** 用户领域实体表示用户状态为必须修改密码
- **When** 用户提交的密码校验通过
- **Then** `AuthService` MUST 继续签发受限改密凭据
- **Then** 系统 MUST NOT 创建普通 Redis Refresh Token 会话
- **Then** 响应语义 MUST 与现有会话控制能力保持一致

#### Scenario: Password change validates domain user state
- **Given** 受限改密凭据验证通过并解析出外部用户 ID
- **When** `AuthService` 读取用户领域实体
- **Then** `AuthService` MUST 通过领域实体或 `domain.UserStatus` 方法确认用户仍处于必须改密状态
- **Then** 状态校验通过后系统 MUST 继续通过 Repository 凭证更新契约写入新 `password_hash`、更新状态为正常并递增 token version
- **Then** 状态校验失败时响应语义 MUST 与现有改密流程保持一致

#### Scenario: Token version operations preserve repository contracts
- **Given** 刷新、退出全部设备或认证中间件需要读取或更新 token version
- **When** Service 调用用户 Repository 的 token version 相关方法
- **Then** 这些方法 MAY 继续返回标量 token version 结果
- **Then** Repository MUST 继续以 `domain.ErrUserNotFound` 表达未找到未软删除用户
- **Then** Service MUST 继续负责将领域错误映射为认证失败、token 无效、not found 或内部错误响应
