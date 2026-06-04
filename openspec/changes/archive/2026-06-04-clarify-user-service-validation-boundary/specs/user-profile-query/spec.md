## ADDED Requirements

### Requirement: Validate and normalize query input before service business flow
系统 MUST 在用户查询 Service 执行业务编排前完成查询请求的请求级清洗和基础校验。路径 `user_id` 的 UUID 格式校验、列表分页归一化和过滤字段空白裁剪 MUST 位于 Controller、共享请求校验器或服务内 Validation 层，而不是作为用户查询 Service 的主要职责。

#### Scenario: Validate user ID before service lookup
- **Given** 调用方请求 `GET /api/v1/users/:user_id`
- **When** `user_id` 不是合法 UUID 或缺失
- **Then** 请求 MUST 在调用 Repository 查询前被判定为参数错误
- **Then** controller 或服务内 Validation 层 MUST 负责 UUID 格式校验
- **Then** Service MUST 保留用户不存在和内部查询错误的业务错误映射

#### Scenario: Pass normalized user ID to service
- **Given** 调用方请求 `GET /api/v1/users/018f0000-0000-7000-8000-000000000001`
- **When** controller 调用查询用户 Service
- **Then** Service MUST 接收已通过基础格式校验的用户 ID 输入
- **Then** Service MUST NOT 将路径参数解析错误作为主要业务分支

#### Scenario: Normalize list filters before repository access
- **Given** 调用方请求用户列表并提交 `nickname`、`username`、`page` 或 `page_size` 查询参数
- **When** controller 调用列表查询 Service
- **Then** 过滤字段空白裁剪和分页归一化 MUST 在 Controller 或服务内 Validation 层完成
- **Then** Service MUST 使用规范化后的分页和过滤条件编排 Repository 查询
