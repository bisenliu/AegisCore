## ADDED Requirements

### Requirement: Create flow returns domain user model
用户创建能力 SHALL 在持久化创建成功后由 Repository 返回用户领域实体，Service 层 MUST 将该领域实体映射为创建响应 DTO。创建流程中的请求校验、用户名唯一性检查、密码 hash、UUIDv7 生成、冲突错误映射和成功响应内容 MUST 保持不变。

#### Scenario: Create repository returns domain user
- **Given** Service 已完成创建用户业务编排并调用 Repository 写入用户记录
- **When** PostgreSQL repository 使用 Ent 创建用户成功
- **Then** Repository MUST 将创建后的 Ent 用户模型转换为用户领域实体
- **Then** Repository MUST 返回用户领域实体给 Service
- **Then** Service MUST NOT 直接读取 Ent 用户模型构造创建响应

#### Scenario: Create response remains compatible
- **Given** 创建用户成功并获得用户领域实体
- **When** Service 构造创建响应
- **Then** 响应 MUST 继续返回 HTTP 201 和统一成功响应信封
- **Then** 响应 `data` MUST 继续包含 `user_id`、`nickname`、`username`、`status`、`created_at` 和 `updated_at`
- **Then** 响应 `data` MUST NOT 包含 `password`、`password_hash`、`token_version`、内部 `id` 或 `deleted_at`

#### Scenario: Create uniqueness errors remain domain errors
- **Given** Ent 创建用户时发生唯一约束冲突
- **When** PostgreSQL repository 处理该持久化错误
- **Then** Repository MUST 继续返回 `domain.ErrUserAlreadyExists`
- **Then** Service MUST 继续将该领域错误映射为用户已存在冲突响应
