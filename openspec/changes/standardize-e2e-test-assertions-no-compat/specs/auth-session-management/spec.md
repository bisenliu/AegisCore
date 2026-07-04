## ADDED Requirements

### Requirement: 认证会话 E2E flow 断言规范
系统 MUST 使用语义化断言覆盖 user-service E2E HTTP flow 中的认证会话行为，包括普通登录、强制改密登录、修改密码、旧密码登录失败、登出当前会话和 refresh token 失效。断言迁移 MUST 保持当前认证会话、token、错误码和 response envelope 语义不变。

#### Scenario: 普通登录 token 断言
- **WHEN** E2E flow 使用合法用户名和密码完成普通登录
- **THEN** 测试 MUST 使用 `require.NotEmpty`、`require.Equal`、`require.Greater` 或必要 `assert` 验证 access token、refresh token、token type 和 expires_in
- **AND** 测试 MUST NOT 接受缺失 refresh token、旧 token type、旧错误码或旧响应字段兼容分支

#### Scenario: 强制改密登录断言
- **WHEN** E2E flow 使用强制改密用户凭据登录
- **THEN** 测试 MUST 使用语义化断言验证 HTTP `200 OK`、`success=false`、`CodePasswordChangeRequired`、受限 access token metadata 和空 refresh token
- **AND** 测试 MUST NOT 将强制改密分支断言为普通 `CodeOK` 成功登录

#### Scenario: 改密、登出和 refresh 失败断言
- **WHEN** E2E flow 完成改密、使用旧密码重试登录、登出当前会话并使用旧 refresh token 刷新
- **THEN** 测试 MUST 使用语义化断言验证改密成功、旧密码认证失败、登出成功和 refresh token 失效的当前 HTTP status 与应用错误码
- **AND** 迁移 MUST NOT 改变 refresh session、token version、password change token 或 logout 运行时语义
