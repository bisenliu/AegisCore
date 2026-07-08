## MODIFIED Requirements

### Requirement: OpenAPI 文档

系统 MUST 暴露和生成 OpenAPI 3 文档，覆盖认证会话、用户资料、角色管理、权限目录、RBAC 授权保护接口和健康检查。OpenAPI 文档 MUST 与 user-service 当前 HTTP API 的响应 shape 保持一致，尤其是登录接口 MUST 同步表达普通登录和强制改密登录两种 envelope 语义。

#### Scenario: 访问 OpenAPI

- **WHEN** 调用方访问 OpenAPI 文档路由
- **THEN** 系统 MUST 返回与当前 user-service HTTP API 匹配的 OpenAPI 内容

#### Scenario: 生成 OpenAPI 文件

- **WHEN** 执行 `make user-service-openapi-generate`
- **THEN** 系统 MUST 更新 `user-service/docs/openapi.json`、`user-service/docs/openapi.yaml` 和相关生成文件

#### Scenario: OpenAPI drift

- **WHEN** API 注解或路由行为变化但 OpenAPI 生成物未同步
- **THEN** `make verify` MUST 能通过生成后 `git diff --exit-code` 暴露 drift

#### Scenario: 运行时文档路由归属

- **WHEN** user-service 暴露 OpenAPI UI、JSON 或 docs redirect
- **THEN** 路由 MUST 由 `user-service/internal/router/openapi.go` 拥有，且健康检查或 metrics endpoint MUST NOT 被当作 `/api/v1` 下的 feature 业务 API

#### Scenario: 登录接口文档表达 envelope 分支

- **WHEN** user-service 生成 OpenAPI 文档
- **THEN** 登录接口 MUST 声明普通登录响应携带 `success=true`、`CodeOK`、access token、refresh token、token type 和 expires_in
- **AND** 登录接口 MUST 描述强制改密登录响应携带 `success=false`、`CodePasswordChangeRequired`、受限 access token、token type 和 expires_in
- **AND** 登录接口 MUST 复用 `TokenResponse` schema，MUST NOT 声明单独的 `LoginResponse` schema
- **AND** 登录接口 MUST NOT 声明 `status`、`authenticated` 或 `password_change_required` 响应枚举
- **AND** 登录接口 MUST 继续声明 KDF busy 可能返回 `503 Service Unavailable`
