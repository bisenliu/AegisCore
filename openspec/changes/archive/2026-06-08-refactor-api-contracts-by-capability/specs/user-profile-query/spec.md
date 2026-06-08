## ADDED Requirements

### Requirement: User query API contracts are grouped by capability

用户资料查询能力 SHALL 使用按业务能力组织的用户 API 契约包承载查询请求、列表请求、用户响应和用户列表文档模型。实现 MUST NOT 继续依赖全局 `user-services/internal/dto` 包表达用户资料查询契约，并 MUST 保持 `GET /api/v1/users/:user_id` 与 `GET /api/v1/users` 的外部 HTTP 行为不变。

#### Scenario: Query user contract types use user API package
- **WHEN** controller、service、validation 或测试引用用户查询请求、列表请求、用户响应或用户列表文档模型
- **THEN** 这些引用 MUST 来自用户 API 契约包
- **THEN** 这些引用 MUST NOT 来自全局 `internal/dto` 包

#### Scenario: Query user behavior remains compatible after package migration
- **WHEN** 用户查询 API 契约类型迁移完成
- **THEN** `GET /api/v1/users/:user_id` 的路径、认证要求、UUID 校验、响应信封、错误语义和公开 JSON 字段 MUST 保持不变
- **THEN** `GET /api/v1/users` 的分页、过滤参数、响应信封、分页结构和公开 JSON 字段 MUST 保持不变

#### Scenario: Query response still hides internal fields
- **WHEN** Service 将用户领域实体映射为用户查询或列表响应
- **THEN** 响应 MUST 继续包含 `user_id`、`nickname`、`username`、`status`、`created_at` 和 `updated_at`
- **THEN** 响应 MUST NOT 包含 `password`、`password_hash`、内部 `id`、`token_version` 或 `deleted_at`
