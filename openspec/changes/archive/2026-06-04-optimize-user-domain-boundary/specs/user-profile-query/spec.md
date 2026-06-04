## ADDED Requirements

### Requirement: Query service uses domain user model
用户资料查询能力 SHALL 通过根 `repository.UserRepository` 获取用户领域实体，并将领域实体映射为查询响应 DTO。Service 层 MUST NOT 直接依赖 Ent 用户模型或 Ent 查询实现类型，查询 API 的认证要求、参数校验、错误映射和响应内容 MUST 保持不变。

#### Scenario: Query maps domain user to response
- **Given** PostgreSQL repository 读取到未软删除用户并返回用户领域实体
- **When** `UserService.GetUserByID` 处理查询结果
- **Then** Service MUST 将用户领域实体映射为 `dto.UserResponse`
- **Then** 响应 MUST 继续包含 `user_id`、`nickname`、`username`、`status`、`created_at` 和 `updated_at`
- **Then** 响应 MUST NOT 包含 `password_hash`、内部 `id` 或 `deleted_at`

#### Scenario: Query service remains independent of Ent
- **Given** 用户资料查询 Service 编译
- **When** 检查 Service 层依赖
- **Then** `user-services/internal/service` MUST NOT 为用户资料查询导入 `github.com/aegiscore/user-services/ent`
- **Then** Ent 查询和 Ent 到 Domain 映射 MUST 位于 `user-services/internal/repository/postgres`

#### Scenario: Query not found mapping remains unchanged
- **Given** PostgreSQL repository 未找到未软删除用户
- **When** Repository 返回 `domain.ErrUserNotFound`
- **Then** Service MUST 继续将该领域错误映射为用户不存在响应
- **Then** HTTP 404 响应信封和公开错误消息 MUST 与现有查询能力保持一致
