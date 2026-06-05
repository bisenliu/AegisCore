## Why

当前 `user-services/internal/service/auth_service.go:38-207` 中，`authService` 虽然已经拆出 `CredentialVerifier`、`AuthTokenIssuer` 和 `AuthSessionManager`，但仍直接持有 `repo`、`jwt`、`config` 等底层依赖，并在修改密码与退出全部设备流程中直接执行用户仓储写操作、密码哈希和 token version 失效编排。认证编排、凭证更新、token 签发、session 管理和用户状态变更继续交织，会导致后续 MFA、登录风控、设备管理、审计日志等能力扩展时继续向 `authService` 堆叠底层写逻辑。

本变更通过推动认证子组件自治，使 `authService` 收敛为纯工作流编排器：主服务只表达高层调用顺序，凭证更新和会话整体吊销分别由所属组件封装完整领域管道。

## What Changes

- 扩展 `CredentialVerifier` 职责，使其除登录密码校验外，自治负责修改密码时的用户状态校验、密码哈希生成、凭证更新和用户状态恢复。
- 扩展 `AuthSessionManager` 职责，使其自治负责用户级安全事件下的全部会话吊销，包括 PostgreSQL `token_version` 原子递增、Redis token version 缓存清理和全部 Refresh Token 会话删除。
- 精简 `authService` 字段和构造函数，移除对 `repository.UserRepository`、原始 `*auth.JWTService` 等不再需要直接持有的底层依赖，仅保留子组件和刷新轮转策略所需依赖。
- 重写修改密码和退出全部设备的主流程，使它们只保留 token 解析、凭证更新、会话吊销和响应返回的高层顺序。
- 保持现有 HTTP 路由、DTO、响应信封、错误码、token claims、Redis key 行为、session TTL 和 token version 失效语义兼容。
- 不新增 MFA、设备管理、审计日志或风控功能，仅为这些后续能力预留更清晰的职责边界。

## Capabilities

### New Capabilities

- 无。

### Modified Capabilities

- `user-session-control`: 调整认证会话服务内部职责要求，要求凭证更新和全部会话吊销由对应 service 内子组件自治承载，`AuthService` 作为登录、改密、刷新和退出流程的编排入口，不再直接持有或调用底层凭证写入、JWT 签发实现和用户级 session 吊销写操作。

## Impact

- 主要代码影响：`user-services/internal/service/auth_service.go`、`auth_credentials.go`、`auth_sessions.go`、`auth_tokens.go` 以及对应单元测试 `auth_service_test.go`、`auth_components_test.go`。
- 依赖注入影响：`NewAuthService` 内部组件构造关系需要调整；`repository.UserRepository` 仍可作为子组件依赖注入来源，但不应再保存在 `authService` 字段中。
- 数据和外部契约影响：不修改数据库 schema、Redis key 格式、HTTP API 路径、请求/响应 DTO、公开错误码和认证中间件契约。
- 测试影响：需要补充或调整组件级测试，验证 `CredentialVerifier` 接管改密写操作，`AuthSessionManager` 接管全部会话吊销管道，`authService` 流程测试只断言编排顺序和响应语义。
