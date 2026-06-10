## MODIFIED Requirements

### Requirement: User domain model owns user state rules
系统 SHALL 在 `user-services/internal/features/user/domain` 中提供用户领域实体，用于表达用户对外身份、基础资料、认证凭据摘要、状态、token version 和公开时间戳。用户状态相关业务判断、状态合法性校验、允许值列表以及 JSON/query 文本解析 MUST 通过用户领域实体或用户状态枚举方法表达，app service 层 MUST NOT 直接依赖 Ent 用户模型字段实现用户状态规则。用户 HTTP API DTO MAY 直接复用用户领域状态枚举作为请求和响应中的状态类型；实现 MUST NOT 在 `user-services/internal/features/user/api` 中重复定义用户状态类型、状态常量或状态解析/枚举校验方法。

#### Scenario: Domain user represents service user data needs
- **Given** Service 层需要处理用户查询、创建响应、登录、改密或 token version 相关流程
- **When** Service 从用户 Repository 获取用户数据
- **Then** Repository MUST 返回用户领域实体
- **Then** 用户领域实体 MUST 包含 Service 当前业务所需的 `user_id`、`nickname`、`username`、`password_hash`、`status`、`token_version`、`created_at` 和 `updated_at`
- **Then** Service MUST NOT 为读取这些字段导入 Ent 用户模型

#### Scenario: User state rules are centralized
- **Given** 登录或改密流程需要判断用户是否正常、禁用或必须修改密码
- **When** Service 执行用户状态判断
- **Then** Service MUST 使用用户领域实体或 `user.UserStatus` 提供的方法表达状态规则
- **Then** Service MUST NOT 通过散落的 Ent 字段类型转换重复实现相同状态规则

#### Scenario: API DTO reuses domain user status enum
- **Given** 用户创建或用户列表 HTTP request DTO 需要表达可选 `status` 字段
- **When** DTO 定义状态字段类型
- **Then** DTO MUST 直接复用 `user-services/internal/features/user/domain.UserStatus`
- **Then** `user-services/internal/features/user/api` MUST NOT 重复定义 `UserStatus` 类型、状态常量、`IsValid`、`AllowedValues`、`UnmarshalText` 或 `UnmarshalJSON`
- **Then** 共享 enum 校验和 Gin 绑定 MUST 继续使用领域状态类型上的解析和枚举方法
