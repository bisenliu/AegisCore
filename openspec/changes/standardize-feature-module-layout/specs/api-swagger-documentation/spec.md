## MODIFIED Requirements

### Requirement: Swagger annotations reference capability API contracts

Swagger/OpenAPI 文档能力 SHALL 使用按业务能力组织的 API 契约包类型生成用户资料和认证会话接口文档。Swagger 注解 MAY 位于 feature-local `transport/http` controller 包中，但请求体、成功响应或分页文档模型 MUST 继续引用对应 feature 的 `api` 包类型。实现 MUST NOT 在 Swagger 注释中引用全局 DTO 类型，并 MUST 保持生成文档与运行时路由、请求参数、响应结构和统一响应契约一致。

#### Scenario: User Swagger annotations use user API contracts
- **WHEN** Swagger 注释描述创建用户、查询用户或用户列表接口
- **THEN** 注释所在 controller MAY 位于 `user-services/internal/features/user/transport/http`
- **THEN** 请求体、成功响应或分页文档模型 MUST 引用用户 API 契约包中的类型
- **THEN** 注释 MUST NOT 引用全局 DTO 类型

#### Scenario: Auth Swagger annotations use auth API contracts
- **WHEN** Swagger 注释描述登录、刷新、强制改密、退出当前设备或退出全部设备接口
- **THEN** 注释所在 controller MAY 位于 `user-services/internal/features/auth/transport/http`
- **THEN** 请求体和成功响应 MUST 引用认证 API 契约包中的类型
- **THEN** 注释 MUST NOT 引用全局 DTO 类型

#### Scenario: Generated Swagger schema remains compatible
- **WHEN** Swagger 文档从迁移后的源码注解生成
- **THEN** 文档 MUST 继续描述现有 HTTP 方法、路径、认证要求、请求字段、响应信封和失败响应
- **THEN** 文档中的用户资料响应、token 响应、改密响应和登出响应字段 MUST 与迁移前保持兼容
