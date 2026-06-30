## 1. 共享错误契约

- [x] 1.1 在 `common/contract/errors` 增加业务中立的服务不可用错误分类和构造 helper，HTTP status MUST 为 `503 Service Unavailable`。
- [x] 1.2 补充或更新 `common/contract/errors` 单元测试，覆盖服务不可用错误码、message、HTTP status 和 `Cause` 包装语义。

## 2. 认证应用层错误传播

- [x] 2.1 更新 `user-service/internal/features/auth/application/credentials`，使 `password.ErrPasswordKDFBusy` 在登录密码校验中原样向上返回，并记录不泄露内部容量细节的 warn 日志。
- [x] 2.2 保持用户名不存在、密码不匹配、用户状态拒绝和非 busy 密码校验错误的既有认证失败语义，避免泄露凭据细节。
- [x] 2.3 更新 credentials 和 command 层测试，覆盖 KDF busy 不再映射为 `authdomain.ErrInvalidCredentials`，且不会签发任何 token。

## 3. 认证 HTTP 响应与指标

- [x] 3.1 在 auth login failure metrics 中新增稳定英文 reason，例如 `password_kdf_busy`，并更新 `loginFailureReason` 测试。
- [x] 3.2 更新 `user-service/internal/features/auth/transport/http` 错误映射，使 `password.ErrPasswordKDFBusy` 返回服务不可用错误 envelope 和认证服务繁忙消息。
- [x] 3.3 更新 auth controller 测试，断言登录 KDF busy 返回 HTTP 503、非成功 envelope、服务不可用错误码和公开繁忙消息。

## 4. OpenAPI 与文档生成物

- [x] 4.1 更新登录接口 Swagger 注解，声明 `POST /api/v1/auth/login` 可能返回 503。
- [x] 4.2 运行 `make user-service-openapi-generate` 生成 `user-service/docs/openapi.go`、`user-service/docs/openapi.json` 和 `user-service/docs/openapi.yaml`。
- [x] 4.3 检查 OpenAPI 生成物 diff，确认仅包含登录 503 响应等预期变更。

## 5. 验证

- [x] 5.1 运行相关包测试：`go test ./common/contract/errors ./user-service/internal/features/auth/...`。
- [x] 5.2 运行 `make user-service-architecture-lint`，确认架构边界未被破坏。
- [x] 5.3 运行 `make lint`。
- [x] 5.4 运行 `make verify`。
- [x] 5.5 运行 `git diff --exit-code -- user-service/docs/openapi.go user-service/docs/openapi.json user-service/docs/openapi.yaml` 或人工确认生成物 diff 已纳入预期变更。
