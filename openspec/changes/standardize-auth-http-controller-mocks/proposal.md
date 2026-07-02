## Why

认证 HTTP controller 测试当前依赖手写 `stubAuthUseCases`，调用契约、参数归一化和错误映射断言分散在自定义字段与闭包中，和 user/permission HTTP controller 已采用的 `gomock` mockgen 风格不一致。统一为生成 mock 可以让认证 HTTP 边界测试显式表达 use case 调用预期，并让生成物 drift 由现有交付流程发现。

## What Changes

- 新增 auth HTTP transport 本地 `go:generate` 入口，为 `LoginUseCase`、`RefreshTokenUseCase`、`ChangePasswordUseCase`、`LogoutCurrentSessionUseCase`、`LogoutAllSessionsUseCase` 生成 `go.uber.org/mock/gomock` mock。
- 改造 `user-service/internal/features/auth/transport/http/controller_test.go`，使用 `gomock` expectation、matcher 或 `DoAndReturn` 验证认证 HTTP controller 与 use case 的调用契约。
- 删除 `stubAuthUseCases` 和只服务于该 stub 的旧辅助字段，迁移后测试只通过生成 mock 表达 use case 调用。
- 保持认证 HTTP 路由、请求/响应 DTO、错误码、OpenAPI 注解和生产行为不变。

## Capabilities

### New Capabilities

- 无。

### Modified Capabilities

- `auth-session-management`: 明确认证 HTTP 边界测试必须通过 feature-local 生成 mock 验证 use case 调用契约，不再保留手写 stub 兼容入口。
- `delivery-operations`: 明确新增的 auth HTTP controller mockgen 入口必须纳入 `make user-service-generate` 和生成物 drift 校验。

## Impact

- 受影响代码：`user-service/internal/features/auth/transport/http/mock_generate.go`、生成的 auth HTTP mock 文件、`user-service/internal/features/auth/transport/http/controller_test.go`。
- 依赖与工具：复用 `go.uber.org/mock/gomock` 与 `mockgen`，不新增全局 mock 包或跨 feature mock 包。
- API 与生产行为：不改变认证 HTTP API、路由、请求/响应 DTO、错误码、OpenAPI 注解或 auth application use case 实现。
- 验证：需要执行 `make user-service-generate`、`cd user-service && go test ./internal/features/auth/transport/http`、`make user-service-architecture-lint`。
