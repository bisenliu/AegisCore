## ADDED Requirements

### Requirement: Initialize user token version on creation
系统 SHALL 在创建用户时初始化认证 token 版本。新用户的 `token_version` MUST 从 `1` 开始；创建用户 API 的成功响应 MUST 保持不返回密码或密码哈希，并不得要求客户端传入 `token_version`。

#### Scenario: Created user has default token version
- **Given** 调用方提交有效创建用户请求
- **When** 系统持久化新用户
- **Then** PostgreSQL 用户记录的 `token_version` MUST 为 `1`
- **Then** 创建用户响应 MUST NOT 包含 `password` 或 `token_version`

#### Scenario: Client cannot set token version during user creation
- **Given** 调用方在创建用户请求中携带 `token_version`
- **When** 系统处理创建用户请求
- **Then** 系统 MUST 忽略客户端提供的 `token_version`
- **Then** 新用户的 `token_version` MUST 使用服务端默认值 `1`
