## ADDED Requirements

### Requirement: 用户资料 E2E 响应断言规范
系统 MUST 使用语义化断言覆盖 user-service E2E HTTP flow 中的用户创建、用户详情查询和用户状态流转响应。断言迁移 MUST 保持用户资料业务语义、测试数据构造和公开响应字段不变。

#### Scenario: 创建用户响应断言
- **WHEN** E2E flow 调用 `POST /api/v1/users` 创建用户
- **THEN** 测试 MUST 使用 `require.NotEmpty`、`require.Equal` 或必要 `assert` 验证 `user_id`、`username` 和当前成功 response envelope
- **AND** 测试 MUST NOT 接受旧用户响应字段、空 `user_id` 或旧创建状态兼容断言

#### Scenario: 查询用户响应断言
- **WHEN** E2E flow 调用 `GET /api/v1/users/:id` 查询目标用户
- **THEN** 测试 MUST 使用语义化断言验证 `user_id`、`username`、`status` 和当前成功 response envelope
- **AND** 测试 MUST NOT 改变公开资料字段、内部 ID 隐藏语义或用户不存在错误语义

#### Scenario: 强制改密后用户状态断言
- **WHEN** E2E flow 创建强制改密用户、完成改密并再次查询该用户
- **THEN** 测试 MUST 使用语义化断言验证用户状态从 `UserStatusMustChangePassword` 流转到 `UserStatusNormal`
- **AND** 迁移 MUST NOT 修改用户状态常量、账号生命周期判断、测试密码或 seed 数据构造
