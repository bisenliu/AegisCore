## 1. 路由契约实现

- [x] 1.1 将 `user-service/internal/features/auth/transport/http/routes.go` 中强制改密 public route 从 `POST /change-password` 改为 `POST /force-change-password`。
- [x] 1.2 将当前强制改密流程的 handler、use case、command/result、request/response、preparer、mapper、validator、Fx wiring、mock 和测试 helper 同步重命名为 `ForceChangePassword` 语义，并将 OpenAPI `@Router` 改为 `/auth/force-change-password [post]`。
- [x] 1.3 使用文本搜索检查 HTTP path 语义的 `/auth/change-password` 引用，确认旧路径不再注册、不再出现在路由测试期望或 OpenAPI 注解中。

## 2. 测试与文档同步

- [x] 2.1 更新 `user-service/internal/router/router_registration_test.go` 中认证路由列表，期望 `POST /api/v1/auth/force-change-password` 且不包含旧路径。
- [x] 2.2 更新 `user-service/internal/providers/transport/routes_auth_middleware_test.go` 中 public auth 路由用例，确认 `force-change-password` 不需要普通 access token middleware，但仍由业务层校验受限 token。
- [x] 2.3 更新 `docs/ARCHITECTURE.md` 的认证路由说明，将强制改密路径改为 `/api/v1/auth/force-change-password`。
- [x] 2.4 将 `openspec/specs/auth-session-management/spec.md` 的主规格按本 change delta 同步为新路径要求。

## 3. 生成物与验证

- [x] 3.1 运行 `make user-service-openapi-generate`，更新 `user-service/docs/openapi.go`、`user-service/docs/openapi.json` 和 `user-service/docs/openapi.yaml`，并检查生成物 diff 只包含预期路径、描述和 schema 命名变化。
- [x] 3.2 运行相关 Go 测试，至少覆盖 `./user-service/internal/features/auth/transport/http`、`./user-service/internal/router` 和 `./user-service/internal/providers/transport`。
- [x] 3.3 运行 `make user-service-architecture-lint` 验证架构和 OpenSpec 文档规则。
- [x] 3.4 将本次预期代码、文档、规格和 OpenAPI 生成物加入暂存区。
- [x] 3.5 运行 `make lint`，通过后再运行 `make verify`，确认最终验证通过且没有未暂存的预期变更阻塞 git diff 检查。
