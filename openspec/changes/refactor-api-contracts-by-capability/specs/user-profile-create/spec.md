## ADDED Requirements

### Requirement: User creation API contracts are grouped by capability

用户资料创建能力 SHALL 使用按业务能力组织的用户 API 契约包承载创建请求和用户响应模型。实现 MUST NOT 继续依赖全局 `user-services/internal/dto` 包表达用户创建契约，并 MUST 保持 `POST /api/v1/users` 的外部 HTTP 行为不变。

#### Scenario: Create user contract types use user API package
- **WHEN** controller、service、validation 或测试引用创建用户请求或创建成功用户响应
- **THEN** 这些引用 MUST 来自用户 API 契约包
- **THEN** 这些引用 MUST NOT 来自全局 `internal/dto` 包

#### Scenario: Create user request contract remains compatible
- **WHEN** 用户创建请求类型迁移完成
- **THEN** 请求体 MUST 继续使用 `nickname`、`username`、`password` 和可选 `status` 字段
- **THEN** 原有 JSON tag、校验 tag、label 和 example 语义 MUST 保持不变
- **THEN** 缺省用户状态和请求级规范化行为 MUST 保持不变

#### Scenario: Create user response contract remains compatible
- **WHEN** 用户创建响应类型迁移完成
- **THEN** `POST /api/v1/users` MUST 继续返回 HTTP 201 和统一成功响应信封
- **THEN** 创建响应 MUST 继续包含 `user_id`、`nickname`、`username`、`status`、`created_at` 和 `updated_at`
- **THEN** 创建响应 MUST NOT 包含 `password`、`password_hash`、`token_version`、内部 `id` 或 `deleted_at`
