## ADDED Requirements

### Requirement: 用户写接口请求体容量边界

系统 MUST 对 `/api/v1/users` 下需要 JSON 请求体的用户写接口执行请求体字节上限检查。超限请求 MUST 在输入 preparer、字段校验、授权后业务 use case 或持久化写入前被拒绝，并 MUST 返回 `413 Payload Too Large` 与统一错误 envelope。

#### Scenario: 创建用户拒绝超限请求体

- **WHEN** 已认证且已授权调用方向 `POST /api/v1/users` 提交超过配置上限的 JSON 请求体
- **THEN** 系统 MUST 返回 `413 Payload Too Large`
- **AND** 系统 MUST NOT 创建用户资料、认证凭证或任何部分持久化数据

#### Scenario: 用户写接口尾随 JSON 超限

- **WHEN** 创建用户请求体首个 JSON 文档合法，但其后追加的尾随 JSON 使总请求体超过配置上限
- **THEN** 系统 MUST 返回 `413 Payload Too Large`
- **AND** 用户 create use case MUST NOT 被调用

#### Scenario: 查询接口不引入请求体限制副作用

- **WHEN** 调用方访问 `GET /api/v1/users/:id` 或 `GET /api/v1/users` 并仅提交合法路径或 query 参数
- **THEN** 请求体容量边界 MUST NOT 改变既有查询、分页、认证、授权和错误渲染语义
