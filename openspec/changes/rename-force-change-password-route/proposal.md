## Why

当前 `/api/v1/auth/change-password` 实际承载强制改密/首次登录改密流程，需要使用 `password_change` 受限 token，而不是普通已登录用户改密。继续使用通用 `change-password` 命名会和未来普通改密接口冲突，并误导客户端、OpenAPI 文档和路由保护语义。

## What Changes

- **BREAKING** 将强制改密 HTTP 入口从 `POST /api/v1/auth/change-password` 重命名为 `POST /api/v1/auth/force-change-password`。
- 明确 `force-change-password` 仍属于不经过普通 access token middleware 的 auth public route，但必须在业务层校验 `password_change` 受限 token。
- 将当前强制改密流程的内部命名同步收窄为 `ForceChangePassword`，包括 controller handler、application use case、command/result、HTTP request/response DTO、input preparer、mapper、validator、Fx wiring、mocks 和测试 helper。
- 更新 OpenAPI 注解、路由注册测试、认证 middleware 测试、架构文档和 auth-session-management 规格中的路径描述。
- 不在本 change 中新增普通已登录改密接口；未来普通改密应单独放在 protected auth route，并使用普通 access token 与旧密码校验。
- 不保留旧 `/api/v1/auth/change-password` 别名，避免暴露旧认证路径别名或产生认证绕过歧义。

## Capabilities

### New Capabilities

无。

### Modified Capabilities

- `auth-session-management`: 调整强制改密 HTTP 契约，将受限 token 改密入口命名为 `force-change-password`，并保持普通改密入口的保护语义独立。

## Impact

- API 契约：`POST /api/v1/auth/change-password` 将替换为 `POST /api/v1/auth/force-change-password`。
- OpenAPI：需要重新生成 `user-service/docs/openapi.go`、`user-service/docs/openapi.json` 和 `user-service/docs/openapi.yaml`。
- 代码：影响 `user-service/internal/features/auth/` 的强制改密 symbol 命名、路由注册、controller 注解、Fx wiring、mock 生成和相关测试，以及 `user-service/internal/providers/transport/`、`user-service/internal/router/` 中引用该路径的测试。
- 文档与规格：影响 `docs/ARCHITECTURE.md` 和 `openspec/specs/auth-session-management/spec.md` 的认证路由说明。
- 客户端：调用强制改密流程的客户端需要改用新路径；响应结构、请求体、token subject、会话消费和撤销语义不变。
