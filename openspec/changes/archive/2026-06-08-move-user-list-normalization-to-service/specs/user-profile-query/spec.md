## MODIFIED Requirements

### Requirement: Validate and normalize query input before service business flow
系统 MUST 在用户查询 Service 调用 Repository 前完成查询请求的请求级清洗和基础校验。路径 `user_id` 的 UUID 格式校验 MUST 位于 Controller、共享请求校验器或服务内 validators 层，并在调用 Service 查询前完成。用户列表的分页归一化、`offset`/`limit` 派生和过滤字段空白裁剪 MUST 由 Service 在 Repository 访问前完成，可复用服务内 validators 层；Controller MUST NOT 调用带副作用的列表归一化函数。

#### Scenario: Validate user ID before service lookup
- **Given** 调用方请求 `GET /api/v1/users/:user_id`
- **When** `user_id` 不是合法 UUID 或缺失
- **Then** 请求 MUST 在调用 Repository 查询前被判定为参数错误
- **Then** controller、共享请求校验器或服务内 validators 层 MUST 负责 UUID 格式校验
- **Then** Service MUST 保留用户不存在和内部查询错误的业务错误映射

#### Scenario: Pass normalized user ID to service
- **Given** 调用方请求 `GET /api/v1/users/018f0000-0000-7000-8000-000000000001`
- **When** controller 调用查询用户 Service
- **Then** Service MUST 接收已通过基础格式校验的用户 ID 输入
- **Then** Service MUST NOT 将路径参数解析错误作为主要业务分支

#### Scenario: Normalize list filters in service before repository access
- **Given** 调用方请求用户列表并提交 `nickname`、`username`、`page` 或 `page_size` 查询参数
- **When** controller 调用列表查询 Service
- **Then** controller MUST 将绑定后的请求传递给 Service，且不得调用带副作用的列表归一化函数
- **Then** Service MUST 在访问 Repository 前应用分页默认值、派生 `offset` 和 `limit`，并裁剪过滤字段空白字符
- **Then** Service MUST 使用规范化后的分页和过滤条件编排 Repository 查询

#### Scenario: Service protects non-HTTP list callers
- **Given** 非 HTTP 调用方使用零值或未归一化分页字段调用 `UserService.ListUsers`
- **When** Service 处理列表查询
- **Then** Service MUST 在调用 Repository 前归一化请求
- **Then** Repository MUST 接收与 HTTP API 默认值等价的有界分页输入
