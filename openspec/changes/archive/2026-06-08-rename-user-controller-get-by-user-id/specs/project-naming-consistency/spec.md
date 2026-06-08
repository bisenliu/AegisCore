## ADDED Requirements

### Requirement: Name user query controller handler by external user ID

系统 SHALL 要求用户资料查询 controller handler 使用外部用户 ID 语义命名。对于处理 `GET /api/v1/users/:user_id` 的 controller 方法，内部 Go 方法名 MUST 明确包含 `UserID`，避免与内部数据库自增 `id` 查询语义混淆。命名标准化 MUST 同步更新所有 workspace 内 Go 引用，并保持外部可观察契约不变。

#### Scenario: Rename user query controller handler
- **WHEN** `UserController` 处理 `GET /api/v1/users/:user_id` 查询用户资料请求
- **THEN** 对应 handler 方法名 MUST 为 `GetByUserID`
- **THEN** 代码中 MUST NOT 继续使用 `UserController.GetByID` 表达该外部 UUID 查询 handler

#### Scenario: Preserve user query external contract during handler rename
- **WHEN** 用户查询 controller handler 命名标准化完成
- **THEN** `GET /api/v1/users/:user_id` 路径 MUST 保持不变
- **THEN** `user_id` 路径参数名、响应 envelope、公开 JSON 字段、业务错误码和认证要求 MUST 保持不变
- **THEN** 实现 MUST NOT 修改数据库 schema、Atlas migration 或内部自增 `id` 字段语义
